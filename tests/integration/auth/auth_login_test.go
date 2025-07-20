package auth

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/testutil"
	"github.com/nkiryanov/gophermart/tests/integration"
)

const (
	LoginURL = "/auth/login"
)

func Test_Login(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	t.Run("login ok", func(t *testing.T) {
		integration.RunTx(pg.Pool, t, func(srvURL string, s integration.Services) {
			_, err := s.AuthService.Register(t.Context(), "nk", "StrongEnoughPassword")
			require.NoError(t, err)

			data := `{"login": "nk", "password": "StrongEnoughPassword"}`
			resp, err := http.Post(srvURL+LoginURL, "application/json", strings.NewReader(data))
			require.NoError(t, err)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			require.Equalf(t, http.StatusOK, resp.StatusCode, "not expected code. Body: %s", string(body))
			require.JSONEq(t, `
				{
					"message": "User logged in successfully"
				}`, string(body))

			require.Equal(t, 1, len(resp.Cookies()))
			cookie := resp.Cookies()[0]
			require.Equal(t, "refreshtoken", cookie.Name)
			require.Equal(t, cookie.HttpOnly, true, "refresh cookie should be HttpOnly")
			require.Equal(t, "/", cookie.Path, "refresh cookie should be available on / path")
			require.Equal(t, http.SameSiteStrictMode, cookie.SameSite, "refresh cookie should be SameSite Strict")
			require.InDelta(t, (24 * time.Hour).Seconds(), cookie.MaxAge, 1, "max age should be refresh TTL with 1 second delta")
			require.NotEmpty(t, cookie.Value, "refresh cookie should not be empty")

			require.Contains(t, resp.Header, "Authorization")
			header := resp.Header.Get("Authorization")
			require.Contains(t, header, "Bearer")
		})
	})

	t.Run("login failed", func(t *testing.T) {
		integration.RunTx(pg.Pool, t, func(srvURL string, _ integration.Services) {
			data := `{"login": "nk", "password": "WrongPassword"}`

			resp, err := http.Post(srvURL+LoginURL, "application/json", strings.NewReader(data))
			require.NoError(t, err)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			require.Equalf(t, http.StatusUnauthorized, resp.StatusCode, "not expected code. Body: %s", string(body))
			require.JSONEq(t, `
				{
					"error": "service_error",
					"message": "User not found"
				}`, string(body))

			require.Equal(t, 0, len(resp.Cookies()), "no cookies should be set on login error")
			require.NotContains(t, resp.Header, "Authorization", "Authorization header should not be set")
		})
	})
}
