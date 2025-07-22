package user

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/repository/postgres"
	"github.com/nkiryanov/gophermart/internal/testutil"
)

func TestUser(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	// Helper function to create UserService within transaction
	withTx := func(t *testing.T, fn func(s *UserService)) {
		testutil.InTx(pg.Pool, t, func(tx pgx.Tx) {
			storage := postgres.NewStorage(tx)
			userService := NewService(DefaultHasher, storage)
			fn(userService)
		})
	}

	t.Run("CreateUser", func(t *testing.T) {
		t.Run("create ok", func(t *testing.T) {
			withTx(t, func(s *UserService) {
				user, err := s.CreateUser(t.Context(), "test-user", "password123")

				require.NoError(t, err, "creating new user should be ok")
				require.NotEmpty(t, user.ID, "user ID should not be empty")
				require.Equal(t, "test-user", user.Username, "username should match")
				require.NotEmpty(t, user.HashedPassword, "password hash should not be empty")
				require.NotEqual(t, "password123", user.HashedPassword, "password should be hashed")
				require.NotZero(t, user.CreatedAt, "created at should be set")

				balance, err := s.storage.Balance().GetBalance(t.Context(), user.ID)

				require.NoError(t, err, "balance creation should not fail")
				require.Equal(t, user.ID, balance.UserID, "balance user ID should match created")
				require.True(t, balance.Current.IsZero(), "initial balance should be zero")
				require.True(t, balance.Withdrawn.IsZero(), "initial withdrawn should be zero")

			})
		})

		t.Run("empty password fail", func(t *testing.T) {
			withTx(t, func(s *UserService) {
				_, err := s.CreateUser(t.Context(), "test-user", "")

				require.Error(t, err, "creating user with empty password should fail")
			})
		})

		t.Run("create duplicate user fail", func(t *testing.T) {
			withTx(t, func(s *UserService) {
				_, err := s.CreateUser(t.Context(), "test-user", "password123")
				require.NoError(t, err, "first user creation should succeed")

				_, err = s.CreateUser(t.Context(), "test-user", "different_password")

				require.Error(t, err, "creating duplicate user should fail")
				require.ErrorIs(t, err, apperrors.ErrUserAlreadyExists)
			})
		})
	})

	t.Run("Login", func(t *testing.T) {
		t.Run("login ok", func(t *testing.T) {
			withTx(t, func(s *UserService) {
				// Create user first
				createdUser, err := s.CreateUser(t.Context(), "test-user", "password123")
				require.NoError(t, err)

				user, err := s.Login(t.Context(), "test-user", "password123")

				require.NoError(t, err, "login with correct credentials should succeed")
				require.Equal(t, createdUser.ID, user.ID, "user ID should match")
				require.Equal(t, createdUser.Username, user.Username, "username should match")
				require.Equal(t, createdUser.HashedPassword, user.HashedPassword, "password hash should match")
			})
		})

		t.Run("invalid password fail", func(t *testing.T) {
			withTx(t, func(s *UserService) {
				// Create user first
				_, err := s.CreateUser(t.Context(), "test-user", "password123")
				require.NoError(t, err)

				_, err = s.Login(t.Context(), "test-user", "wrong-password")

				require.Error(t, err, "login with wrong password should fail")
				require.ErrorIs(t, err, apperrors.ErrUserNotFound)
			})
		})

		t.Run("not existed user fail", func(t *testing.T) {
			withTx(t, func(s *UserService) {
				_, err := s.Login(t.Context(), "non-existed-user", "password123")

				require.Error(t, err, "login with non-existent user should fail")
				require.ErrorIs(t, err, apperrors.ErrUserNotFound)
			})
		})
	})

	t.Run("GetUserByID", func(t *testing.T) {
		t.Run("existed ok", func(t *testing.T) {
			withTx(t, func(s *UserService) {
				// Create user first
				createdUser, err := s.CreateUser(t.Context(), "test-user", "password123")
				require.NoError(t, err)

				user, err := s.GetUserByID(t.Context(), createdUser.ID)

				require.NoError(t, err, "getting existing user by ID should succeed")
				require.Equal(t, createdUser.ID, user.ID, "user ID should match")
				require.Equal(t, createdUser.Username, user.Username, "username should match")
				require.Equal(t, createdUser.HashedPassword, user.HashedPassword, "password hash should match")
				require.Equal(t, createdUser.CreatedAt, user.CreatedAt, "created at should match")
			})
		})

		t.Run("not existed fail", func(t *testing.T) {
			withTx(t, func(s *UserService) {
				_, err := s.GetUserByID(t.Context(), uuid.New()) // Non-existent ID

				require.Error(t, err, "getting non-existent user should fail")
				require.ErrorIs(t, err, apperrors.ErrUserNotFound)
			})
		})
	})

	t.Run("GetBalance", func(t *testing.T) {
		t.Run("new user", func(t *testing.T) {
			withTx(t, func(s *UserService) {
				// Create user first
				createdUser, err := s.CreateUser(t.Context(), "test-user", "password123")
				require.NoError(t, err)

				balance, err := s.GetBalance(t.Context(), createdUser.ID)

				require.NoError(t, err, "getting balance for new user should succeed")
				require.Equal(t, createdUser.ID, balance.UserID, "balance user ID should match")
				require.True(t, balance.Current.IsZero(), "initial balance should be zero")
				require.True(t, balance.Withdrawn.IsZero(), "initial withdrawn should be zero")
			})
		})
	})
}
