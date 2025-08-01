package order

import (
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/repository"
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
				require.Equal(t, models.OrderStatusNew, order.Status, "order status new by default")
				require.NotZero(t, order.UploadedAt, "order uploaded at should be set")
				require.NotZero(t, order.ModifiedAt, "order modified at should be set")
				require.Nil(t, order.Accrual, "order accrual should be nil for new orders")
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

	t.Run("SetProcessed", func(t *testing.T) {
		t.Run("order can be set to processed", func(t *testing.T) {
			withTx(t, func(s *OrderService, user *models.User, _ *models.User) {
				order, err := s.CreateOrder(t.Context(), "17893729974", user)
				require.NoError(t, err, "creating order should not fail")
				require.Equal(t, models.OrderStatusNew, order.Status, "order should be new initially")

				// Set order as processed
				accrual := decimal.RequireFromString("100.50")
				updatedOrder, err := s.SetProcessed(t.Context(), "17893729974", models.OrderStatusProcessed, &accrual)

				require.NoError(t, err, "setting order as processed should not fail")
				require.Equal(t, models.OrderStatusProcessed, updatedOrder.Status, "order status should be processed")
				require.NotNil(t, updatedOrder.Accrual, "order accrual should be set")
				require.True(t, updatedOrder.Accrual.Equal(accrual), "order accrual should match provided amount")
			})
		})

		t.Run("order in invalid status cannot be updated", func(t *testing.T) {
			withTx(t, func(s *OrderService, user *models.User, _ *models.User) {
				// Create order first
				order, err := s.CreateOrder(t.Context(), "17893729974", user, repository.WithOrderStatus(models.OrderStatusInvalid))
				require.NoError(t, err, "creating order should not fail")

				_, err = s.SetProcessed(t.Context(), order.Number, models.OrderStatusProcessed, nil)

				require.Error(t, err, "updating already invalid order should fail")
				require.ErrorIs(t, err, apperrors.ErrOrderAlreadyProcessed, "should return ErrOrderAlreadyProcessed error")
			})
		})
	})
}
