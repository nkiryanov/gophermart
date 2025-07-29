package auth

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/testutil"
	"github.com/nkiryanov/gophermart/tests/e2e"
)

const (
	RegisterURL = "/api/user/register"
)

func Test_AuthRegister(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	e2e.ServeInTx(pg.Pool, t, func(tx pgx.Tx, srvURL string, s e2e.Services) {
		t.Run("register ok", func(t *testing.T) {
			testutil.InTx(tx, t, func(_ pgx.Tx) {
				data := `{"login": "nk", "password": "StrongEnoughPassword"}`

				resp, err := http.Post(srvURL+RegisterURL, "application/json", strings.NewReader(data))
				require.NoError(t, err)
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				defer func() { _ = resp.Body.Close() }()

				require.Equalf(t, http.StatusOK, resp.StatusCode, "not expected code. Body: %s", string(body))
				require.JSONEq(t, `
					{
						"message": "User registered successfully"
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

		t.Run("register existed user fails", func(t *testing.T) {
			testutil.InTx(tx, t, func(_ pgx.Tx) {
				_, err := s.AuthService.Register(t.Context(), "nk", "StrongEnoughPassword")
				require.NoError(t, err)

				data := `{"login": "nk", "password": "StrongEnoughPassword"}`
				resp, err := http.Post(srvURL+RegisterURL, "application/json", strings.NewReader(data))
				require.NoError(t, err)
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				defer func() { _ = resp.Body.Close() }()

				require.Equalf(t, http.StatusConflict, resp.StatusCode, "not expected code. Body: %s", string(body))
				require.JSONEq(t, `
					{
						"error": "service_error",
						"message": "User already exists"
					}`, string(body))

				require.Equal(t, 0, len(resp.Cookies()))
				require.NotContains(t, resp.Header, "Authorization", "Authorization header should not be set for register request")
			})
		})

	})
}
