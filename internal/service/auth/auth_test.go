package auth

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/repository/postgres"
	"github.com/nkiryanov/gophermart/internal/testutil"
)

func Test_Auth(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	cfg := AuthServiceConfig{
		SecretKey:       "secret",
		Hasher:          BcryptHasher{},
		AccessTokenTTL:  5 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
	}

	// Begin new db transaction and create new AuthService
	// Rollback transaction when test stops
	withTx := func(dbpool *pgxpool.Pool, t *testing.T, fn func(s *AuthService)) {
		testutil.WithTx(dbpool, t, func(tx pgx.Tx) {
			s, err := NewService(cfg, &postgres.UserRepo{DB: tx}, &postgres.RefreshTokenRepo{DB: tx})
			require.NoError(t, err, "auth service could't be started", err)

			fn(s)
		})
	}

	t.Run("Register", func(t *testing.T) {
		t.Run("new user ok", func(t *testing.T) {
			withTx(pg.Pool, t, func(s *AuthService) {
				pair, err := s.Register(t.Context(), "nkiryanov", "pwd")

				require.NoError(t, err, "registering new user should be ok")
				require.NotEmpty(t, pair.Access.Value, "access token should not be empty")
				require.NotEmpty(t, pair.Refresh.Value, "refresh token should not be empty")
			})
		})

		t.Run("fail if user exists", func(t *testing.T) {
			withTx(pg.Pool, t, func(s *AuthService) {
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
			withTx(pg.Pool, t, func(s *AuthService) {
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
				withTx(pg.Pool, t, func(s *AuthService) {
					_, err := s.Register(t.Context(), "nkiryanov", "pwd")
					require.NoError(t, err)

					_, err = s.Login(t.Context(), tt.login, tt.password)

					require.Error(t, err)
					require.ErrorIs(t, err, tt.expectedErr)
				})

			})
		}
	})

	t.Run("Refresh", func(t *testing.T) {
		t.Run("refresh once ok", func(t *testing.T) {
			withTx(pg.Pool, t, func(s *AuthService) {
				// Register user and get initial token pair
				initialPair, err := s.Register(t.Context(), "nkiryanov", "pwd")
				require.NoError(t, err)

				// Use refresh token to get new token pair
				newPair, err := s.Refresh(t.Context(), initialPair.Refresh.Value)

				require.NoError(t, err)
				require.NotEqual(t, initialPair.Access.Value, newPair.Access.Value, "new access token should be different")
				require.NotEqual(t, initialPair.Refresh.Value, newPair.Refresh.Value, "new refresh token should be different")
			})
		})

		t.Run("fail if user", func(t *testing.T) {
			withTx(pg.Pool, t, func(s *AuthService) {
				// Register user and get token pair
				initialPair, err := s.Register(t.Context(), "nkiryanov", "pwd")
				require.NoError(t, err)

				// Use refresh token once - should work
				_, err = s.Refresh(t.Context(), initialPair.Refresh.Value)
				require.NoError(t, err)

				// Try to use same refresh token again - should fail
				_, err = s.Refresh(t.Context(), initialPair.Refresh.Value)
				require.Error(t, err)
				require.ErrorIs(t, err, apperrors.ErrRefreshTokenIsUsed, "should return error if token already used")
			})
		})

		t.Run("fail if expired", func(t *testing.T) {
			testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
				cfg := AuthServiceConfig{
					SecretKey:       "secret",
					Hasher:          BcryptHasher{},
					AccessTokenTTL:  50 * time.Millisecond,
					RefreshTokenTTL: 100 * time.Millisecond, // Set refresh token expiration time very low
				}

				s, err := NewService(cfg, &postgres.UserRepo{DB: tx}, &postgres.RefreshTokenRepo{DB: tx})
				require.NoError(t, err, "auth service could't be started", err)

				// Register user and get token pair
				initialPair, err := s.Register(t.Context(), "nkiryanov", "pwd")
				require.NoError(t, err)

				// Move time forward to make sure refresh token is expired
				time.Sleep(100 * time.Millisecond)

				_, err = s.Refresh(t.Context(), initialPair.Refresh.Value)
				require.Error(t, err)
				require.ErrorIs(t, err, apperrors.ErrRefreshTokenExpired, "should return error if token expired")
			})
		})
	})
}
