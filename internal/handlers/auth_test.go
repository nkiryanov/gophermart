package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/repository/postgres"
	"github.com/nkiryanov/gophermart/internal/service/auth"
	"github.com/nkiryanov/gophermart/internal/service/auth/tokenmanager"
	"github.com/nkiryanov/gophermart/internal/testutil"
)

func Test_AuthHandler(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	// Run http server and attach auth handlers
	// Production AuthService will be used
	withTx := func(dbpool *pgxpool.Pool, t *testing.T, fn func(url string, auth *auth.AuthService)) {
		testutil.WithTx(dbpool, t, func(tx pgx.Tx) {
			userRepo := &postgres.UserRepo{DB: tx}
			refreshRepo := &postgres.RefreshTokenRepo{DB: tx}

			tokenManager, err := tokenmanager.New(tokenmanager.Config{SecretKey: "test-secret"}, refreshRepo)
			require.NoError(t, err, "token manager should be created without errors")

			// Initialize production auth service
			s, err := auth.NewService(auth.Config{}, tokenManager, userRepo)
			require.NoError(t, err, "auth service starting error", err)

			h := NewAuth(s)
			srv := httptest.NewServer(h.Handler())
			defer srv.Close()

			fn(srv.URL, s)
		})
	}

	t.Run("login ok", func(t *testing.T) {
		withTx(pg.Pool, t, func(url string, auth *auth.AuthService) {
			_, err := auth.Register(t.Context(), "nk", "StrongEnoughPassword")
			require.NoError(t, err)

			data := `{"login": "nk", "password": "StrongEnoughPassword"}`
			resp, err := http.Post(url+"/login", "application/json", strings.NewReader(data))
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
		withTx(pg.Pool, t, func(url string, auth *auth.AuthService) {
			data := `{"login": "nk", "password": "WrongPassword"}`

			resp, err := http.Post(url+"/login", "application/json", strings.NewReader(data))
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

	t.Run("register ok", func(t *testing.T) {
		withTx(pg.Pool, t, func(url string, auth *auth.AuthService) {
			data := `{"login": "nk", "password": "StrongEnoughPassword"}`

			resp, err := http.Post(url+"/register", "application/json", strings.NewReader(data))
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
		withTx(pg.Pool, t, func(url string, auth *auth.AuthService) {
			_, err := auth.Register(t.Context(), "nk", "StrongEnoughPassword")
			require.NoError(t, err)

			data := `{"login": "nk", "password": "StrongEnoughPassword"}`
			resp, err := http.Post(url+"/register", "application/json", strings.NewReader(data))
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

	t.Run("refresh token ok", func(t *testing.T) {
		withTx(pg.Pool, t, func(url string, auth *auth.AuthService) {
			_, err := auth.Register(t.Context(), "nk", "StrongEnoughPassword")
			require.NoError(t, err)

			// Login and get refresh cookie
			data := `{"login": "nk", "password": "StrongEnoughPassword"}`
			resp, err := http.Post(url+"/login", "application/json", strings.NewReader(data))
			require.NoError(t, err)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()
			require.Equal(t, http.StatusOK, resp.StatusCode, "not expected code. Body: %s", string(body))
			require.Equal(t, 1, len(resp.Cookies()))

			// Send refresh request
			firstRefresh := resp.Cookies()[0]
			firstAccess := resp.Header.Get("Authorization")
			req, err := http.NewRequest("POST", url+"/refresh", nil)
			require.NoError(t, err)
			req.AddCookie(&http.Cookie{
				Name:  firstRefresh.Name,
				Value: firstRefresh.Value,
			})
			resp, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			body, err = io.ReadAll(resp.Body)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			require.Equalf(t, http.StatusOK, resp.StatusCode, "not expected code. Body: %s", string(body))
			require.JSONEq(t, `
				{
					"message": "Tokens refreshed successfully"
				}`, string(body))

			require.Equal(t, 1, len(resp.Cookies()))

			secondRefresh := resp.Cookies()[0]
			secondAccess := resp.Header.Get("Authorization")
			require.NotEqual(t, firstRefresh.Value, secondRefresh.Value, "refresh token should be changed after refresh")
			require.NotEqual(t, firstAccess, secondAccess, "access token should be changed after refresh")
		})
	})

	t.Run("refresh refresh twice fail", func(t *testing.T) {
		withTx(pg.Pool, t, func(url string, auth *auth.AuthService) {
			_, err := auth.Register(t.Context(), "nk", "StrongEnoughPassword")
			require.NoError(t, err)

			// Login and get refresh cookie
			data := `{"login": "nk", "password": "StrongEnoughPassword"}`
			resp, err := http.Post(url+"/login", "application/json", strings.NewReader(data))
			require.NoError(t, err)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()
			require.Equal(t, http.StatusOK, resp.StatusCode, "not expected code. Body: %s", string(body))
			require.Equal(t, 1, len(resp.Cookies()))

			refreshCookie := resp.Cookies()[0]

			// Send refresh request
			req, err := http.NewRequest("POST", url+"/refresh", nil)
			require.NoError(t, err)
			req.AddCookie(&http.Cookie{
				Name:  refreshCookie.Name,
				Value: refreshCookie.Value,
			})
			resp, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			body, err = io.ReadAll(resp.Body)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()
			require.Equalf(t, http.StatusOK, resp.StatusCode, "not expected code. Body: %s", string(body))

			// Try to refresh tokens second time
			req, err = http.NewRequest("POST", url+"/refresh", nil)
			require.NoError(t, err)
			req.AddCookie(&http.Cookie{
				Name:  refreshCookie.Name,
				Value: refreshCookie.Value,
			})
			resp, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			body, err = io.ReadAll(resp.Body)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()
			require.Equalf(t, http.StatusUnauthorized, resp.StatusCode, "not expected code. Body: %s", string(body))
			require.JSONEq(t, `
				{
					"error": "service_error",
					"message": "Refresh token not found"
				}`, string(body))
		})
	})

}
