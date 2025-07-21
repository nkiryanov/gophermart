package postgres

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/jackc/pgx/v5"
	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/repository"
	"github.com/nkiryanov/gophermart/internal/testutil"
)

func TestOrders(t *testing.T) {
	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	// Create transaction and repository base on it
	// May be called several times(aka transaction in transaction)
	withTx := func(t *testing.T, tx DBTX, fn func(pgx.Tx, *repository.Repos)) {
		testutil.WithTx(tx, t, func(ttx pgx.Tx) {
			repos := &repository.Repos{
				UserRepo:    &UserRepo{DB: ttx},
				OrderRepo:   &OrderRepo{DB: ttx},
				RefreshRepo: nil,
			}

			fn(ttx, repos)
		})
	}

	t.Run("CreateOrder", func(t *testing.T) {
		withTx(t, pg.Pool, func(tx pgx.Tx, repos *repository.Repos) {
			user, err := repos.UserRepo.CreateUser(t.Context(), "testuser", "hashedpassword")
			require.NoError(t, err)

			t.Run("create ok", func(t *testing.T) {
				withTx(t, tx, func(ttx pgx.Tx, repos *repository.Repos) {
					order, err := repos.OrderRepo.CreateOrder(t.Context(), "123", user.ID)

					require.NoError(t, err, "order has to be created ok")

					require.NotZero(t, order.ID)
					require.Equal(t, "123", order.Number)
					require.Equal(t, user.ID, order.UserID)
					require.WithinDuration(t, time.Now(), order.UploadedAt, time.Second)
					require.WithinDuration(t, time.Now(), order.ModifiedAt, time.Second)
					require.Equal(t, models.OrderNew, order.Status)
					require.True(t, order.Accrual.IsZero(), "accrual should be zero for new orders")
				})
			})

			t.Run("create twice", func(t *testing.T) {
				withTx(t, tx, func(ttx pgx.Tx, repos *repository.Repos) {
					_, err := repos.OrderRepo.CreateOrder(t.Context(), "123", user.ID)
					require.NoError(t, err, "order has to be created ok")

					// Crete order second time but with different status
					_, err = repos.OrderRepo.CreateOrder(t.Context(), "123", user.ID, models.WithOrderStatus(models.OrderProcessed))

					require.Error(t, err, "crating same order must failed")
					require.ErrorIs(t, err, apperrors.ErrOrderAlreadyExists, "should return well known error")
				})
			})

			t.Run("create conflict", func(t *testing.T) {
				withTx(t, tx, func(ttx pgx.Tx, repos *repository.Repos) {
					_, err := repos.OrderRepo.CreateOrder(t.Context(), "123", user.ID)
					require.NoError(t, err, "order has to be created ok")
					yaUser, err := repos.UserRepo.CreateUser(t.Context(), "anotheruser", "hashedpassword")
					require.NoError(t, err)

					// Crete order second time but with different status
					_, err = repos.OrderRepo.CreateOrder(t.Context(), "123", yaUser.ID, models.WithOrderStatus(models.OrderProcessed))

					require.Error(t, err, "crating same order must failed")
					require.ErrorIs(t, err, apperrors.ErrOrderNumberTaken, "should return well known error")
				})
			})

		})
	})

	t.Run("ListOrders", func(t *testing.T) {
		withTx(t, pg.Pool, func(tx pgx.Tx, repos *repository.Repos) {
			user, err := repos.UserRepo.CreateUser(t.Context(), "user1", "hashedpassword")
			require.NoError(t, err)

			t.Run("empty list", func(t *testing.T) {
				withTx(t, tx, func(ttx pgx.Tx, repos *repository.Repos) {
					orders, err := repos.OrderRepo.ListOrders(t.Context(), user.ID)

					require.NoError(t, err, "listing orders should not fail")
					require.Empty(t, orders, "orders list should be empty for new user")
				})
			})

			t.Run("single order", func(t *testing.T) {
				withTx(t, tx, func(ttx pgx.Tx, repos *repository.Repos) {
					createdOrder, err := repos.OrderRepo.CreateOrder(t.Context(), "456", user.ID)
					require.NoError(t, err)

					orders, err := repos.OrderRepo.ListOrders(t.Context(), user.ID)
					require.NoError(t, err, "listing orders should not fail")

					require.Len(t, orders, 1, "should return exactly one order")
					order := orders[0]
					require.Equal(t, createdOrder.ID, order.ID)
					require.Equal(t, createdOrder.Number, order.Number)
					require.Equal(t, createdOrder.UserID, order.UserID)
					require.Equal(t, createdOrder.Status, order.Status)
					require.Equal(t, createdOrder.Accrual, order.Accrual)
				})
			})

			t.Run("multiple orders", func(t *testing.T) {
				withTx(t, tx, func(ttx pgx.Tx, repos *repository.Repos) {
					order, err := repos.OrderRepo.CreateOrder(t.Context(), "111", user.ID)
					require.NoError(t, err)
					yaOrder, err := repos.OrderRepo.CreateOrder(t.Context(), "222", user.ID,
						models.WithOrderStatus(models.OrderProcessed),
						models.WithOrderAccrual(decimal.RequireFromString("100.50")),
					)
					require.NoError(t, err)

					orders, err := repos.OrderRepo.ListOrders(t.Context(), user.ID)
					require.NoError(t, err, "listing orders should not fail")

					require.Len(t, orders, 2)
					require.Equal(t, yaOrder.ID, orders[0].ID)
					require.Equal(t, yaOrder.Status, orders[0].Status)
					require.Equal(t, yaOrder.Accrual, orders[0].Accrual)
					require.Equal(t, order.ID, orders[1].ID)
					require.Equal(t, order.Status, orders[1].Status)
					require.Equal(t, order.Accrual, orders[1].Accrual)
				})
			})

			t.Run("nonexistent user", func(t *testing.T) {
				withTx(t, tx, func(ttx pgx.Tx, repos *repository.Repos) {
					orders, err := repos.OrderRepo.ListOrders(t.Context(), uuid.New())

					require.NoError(t, err, "listing orders for nonexistent user should not fail")
					require.Empty(t, orders, "should return empty list for nonexistent user")
				})
			})
		})
	})
}
