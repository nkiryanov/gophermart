package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/testutil"
	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/repository/postgres"
)

type dummyHasher struct {}
func (h dummyHasher) Hash(password string) (string, error) {
	return password, nil
}
func (h dummyHasher) Compare(knownGoodPassword string, userProvided string) (string error) {
	if userProvided != knownGoodPassword {
		return errors.New("wrong password")
	}

	return nil
}

func Test_Auth(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	cfg := AuthServiceConfig{
		SecretKey: "secret",
		Hasher: dummyHasher{},
		AccessTokenTTL: 5*time.Minute,
		RefreshTokenTTL: 24*time.Hour,
	}

	// Begin new db transaction and create new AuthService
	// Rollback transaction when test stops
	withAuthTx := func(dbpool *pgxpool.Pool, t *testing.T, fn func(s *AuthService)) {
		testutil.WithTx(dbpool, t, func(tx pgx.Tx) {
			s, err := NewAuthService(cfg, &postgres.UserRepo{DB: tx}, &postgres.RefreshTokenRepo{DB: tx})
			require.NoError(t, err, "auth service could't be started", err)

			fn(s)
		})
	}

	t.Run("register new user ok", func(t *testing.T) {
		withAuthTx(pg.Pool, t, func(s *AuthService) {
			pair, err := s.Register(t.Context(), "nkiryanov", "pwd")

			require.NoError(t, err, "registering new user should be ok")
			require.NotEmpty(t, pair.Access, "access token should not be empty")
			require.NotEmpty(t, pair.Refresh, "refresh token should not be empty")
		})
	})

	t.Run("register fail if user exists", func(t *testing.T) {
		withAuthTx(pg.Pool, t, func(s *AuthService) {
			_, err := s.Register(t.Context(), "nkiryanov", "pwd")
			require.NoError(t, err, "no error has should happen if user not exists")

			_, err = s.Register(t.Context(), "nkiryanov", "other-pwd")
			require.Error(t, err)
			require.ErrorIs(t, err, apperrors.ErrUserAlreadyExists)
		})
	})

	t.Run("login ok", func(t *testing.T) {
		withAuthTx(pg.Pool, t, func(s *AuthService) {
			_, err := s.Register(t.Context(), "nkiryanov", "pwd")
			require.NoError(t, err)

			pair, err := s.Login(t.Context(), "nkiryanov", "pwd")
			require.NoError(t, err)
			require.NotEmpty(t, pair.Access, "access token should not be empty")
			require.NotEmpty(t, pair.Refresh, "refresh token should not be empty")

		})
	})

	tests := []struct{
		name string
		login string
		password string
		expectedErr error
	}{
		{"login fail if wrong password", "nkiryanov", "wrong", apperrors.ErrUserNotFound},
		{"login fail if user not exists", "not-existed-user", "password", apperrors.ErrUserNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withAuthTx(pg.Pool, t, func(s *AuthService) {
				_, err := s.Register(t.Context(), "nkiryanov", "pwd")
				require.NoError(t, err)

				_, err = s.Login(t.Context(), tt.login, tt.password)

				require.Error(t, err)
				require.ErrorIs(t, err, tt.expectedErr)
			})

		})
	}
}
