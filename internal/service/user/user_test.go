package user

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/repository"
	"github.com/nkiryanov/gophermart/internal/repository/postgres"
	"github.com/nkiryanov/gophermart/internal/testutil"
)

func TestUser(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	// Helper function to create UserService within transaction
	inTx := func(t *testing.T, fn func(s *UserService, storage repository.Storage)) {
		testutil.InTx(pg.Pool, t, func(tx pgx.Tx) {
			storage := postgres.NewStorage(tx)
			userService := NewService(DefaultHasher, storage)
			fn(userService, storage)
		})
	}

	t.Run("CreateUser", func(t *testing.T) {
		t.Run("create ok", func(t *testing.T) {
			inTx(t, func(s *UserService, _ repository.Storage) {
				user, err := s.CreateUser(t.Context(), "test-user", "password123")

				require.NoError(t, err, "creating new user should be ok")
				require.NotEmpty(t, user.ID, "user ID should not be empty")
				require.Equal(t, "test-user", user.Username, "username should match")
				require.NotEmpty(t, user.HashedPassword, "password hash should not be empty")
				require.NotEqual(t, "password123", user.HashedPassword, "password should be hashed")
				require.NotZero(t, user.CreatedAt, "created at should be set")

				balance, err := s.storage.Balance().GetBalance(t.Context(), user.ID, false)

				require.NoError(t, err, "balance creation should not fail")
				require.Equal(t, user.ID, balance.UserID, "balance user ID should match created")
				require.True(t, balance.Current.IsZero(), "initial balance should be zero")
				require.True(t, balance.Withdrawn.IsZero(), "initial withdrawn should be zero")

			})
		})

		t.Run("empty password fail", func(t *testing.T) {
			inTx(t, func(s *UserService, _ repository.Storage) {
				_, err := s.CreateUser(t.Context(), "test-user", "")

				require.Error(t, err, "creating user with empty password should fail")
			})
		})

		t.Run("create duplicate user fail", func(t *testing.T) {
			inTx(t, func(s *UserService, _ repository.Storage) {
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
			inTx(t, func(s *UserService, _ repository.Storage) {
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
			inTx(t, func(s *UserService, _ repository.Storage) {
				// Create user first
				_, err := s.CreateUser(t.Context(), "test-user", "password123")
				require.NoError(t, err)

				_, err = s.Login(t.Context(), "test-user", "wrong-password")

				require.Error(t, err, "login with wrong password should fail")
				require.ErrorIs(t, err, apperrors.ErrUserNotFound)
			})
		})

		t.Run("not existed user fail", func(t *testing.T) {
			inTx(t, func(s *UserService, _ repository.Storage) {
				_, err := s.Login(t.Context(), "non-existed-user", "password123")

				require.Error(t, err, "login with non-existent user should fail")
				require.ErrorIs(t, err, apperrors.ErrUserNotFound)
			})
		})
	})

	t.Run("GetUserByID", func(t *testing.T) {
		t.Run("existed ok", func(t *testing.T) {
			inTx(t, func(s *UserService, _ repository.Storage) {
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
			inTx(t, func(s *UserService, _ repository.Storage) {
				_, err := s.GetUserByID(t.Context(), uuid.New()) // Non-existent ID

				require.Error(t, err, "getting non-existent user should fail")
				require.ErrorIs(t, err, apperrors.ErrUserNotFound)
			})
		})
	})

	t.Run("GetBalance", func(t *testing.T) {
		t.Run("new user", func(t *testing.T) {
			inTx(t, func(s *UserService, _ repository.Storage) {
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

	t.Run("Withdrawn", func(t *testing.T) {
		// Create initial user with balance 1000
		setup := func(t *testing.T, userService *UserService, storage repository.Storage) models.User {
			user, err := userService.CreateUser(t.Context(), "test-user", "password123")
			require.NoError(t, err, "creating user for withdrawal test should not fail")

			_, err = storage.Balance().UpdateBalance(t.Context(), models.Transaction{
				UserID: user.ID,
				Type:   models.TransactionTypeAccrual,
				Amount: decimal.NewFromInt(1000), // Initial balance for testing
			})
			require.NoError(t, err, "initial balance update should not fail")

			return user
		}

		t.Run("withdrawn insufficient fail", func(t *testing.T) {
			inTx(t, func(s *UserService, storage repository.Storage) {
				user := setup(t, s, storage)

				_, err := s.Withdraw(t.Context(), user.ID, "2444", decimal.NewFromInt(1500)) // Trying to withdraw more than balance

				require.Error(t, err, "withdrawing more than balance should fail")
				require.ErrorIs(t, err, apperrors.ErrBalanceInsufficient)
			})
		})

		t.Run("withdrawn ok", func(t *testing.T) {
			inTx(t, func(s *UserService, storage repository.Storage) {
				user := setup(t, s, storage)

				// Withdraw 900 from balance
				withdrawnAmount := decimal.NewFromInt(900)
				balance, err := s.Withdraw(t.Context(), user.ID, "2444", withdrawnAmount)

				require.NoError(t, err, "withdrawing valid amount should succeed")
				require.True(t, balance.Current.Equal(decimal.NewFromInt(100)), "not expected balance after withdrawal")
				require.Truef(t, balance.Withdrawn.Equal(withdrawnAmount), "withdrawn amount should be %s", withdrawnAmount.String())
			})
		})

		t.Run("withdrawn with invalid number", func(t *testing.T) {
			inTx(t, func(s *UserService, storage repository.Storage) {
				user := setup(t, s, storage)

				_, err := s.Withdraw(t.Context(), user.ID, "1444", decimal.NewFromInt(100))

				require.Error(t, err)
				require.ErrorIs(t, err, apperrors.ErrOrderNumberInvalid, "should return ErrOrderNumberInvalid error")
			})
		})
	})
}
