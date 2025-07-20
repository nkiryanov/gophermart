package middleware

import (
	"context"
	"errors"
	"testing"

	"io"
	"net/http"
	"net/http/httptest"

	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/handlers"
	"github.com/nkiryanov/gophermart/internal/models"
)

// Allow to use a function as auth service
type authFunc func(ctx context.Context, r *http.Request) (models.User, error)

func (f authFunc) Auth(ctx context.Context, r *http.Request) (models.User, error) {
	return f(ctx, r)
}

func TestAuthMiddleware_Auth(t *testing.T) {
	// Simple handler that try to get user from context
	// If ok write it username to response
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Must always be true cause middleware has to set user to response or write error to response
		user, ok := handlers.UserFromContext(r.Context())
		require.True(t, ok)

		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(user.Username))
		require.NoError(t, err, "should write username to response")
	})

	t.Run("auth ok", func(t *testing.T) {
		// Middleware that always return ok
		middleware := NewAuth(authFunc(func(ctx context.Context, r *http.Request) (models.User, error) {
			return models.User{Username: "test-user"}, nil
		}))

		srv := httptest.NewServer(middleware.Auth(handler))
		defer srv.Close()

		resp, err := http.Get(srv.URL + "/test")
		require.NoError(t, err, "should make request to test server")
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "should read response body")
		defer resp.Body.Close() // nolint:errcheck

		require.Equalf(t, http.StatusOK, resp.StatusCode, "should return status OK. Resp: %s", string(body))
		require.Equal(t, "test-user", string(body), "should return username in response")
	})

	t.Run("auth fail", func(t *testing.T) {
		// Middleware that always fails
		middleware := NewAuth(authFunc(func(ctx context.Context, r *http.Request) (models.User, error) {
			return models.User{}, errors.New("fuck off!")
		}))

		srv := httptest.NewServer(middleware.Auth(handler))
		defer srv.Close()

		resp, err := http.Get(srv.URL + "/test")
		require.NoError(t, err, "should make request to test server")
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "should read response body")
		defer resp.Body.Close() // nolint:errcheck

		require.Equalf(t, http.StatusUnauthorized, resp.StatusCode, "should return status Unauthorized. Resp: %s", string(body))
		require.JSONEq(t,
			`{
				"error": "service_error",
				"message": "Unauthorized"
			}`,
			string(body),
		)
	})
}
