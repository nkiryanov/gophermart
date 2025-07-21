package orders

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/testutil"
	"github.com/nkiryanov/gophermart/tests/e2e"
)

const (
	OrderCreateURL = "/api/user/orders"
)

func Test_OrdersCreate(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	e2e.ServeWithTx(pg.Pool, t, func(tx pgx.Tx, srvURL string, s e2e.Services) {
		user, err := s.UserService.CreateUser(t.Context(), "test-user", "pwd")
		require.NoError(t, err)

		type Response struct {
			Number     string    `json:"number"`
			Status     string    `json:"status"`
			UploadedAt time.Time `json:"uploaded_at"`
		}

		authReq := func(user string, pwd string, orderNum string, t *testing.T) *http.Request {
			req, err := http.NewRequest(http.MethodPost, srvURL+OrderCreateURL, strings.NewReader(orderNum))
			require.NoError(t, err, "failed to create request")

			pair, err := s.AuthService.Login(t.Context(), user, pwd)
			require.NoError(t, err, "failed to login user")

			s.AuthService.SetTokenPairToRequest(req, pair)
			return req
		}

		t.Run("create order ok", func(t *testing.T) {
			testutil.WithTx(tx, t, func(_ pgx.Tx) {
				req := authReq("test-user", "pwd", "123", t)
				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err, "failed to send request")
				defer resp.Body.Close() // nolint:errcheck
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err, "failed to read response body")

				require.Equalf(t, http.StatusAccepted, resp.StatusCode, "not expected status code. Body: %s", string(body))

				var response Response
				err = json.Unmarshal(body, &response)
				require.NoError(t, err, "failed to unmarshal response body")

				assert.Equal(t, "123", response.Number, "order number should match")
				assert.Equal(t, models.OrderNew, response.Status, "order status should be 'new'")
				assert.WithinDuration(t, time.Now(), response.UploadedAt, time.Second, "uploaded_at should be close to now")

			})
		})

		t.Run("create twice ok", func(t *testing.T) {
			testutil.WithTx(tx, t, func(_ pgx.Tx) {
				order, err := s.OrderService.CreateOrder(t.Context(), "123", &user)
				require.NoError(t, err, "order has to be created ok")

				req := authReq("test-user", "pwd", "123", t)
				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err, "failed to send request")
				defer resp.Body.Close() // nolint:errcheck
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err, "failed to read response body")

				var response Response
				require.Equalf(t, http.StatusOK, resp.StatusCode, "if order already exists 200 must be returned. Body: %s", string(body))
				err = json.Unmarshal(body, &response)
				require.NoError(t, err, "failed to unmarshal response body")

				assert.Equal(t, order.Number, response.Number, "order number should match")
				assert.Equal(t, order.Status, response.Status, "order status should match")
				assert.Equal(t, order.UploadedAt, response.UploadedAt, "uploaded_at should be the same for the same order")
			})
		})

		t.Run("create on conflict", func(t *testing.T) {
			testutil.WithTx(tx, t, func(_ pgx.Tx) {
				// User order
				_, err := s.OrderService.CreateOrder(t.Context(), "123", &user)
				require.NoError(t, err, "order has to be created ok")

				// ya user
				_, err = s.UserService.CreateUser(t.Context(), "yet-another-user", "pwd")
				require.NoError(t, err)

				req := authReq("yet-another-user", "pwd", "123", t)
				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err, "failed to send request")
				defer resp.Body.Close() // nolint:errcheck
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err, "failed to read response body")

				require.Equalf(t, http.StatusConflict, resp.StatusCode, "if the number is taken by order by other user then 409 expected", string(body))
				require.JSONEq(t, `{
					"error": "service_error",
					"message": "Order number already taken"
				}`, string(body))
			})
		})

	})
}
