package balance

import (
	"io"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/testutil"
	"github.com/nkiryanov/gophermart/tests/e2e"
)

const (
	BalanceURL = "/api/user/balance"
)

func Test_Balance(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	e2e.ServeInTx(pg.Pool, t, func(tx pgx.Tx, srvURL string, s e2e.Services) {
		_, err := s.UserService.CreateUser(t.Context(), "test-user", "pwd")
		require.NoError(t, err)

		authReq := func(username string, pwd string, t *testing.T) *http.Request {
			req, err := http.NewRequest(http.MethodGet, srvURL+BalanceURL, nil)
			require.NoError(t, err, "failed to create request")

			pair, err := s.AuthService.Login(t.Context(), username, pwd)
			require.NoError(t, err, "failed to login user")

			s.AuthService.SetTokenPairToRequest(req, pair)
			return req
		}

		t.Run("get balance ok", func(t *testing.T) {
			testutil.InTx(tx, t, func(_ pgx.Tx) {
				req := authReq("test-user", "pwd", t)
				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err, "failed to send request")
				defer resp.Body.Close() // nolint:errcheck

				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err, "failed to read response body")

				require.Equalf(t, http.StatusOK, resp.StatusCode, "balance request should return 200. Body: %s", string(body))

				require.JSONEq(t, `{
					"current": 0,
					"withdrawn": 0
				}`, string(body))
			})
		})

		t.Run("unauthorized request", func(t *testing.T) {
			testutil.InTx(tx, t, func(_ pgx.Tx) {
				req, err := http.NewRequest(http.MethodGet, srvURL+BalanceURL, nil)
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
