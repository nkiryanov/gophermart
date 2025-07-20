package auth

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/testutil"
	"github.com/nkiryanov/gophermart/tests/integration"
)

const (
	RefreshURL = "/auth/refresh"
)

func Test_AuthRefresh(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	t.Run("refresh token ok", func(t *testing.T) {
		integration.RunTx(pg.Pool, t, func(srvURL string, s integration.Services) {
			pair, err := s.AuthService.Register(t.Context(), "nk", "StrongEnoughPassword")
			require.NoError(t, err)

			// Create request and set auth cookies. Save them to verify they are rolled later
			req, err := http.NewRequest(http.MethodPost, srvURL+RefreshURL, nil)
			require.NoError(t, err)
			s.AuthService.SetTokenPairToRequest(req, pair)
			firstRefreshCookie := req.Cookies()[0]
			firstAccessHeader := req.Header.Get("Authorization")
			assert.NotEmpty(t, firstRefreshCookie.Value, "refresh cookie should not be empty")
			assert.NotEmpty(t, firstAccessHeader, "access token should not be empty")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			defer resp.Body.Close() // nolint:errcheck

			require.Equalf(t, http.StatusOK, resp.StatusCode, "not expected code. Body: %s", string(body))
			require.JSONEq(t, `
				{
					"message": "Tokens refreshed successfully"
				}`, string(body))

			require.Equal(t, 1, len(resp.Cookies()))

			secondRefreshCookie := resp.Cookies()[0]
			require.NotEmpty(t, secondRefreshCookie.Value, "refresh cookie should not be empty")
			secondAccessHeader := resp.Header.Get("Authorization")
			require.NotEmpty(t, secondAccessHeader, "access token should not be empty")
			require.NotEqual(t, firstRefreshCookie.Value, secondRefreshCookie.Value, "refresh token should be changed after refresh")
			require.NotEqual(t, firstAccessHeader, secondAccessHeader, "access token should be changed after refresh")
		})
	})

	t.Run("refresh refresh twice fail", func(t *testing.T) {
		integration.RunTx(pg.Pool, t, func(srvURL string, s integration.Services) {
			pair, err := s.AuthService.Register(t.Context(), "nk", "StrongEnoughPassword")
			require.NoError(t, err)

			// Create request and set auth cookies. Save them to verify they are rolled later
			createRequest := func(pair models.TokenPair) *http.Request {
				req, err := http.NewRequest(http.MethodPost, srvURL+RefreshURL, nil)
				require.NoError(t, err)
				s.AuthService.SetTokenPairToRequest(req, pair)
				return req
			}

			req1 := createRequest(pair)
			resp1, err := http.DefaultClient.Do(req1)
			require.NoError(t, err, "refresh request should always complete")
			body1, err := io.ReadAll(resp1.Body)
			require.NoError(t, err)
			defer resp1.Body.Close() // nolint:errcheck

			require.Equalf(t, http.StatusOK, resp1.StatusCode, "not expected code. Body: %s", string(body1))

			req2 := createRequest(pair)
			resp2, err := http.DefaultClient.Do(req2)
			require.NoError(t, err, "refresh request should always complete")
			body2, err := io.ReadAll(resp2.Body)
			require.NoError(t, err)
			defer resp2.Body.Close() // nolint:errcheck

			require.Equalf(t, http.StatusUnauthorized, resp2.StatusCode, "not expected code. Body: %s", string(body2))
			require.JSONEq(t, `
				{
					"error": "service_error",
					"message": "Refresh token not found"
				}`, string(body2))
		})
	})

}
