package auth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/repository/postgres"
	"github.com/nkiryanov/gophermart/internal/service/auth/tokenmanager"
	"github.com/nkiryanov/gophermart/internal/service/user"
	"github.com/nkiryanov/gophermart/internal/testutil"
)

func Test_Auth(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	// Begin new db transaction and create new AuthService
	// Rollback transaction when test stops
	inTx := func(pool *pgxpool.Pool, accessTTL time.Duration, refreshTTL time.Duration, t *testing.T, fn func(s *AuthService)) {
		testutil.InTx(pool, t, func(tx pgx.Tx) {
			storage := postgres.NewStorage(tx)

			tokenManager, err := tokenmanager.New(
				tokenmanager.Config{
					SecretKey:  "test-secret-key",
					AccessTTL:  accessTTL,
					RefreshTTL: refreshTTL,
				},
				storage,
			)
			require.NoError(t, err, "token manager should be created without errors")

			userService := user.NewService(user.DefaultHasher, storage)

			s, err := NewService(Config{}, tokenManager, userService)
			require.NoError(t, err, "auth service could't be started", err)

			fn(s)
		})
	}

	t.Run("new auth service defaults", func(t *testing.T) {
		s, err := NewService(Config{}, nil, nil)
		require.NoError(t, err, "auth service should be created without errors")

		require.Equal(t, defaultAccessHeaderName, s.accessHeaderName, "default access header name should be set")
		require.Equal(t, defaultAccessAuthScheme, s.accessAuthScheme, "default access auth")
		require.Equal(t, defaultRefreshCookieName, s.refreshCookieName, "default refresh cookie name should be set")
	})

	t.Run("Register", func(t *testing.T) {
		t.Run("new user ok", func(t *testing.T) {
			inTx(pg.Pool, 15*time.Minute, 24*time.Hour, t, func(s *AuthService) {
				pair, err := s.Register(t.Context(), "nkiryanov", "pwd")

				require.NoError(t, err, "registering new user should be ok")
				require.NotEmpty(t, pair.Access.Value, "access token should not be empty")
				require.NotEmpty(t, pair.Refresh.Value, "refresh token should not be empty")
			})
		})

		t.Run("fail if user exists", func(t *testing.T) {
			inTx(pg.Pool, 15*time.Minute, 24*time.Hour, t, func(s *AuthService) {
				_, err := s.Register(t.Context(), "nkiryanov", "pwd")
				require.NoError(t, err, "no error has should happen if user not exists")

				_, err = s.Register(t.Context(), "nkiryanov", "other-pwd")

				require.Error(t, err)
				require.ErrorIs(t, err, apperrors.ErrUserAlreadyExists)
			})
		})
	})

	t.Run("Login", func(t *testing.T) {
		t.Run("existing user ok", func(t *testing.T) {
			inTx(pg.Pool, 15*time.Minute, 24*time.Hour, t, func(s *AuthService) {
				_, err := s.Register(t.Context(), "nkiryanov", "pwd")
				require.NoError(t, err)

				pair, err := s.Login(t.Context(), "nkiryanov", "pwd")

				require.NoError(t, err)
				require.NotEmpty(t, pair.Access.Value, "access token should not be empty")
				require.NotEmpty(t, pair.Refresh.Value, "refresh token should not be empty")
			})
		})

		tests := []struct {
			name        string
			login       string
			password    string
			expectedErr error
		}{
			{
				name:        "login fail if wrong password",
				login:       "nkiryanov",
				password:    "wrong",
				expectedErr: apperrors.ErrUserNotFound,
			},
			{
				name:        "login fail if user not exists",
				login:       "not-existed-user",
				password:    "password",
				expectedErr: apperrors.ErrUserNotFound,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				inTx(pg.Pool, 15*time.Minute, 24*time.Hour, t, func(s *AuthService) {
					_, err := s.Register(t.Context(), "nkiryanov", "pwd")
					require.NoError(t, err)

					_, err = s.Login(t.Context(), tt.login, tt.password)

					require.Error(t, err)
					require.ErrorIs(t, err, tt.expectedErr)
				})

			})
		}
	})

	t.Run("RefreshPair", func(t *testing.T) {
		t.Run("refresh once ok", func(t *testing.T) {
			inTx(pg.Pool, 15*time.Minute, 24*time.Hour, t, func(s *AuthService) {
				// Register user and get initial token pair
				initialPair, err := s.Register(t.Context(), "nkiryanov", "pwd")
				require.NoError(t, err)

				// Use refresh token to get new token pair
				newPair, err := s.RefreshPair(t.Context(), initialPair.Refresh.Value)

				require.NoError(t, err)
				require.NotEqual(t, initialPair.Access.Value, newPair.Access.Value, "new access token should be different")
				require.NotEqual(t, initialPair.Refresh.Value, newPair.Refresh.Value, "new refresh token should be different")
			})
		})

		t.Run("fail if used once", func(t *testing.T) {
			inTx(pg.Pool, 15*time.Minute, 24*time.Hour, t, func(s *AuthService) {
				// Register user and get token pair
				initialPair, err := s.Register(t.Context(), "nkiryanov", "pwd")
				require.NoError(t, err)

				// Use refresh token once - should work
				_, err = s.RefreshPair(t.Context(), initialPair.Refresh.Value)
				require.NoError(t, err)

				// Try to use same refresh token again - should fail
				_, err = s.RefreshPair(t.Context(), initialPair.Refresh.Value)
				require.Error(t, err)
				require.ErrorIs(t, err, apperrors.ErrRefreshTokenIsUsed, "should return error if token already used")
			})
		})

		t.Run("fail if expired", func(t *testing.T) {
			inTx(pg.Pool, 1*time.Second, 1*time.Second, t, func(s *AuthService) {
				// Register user and get token pair
				initialPair, err := s.Register(t.Context(), "nkiryanov", "pwd")
				require.NoError(t, err)

				// Move time forward to make sure refresh token is expired
				time.Sleep(time.Second)

				_, err = s.RefreshPair(t.Context(), initialPair.Refresh.Value)
				require.Error(t, err)
				require.ErrorIs(t, err, apperrors.ErrRefreshTokenExpired, "should return error if token expired")
			})
		})
	})

	t.Run("SetTokenPairToResponse", func(t *testing.T) {
		inTx(pg.Pool, 15*time.Minute, 24*time.Hour, t, func(s *AuthService) {
			// Create new valid token pair
			pair, err := s.Register(t.Context(), "nkiryanov", "pwd")
			require.NoError(t, err)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				s.SetTokenPairToResponse(w, pair)
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte("ok"))
				require.NoError(t, err)
			}))
			defer srv.Close()

			resp, err := http.Get(srv.URL + "/test")
			require.NoError(t, err, "should not return an error when writing token pair")
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err, "should not return an error when reading response body")
			defer func() { _ = resp.Body.Close() }()
			require.Equalf(t, http.StatusOK, resp.StatusCode, "not expected code. Body: %s", string(body))

			// Verify refresh token set as cookie
			require.Equal(t, 1, len(resp.Cookies()))
			refreshCookie := resp.Cookies()[0]
			require.Equal(t, s.refreshCookieName, refreshCookie.Name, "cookie name should match expected")
			require.Equal(t, pair.Refresh.Value, refreshCookie.Value, "cookie value should match refresh token value")
			require.Equal(t, http.SameSiteStrictMode, refreshCookie.SameSite, "cookie should be SameSite Strict")
			require.InDelta(t, (24 * time.Hour).Seconds(), refreshCookie.MaxAge, 1, "cookie max age should match refresh TTL with 1 second delta")
			require.Equal(t, "/", refreshCookie.Path)

			// Verify access token set in Authorization header
			require.Equal(t, "Bearer "+pair.Access.Value, resp.Header.Get("Authorization"), "Authorization header should be set with access token")
		})
	})

	t.Run("GetRefreshString", func(t *testing.T) {
		inTx(pg.Pool, 15*time.Minute, 24*time.Hour, t, func(s *AuthService) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				token, err := s.GetRefreshString(r)
				if err != nil {
					http.Error(w, "fuck off", http.StatusBadRequest)
					return
				}

				w.WriteHeader(http.StatusOK)
				_, err = w.Write([]byte(token))
				require.NoError(t, err, "should write refresh token to response")
			}))

			t.Run("ok if refresh cookie", func(t *testing.T) {
				req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
				require.NoError(t, err, "should create request without errors")
				req.AddCookie(&http.Cookie{Name: s.refreshCookieName, Value: "test-refresh-token"})

				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err, "should send request without errors")
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err, "should read response body without errors")
				defer func() { _ = resp.Body.Close() }()

				require.Equal(t, http.StatusOK, resp.StatusCode, "should return 200 OK status")
				require.Equal(t, "test-refresh-token", string(body))
			})

			t.Run("fail if no cookie", func(t *testing.T) {
				req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
				require.NoError(t, err, "should create request without errors")

				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err, "should send request without errors")
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err, "should read response body without errors")
				defer func() { _ = resp.Body.Close() }()

				require.Equal(t, http.StatusBadRequest, resp.StatusCode)
				require.Equal(t, "fuck off\n", string(body))
			})
		})
	})

	t.Run("GetUserFromRequest", func(t *testing.T) {
		inTx(pg.Pool, time.Second, time.Hour, t, func(s *AuthService) {
			_, err := s.Register(t.Context(), "nk", "pwd")
			require.NoError(t, err)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, err := s.GetUserFromRequest(t.Context(), r)
				if err != nil {
					http.Error(w, "fuck off", http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusOK)
				_, err = w.Write([]byte(user.Username))
				require.NoError(t, err, "should write refresh token to response")
			}))
			defer srv.Close()

			t.Run("ok if token valid", func(t *testing.T) {
				pair, err := s.Login(t.Context(), "nk", "pwd")
				require.NoError(t, err)

				req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+pair.Access.Value)

				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				defer func() { _ = resp.Body.Close() }()

				require.Equal(t, http.StatusOK, resp.StatusCode)
				require.Equal(t, "nk", string(body))
			})

			t.Run("fail if invalid scheme", func(t *testing.T) {
				pair, err := s.Login(t.Context(), "nk", "pwd")
				require.NoError(t, err)

				// Send request with invalid auth scheme (e.g. "JWT" instead of "Bearer")
				req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", "JWT "+pair.Access.Value)

				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				defer func() { _ = resp.Body.Close() }()

				require.Equal(t, http.StatusBadRequest, resp.StatusCode)
				require.Equal(t, "fuck off\n", string(body))
			})

			t.Run("fail if token invalid", func(t *testing.T) {
				req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer not-a-token")

				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				defer func() { _ = resp.Body.Close() }()

				require.Equal(t, http.StatusBadRequest, resp.StatusCode)
				require.Equal(t, "fuck off\n", string(body))
			})

		})
	})

}
