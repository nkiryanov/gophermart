package order

import (
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/repository/postgres"
	"github.com/nkiryanov/gophermart/internal/service/user"
	"github.com/nkiryanov/gophermart/internal/testutil"
)

func TestOrder(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	// Helper function to create OrderService within transaction
	withTx := func(t *testing.T, fn func(s *OrderService, user *models.User, yaUser *models.User)) {
		testutil.InTx(pg.Pool, t, func(tx pgx.Tx) {
			storage := postgres.NewStorage(tx)
			orderService := NewService(storage)

			// Create users for tests purpose
			userService := user.NewService(user.DefaultHasher, storage)
			user, err := userService.CreateUser(t.Context(), "test-user", "password123")
			require.NoError(t, err, "creating user should not fail")
			yaUser, err := userService.CreateUser(t.Context(), "ya-user", "password123")
			require.NoError(t, err, "creating ya-user should not fail")

			fn(orderService, &user, &yaUser)
		})
	}

	t.Run("CreateOrder", func(t *testing.T) {
		t.Run("create valid number ok", func(t *testing.T) {
			withTx(t, func(s *OrderService, user *models.User, _ *models.User) {
				order, err := s.CreateOrder(t.Context(), "17893729974", user)

				require.NoError(t, err, "creating order should not fail")
				require.NotEmpty(t, order.ID, "order ID should not be empty")
				require.Equal(t, "17893729974", order.Number, "order number should match provided number")
				require.Equal(t, user.ID, order.UserID, "order user ID should match created user")
				require.Equal(t, models.OrderNew, order.Status, "order status new by default")
				require.NotZero(t, order.UploadedAt, "order uploaded at should be set")
				require.NotZero(t, order.ModifiedAt, "order modified at should be set")
				require.True(t, order.Accrual.IsZero(), "order accrual should be zero by default")
			})
		})

		t.Run("invalid number fail", func(t *testing.T) {
			withTx(t, func(s *OrderService, user *models.User, _ *models.User) {
				_, err := s.CreateOrder(t.Context(), "1234567890", user)

				require.Error(t, err, "creating order with invalid number should fail")
				require.ErrorIs(t, err, apperrors.ErrOrderNumberInvalid)
			})
		})

		t.Run("error if already exists", func(t *testing.T) {
			withTx(t, func(s *OrderService, user *models.User, _ *models.User) {
				_, err := s.CreateOrder(t.Context(), "17893729974", user)
				require.NoError(t, err, "creating order should not fail on first call")

				_, err = s.CreateOrder(t.Context(), "17893729974", user)

				require.Error(t, err, "creating order with existing number should fail")
				require.ErrorIs(t, err, apperrors.ErrOrderAlreadyExists)
			})
		})

		t.Run("error if already taken", func(t *testing.T) {
			withTx(t, func(s *OrderService, user *models.User, yaUser *models.User) {
				_, err := s.CreateOrder(t.Context(), "17893729974", user)
				require.NoError(t, err, "creating order should not fail on first call")

				_, err = s.CreateOrder(t.Context(), "17893729974", yaUser)

				require.Error(t, err, "creating order with existing number should fail")
				require.ErrorIs(t, err, apperrors.ErrOrderNumberTaken)
			})
		})
	})
}
