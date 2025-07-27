package balance

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/testutil"
	"github.com/nkiryanov/gophermart/tests/e2e"
)

const (
	ListWithdrawalURL = "/api/user/balance/withdrawals"
)

func Test_BalanceListWithdraw(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	e2e.ServeInTx(pg.Pool, t, func(tx pgx.Tx, srvURL string, s e2e.Services) {
		username := "test-user"
		pwd := "pwd"
		user, err := s.UserService.CreateUser(t.Context(), username, pwd)
		require.NoError(t, err)

		listWithdrawals := func(t *testing.T) *http.Response {
			req, err := http.NewRequest(http.MethodGet, srvURL+ListWithdrawalURL, nil)
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

		t.Run("list withdrawals ok", func(t *testing.T) {
			testutil.InTx(tx, t, func(_ pgx.Tx) {
				// Crete withdrawal transaction
				_, err = s.Storage.Balance().CreateTransaction(t.Context(), models.Transaction{
					ID:          uuid.New(),
					ProcessedAt: testutil.MustParseTime(t, "2024-11-01 15:04:05Z"),
					UserID:      user.ID,
					OrderNumber: "1234",
					Amount:      decimal.RequireFromString("123.34"),
					Type:        models.TransactionTypeWithdrawal,
				})
				require.NoError(t, err, "failed to create withdrawal transaction")

				resp := listWithdrawals(t)
				defer resp.Body.Close() // nolint:errcheck
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err, "failed to read response body")

				require.Equalf(t, http.StatusOK, resp.StatusCode, "withdrawals list request should return 200. Body: %s", string(body))

				type response struct {
					Order       string    `json:"order"`
					Sum         float64   `json:"sum"`
					ProcessedAt time.Time `json:"processed_at"`
				}

				got := make([]response, 0)
				err = json.Unmarshal(body, &got)
				require.NoError(t, err, "failed to unmarshal response body")

				require.Equal(t, 1, len(got))
				require.Equal(t, "1234", got[0].Order, "order number should match")
				require.Equal(t, 123.34, got[0].Sum)
			})
		})

		t.Run("exclude accruals", func(t *testing.T) {
			testutil.InTx(tx, t, func(_ pgx.Tx) {
				// Crete accrual transaction
				_, err = s.Storage.Balance().CreateTransaction(t.Context(), models.Transaction{
					ID:          uuid.New(),
					ProcessedAt: testutil.MustParseTime(t, "2024-11-01 15:04:05Z"),
					UserID:      user.ID,
					OrderNumber: "1234",
					Amount:      decimal.RequireFromString("123.34"),
					Type:        models.TransactionTypeAccrual,
				})
				require.NoError(t, err, "failed to create transaction")

				resp := listWithdrawals(t)
				defer resp.Body.Close() // nolint:errcheck
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err, "failed to read response body")

				require.Equalf(t, http.StatusOK, resp.StatusCode, "withdrawals list request should return 200. Body: %s", string(body))
				require.JSONEq(t, `[]`, string(body), "accruals should not be included in withdrawals list")
			})
		})

		t.Run("unauthorized request", func(t *testing.T) {
			testutil.InTx(tx, t, func(_ pgx.Tx) {
				req, err := http.NewRequest(http.MethodGet, srvURL+ListWithdrawalURL, nil)
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
