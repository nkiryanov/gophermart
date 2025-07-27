package balance

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/testutil"
	"github.com/nkiryanov/gophermart/tests/e2e"
)

const (
	WithdrawURL = "/api/user/balance/withdraw"
)

func Test_BalanceWithdraw(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	type request struct {
		Order string  `json:"order"`
		Sum   float64 `json:"sum"`
	}

	e2e.ServeInTx(pg.Pool, t, func(tx pgx.Tx, srvURL string, s e2e.Services) {
		username := "test-user"
		pwd := "pwd"
		user, err := s.UserService.CreateUser(t.Context(), username, pwd)
		require.NoError(t, err)

		doWithdraw := func(t *testing.T, data request) *http.Response {
			// Create request
			d, err := json.Marshal(data)
			require.NoError(t, err, "failed to marshal withdraw request")
			req, err := http.NewRequest(http.MethodPost, srvURL+WithdrawURL, bytes.NewReader(d))
			require.NoError(t, err, "failed to create request")

			// Set authentication data
			pair, err := s.AuthService.Login(t.Context(), username, pwd)
			require.NoError(t, err, "failed to login user")
			s.AuthService.SetTokenPairToRequest(req, pair)

			// Send request
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err, "failed to send request")

			return resp
		}

		t.Run("withdraw insufficient fail", func(t *testing.T) {
			testutil.InTx(tx, t, func(_ pgx.Tx) {
				resp := doWithdraw(t, request{Order: "1234", Sum: 1000})
				defer resp.Body.Close() // nolint:errcheck
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err, "failed to read response body")

				require.Equalf(t, http.StatusPaymentRequired, resp.StatusCode, "not expected code, body: %s", string(body))

				require.JSONEq(t, `{
					"error": "service_error",
					"message": "Insufficient balance"
				}`, string(body), "not expected response body")
			})
		})

		t.Run("withdraw ok", func(t *testing.T) {
			testutil.InTx(tx, t, func(_ pgx.Tx) {
				_, err := s.Storage.Balance().UpdateBalance(t.Context(), models.Transaction{
					ID:          uuid.New(),
					UserID:      user.ID,
					ProcessedAt: testutil.MustParseTime(t, "2024-11-01 15:04:05Z"),
					Amount:      decimal.RequireFromString("1000.01"),
					Type:        models.TransactionTypeAccrual,
				})
				require.NoError(t, err, "failed to update balance")

				resp := doWithdraw(t, request{Order: "1234", Sum: 1000})
				defer resp.Body.Close() // nolint:errcheck
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err, "failed to read response body")

				require.Equalf(t, http.StatusOK, resp.StatusCode, "withdraw request should return 200. Body: %s", string(body))
				require.JSONEq(t, `{
					"current": 0.01,
					"withdrawn": 1000
				}`, string(body), "not expected response body")
			})
		})

		t.Run("unauthorized request", func(t *testing.T) {
			testutil.InTx(tx, t, func(_ pgx.Tx) {
				req, err := http.NewRequest(http.MethodPost, srvURL+WithdrawURL, nil)
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
