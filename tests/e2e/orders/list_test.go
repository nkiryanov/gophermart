package orders

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/repository"
	"github.com/nkiryanov/gophermart/internal/testutil"
	"github.com/nkiryanov/gophermart/tests/e2e"
)

const (
	OrderListURL = "/api/user/orders"
)

func Test_OrdersList(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	type orderResponse struct {
		Number     string           `json:"number"`
		Status     string           `json:"status"`
		Accrual    *decimal.Decimal `json:"accrual,omitempty"`
		UploadedAt time.Time        `json:"uploaded_at"`
	}

	e2e.ServeInTx(pg.Pool, t, func(tx pgx.Tx, srvURL string, s e2e.Services) {
		user, err := s.UserService.CreateUser(t.Context(), "test-user", "pwd")
		require.NoError(t, err)

		listOrdersReq := func(username string, pwd string, t *testing.T) *http.Request {
			req, err := http.NewRequest(http.MethodGet, srvURL+OrderListURL, nil)
			require.NoError(t, err, "failed to create request")
			pair, err := s.AuthService.Login(t.Context(), username, pwd)
			require.NoError(t, err, "failed to login user")
			s.AuthService.SetTokenPairToRequest(req, pair)
			return req
		}

		t.Run("empty list", func(t *testing.T) {
			testutil.InTx(tx, t, func(_ pgx.Tx) {
				req := listOrdersReq("test-user", "pwd", t)
				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err, "failed to send request")
				defer resp.Body.Close() // nolint:errcheck

				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err, "failed to read response body")
				require.Equalf(t, http.StatusNoContent, resp.StatusCode, "empty list should return 204. Body: %s", string(body))
				require.Empty(t, string(body), "body should be empty for 204 status")
			})
		})

		t.Run("list all orders", func(t *testing.T) {
			testutil.InTx(tx, t, func(_ pgx.Tx) {
				_, err := s.OrderService.CreateOrder(t.Context(), "4111111111111111", &user,
					repository.WithOrderStatus(models.OrderStatusNew),
					repository.WithUploadedAt(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)),
				)
				require.NoError(t, err, "first order has to be created ok")

				_, err = s.OrderService.CreateOrder(t.Context(), "4242424242424242", &user,
					repository.WithOrderStatus(models.OrderStatusProcessed),
					repository.WithOrderAccrual(decimal.RequireFromString("100.50")),
					repository.WithUploadedAt(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)),
				)
				require.NoError(t, err, "second order has to be created ok")

				req := listOrdersReq("test-user", "pwd", t)
				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err, "failed to send request")
				defer resp.Body.Close() // nolint:errcheck

				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err, "failed to read response body")
				require.Equalf(t, http.StatusOK, resp.StatusCode, "list with orders should return 200. Body: %s", string(body))

				var response []orderResponse
				err = json.Unmarshal(body, &response)
				require.NoError(t, err, "failed to unmarshal response body")

				require.Equal(t, 2, len(response), "response should contain 2 orders")
				require.Equal(t, "4242424242424242", response[0].Number, "orders must be ordered uploaded_at DESC")
				require.Equal(t, "4111111111111111", response[1].Number, "second order number should match")
			})
		})

		t.Run("unauthorized request", func(t *testing.T) {
			testutil.InTx(tx, t, func(_ pgx.Tx) {
				req, err := http.NewRequest(http.MethodGet, srvURL+OrderListURL, nil)
				require.NoError(t, err, "failed to create request")

				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err, "failed to send request")
				defer resp.Body.Close() // nolint:errcheck

				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err, "failed to read response body")
				require.Equalf(t, http.StatusUnauthorized, resp.StatusCode, "unauthorized request should return 401. Body: %s", string(body))
			})
		})
	})
}
