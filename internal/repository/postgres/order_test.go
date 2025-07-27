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
	inTx := func(t *testing.T, outerTx DBTX, fn func(pgx.Tx, repository.Storage)) {
		testutil.InTx(outerTx, t, func(innerTx pgx.Tx) {
			storage := NewStorage(innerTx)
			fn(innerTx, storage)
		})
	}

	t.Run("CreateOrder", func(t *testing.T) {
		inTx(t, pg.Pool, func(tx pgx.Tx, storage repository.Storage) {
			user, err := storage.User().CreateUser(t.Context(), "testuser", "hashedpassword")
			require.NoError(t, err)

			t.Run("create ok", func(t *testing.T) {
				inTx(t, tx, func(_ pgx.Tx, storage repository.Storage) {
					order, err := storage.Order().CreateOrder(t.Context(), "123", user.ID)

					require.NoError(t, err, "order has to be created ok")

					require.NotZero(t, order.ID)
					require.Equal(t, "123", order.Number)
					require.Equal(t, user.ID, order.UserID)
					require.WithinDuration(t, time.Now(), order.UploadedAt, time.Second)
					require.WithinDuration(t, time.Now(), order.ModifiedAt, time.Second)
					require.Equal(t, models.OrderStatusNew, order.Status)
					require.Nil(t, order.Accrual, "accrual should be nil for new orders")
				})
			})

			t.Run("create twice", func(t *testing.T) {
				inTx(t, tx, func(_ pgx.Tx, storage repository.Storage) {
					_, err := storage.Order().CreateOrder(t.Context(), "123", user.ID)
					require.NoError(t, err, "order has to be created ok")

					// Crete order second time but with different status
					_, err = storage.Order().CreateOrder(t.Context(), "123", user.ID, repository.WithOrderStatus(models.OrderStatusProcessed))

					require.Error(t, err, "crating same order must failed")
					require.ErrorIs(t, err, apperrors.ErrOrderAlreadyExists, "should return well known error")
				})
			})

			t.Run("create conflict", func(t *testing.T) {
				inTx(t, tx, func(_ pgx.Tx, storage repository.Storage) {
					_, err := storage.Order().CreateOrder(t.Context(), "123", user.ID)
					require.NoError(t, err, "order has to be created ok")
					yaUser, err := storage.User().CreateUser(t.Context(), "anotheruser", "hashedpassword")
					require.NoError(t, err)

					// Crete order second time but with different status
					_, err = storage.Order().CreateOrder(t.Context(), "123", yaUser.ID, repository.WithOrderStatus(models.OrderStatusProcessed))

					require.Error(t, err, "crating same order must failed")
					require.ErrorIs(t, err, apperrors.ErrOrderNumberTaken, "should return well known error")
				})
			})

		})
	})

	t.Run("ListOrders", func(t *testing.T) {
		inTx(t, pg.Pool, func(tx pgx.Tx, storage repository.Storage) {
			user, err := storage.User().CreateUser(t.Context(), "user1", "hashedpassword")
			require.NoError(t, err)

			t.Run("empty list", func(t *testing.T) {
				inTx(t, tx, func(_ pgx.Tx, storage repository.Storage) {
					orders, err := storage.Order().ListOrders(t.Context(), repository.ListOrdersOpts{UserID: &user.ID})

					require.NoError(t, err, "listing orders should not fail")
					require.Empty(t, orders, "orders list should be empty for new user")
				})
			})

			t.Run("single order", func(t *testing.T) {
				inTx(t, tx, func(_ pgx.Tx, storage repository.Storage) {
					createdOrder, err := storage.Order().CreateOrder(t.Context(), "456", user.ID, repository.WithOrderAccrual(decimal.RequireFromString("100.50")))
					require.NoError(t, err)

					orders, err := storage.Order().ListOrders(t.Context(), repository.ListOrdersOpts{UserID: &user.ID})
					require.NoError(t, err, "listing orders should not fail")

					require.Len(t, orders, 1, "should return exactly one order")
					order := orders[0]
					require.Equal(t, createdOrder.ID, order.ID)
					require.Equal(t, createdOrder.Number, order.Number)
					require.Equal(t, createdOrder.UserID, order.UserID)
					require.Equal(t, createdOrder.Status, order.Status)
					require.NotNil(t, order.Accrual, "accrual should not be nil")
					require.True(t, createdOrder.Accrual.Equal(*order.Accrual), "accrual should match created order")
				})
			})

			t.Run("multiple orders", func(t *testing.T) {
				inTx(t, tx, func(_ pgx.Tx, storage repository.Storage) {
					order, err := storage.Order().CreateOrder(t.Context(), "111", user.ID)
					require.NoError(t, err)
					yaOrder, err := storage.Order().CreateOrder(t.Context(), "222", user.ID)
					require.NoError(t, err)

					orders, err := storage.Order().ListOrders(t.Context(), repository.ListOrdersOpts{UserID: &user.ID})
					require.NoError(t, err, "listing orders should not fail")

					require.Len(t, orders, 2)
					require.Equal(t, yaOrder.ID, orders[0].ID)
					require.Equal(t, order.ID, orders[1].ID)
				})
			})

			t.Run("nonexistent user", func(t *testing.T) {
				inTx(t, tx, func(ttx pgx.Tx, storage repository.Storage) {
					userID := uuid.New() // Nonexistent user ID
					orders, err := storage.Order().ListOrders(t.Context(), repository.ListOrdersOpts{UserID: &userID})

					require.NoError(t, err, "listing orders for nonexistent user should not fail")
					require.Empty(t, orders, "should return empty list for nonexistent user")
				})
			})
		})
	})

	t.Run("UpdateOrder", func(t *testing.T) {
		inTx(t, pg.Pool, func(tx pgx.Tx, storage repository.Storage) {
			user, err := storage.User().CreateUser(t.Context(), "user1", "hashedpassword")
			require.NoError(t, err)

			order, err := storage.Order().CreateOrder(t.Context(), "456", user.ID)
			require.NoError(t, err)

			t.Run("update status and accrual", func(t *testing.T) {
				inTx(t, tx, func(_ pgx.Tx, storage repository.Storage) {
					status := models.OrderStatusProcessed
					accrual := decimal.RequireFromString("123.45")

					got, err := storage.Order().UpdateOrder(t.Context(), order.Number, repository.UpdateOrderOpts{Status: &status, Accrual: &accrual})
					require.NoError(t, err, "updating order should not fail")

					require.Equal(t, order.ID, got.ID, "order ID should not change")
					require.Equal(t, status, got.Status, "order status should be updated")
					require.NotNil(t, got.Accrual, "accrual should not be nil after update")
					require.True(t, got.Accrual.Equal(accrual), "order accrual should be updated")
					require.Equal(t, order.UserID, got.UserID)
					require.Equal(t, order.UploadedAt, got.UploadedAt, "should not changed")
					require.NotEqual(t, order.ModifiedAt, got.ModifiedAt, "modified_at should be updated")
				})
			})

			t.Run("do nothing if all nil", func(t *testing.T) {
				inTx(t, tx, func(_ pgx.Tx, storage repository.Storage) {
					got, err := storage.Order().UpdateOrder(t.Context(), order.Number, repository.UpdateOrderOpts{})
					require.NoError(t, err, "updating order should not fail")

					require.Equal(t, order.ID, got.ID, "order ID should not change")
					require.Equal(t, order.Status, got.Status, "order status should be updated")
					require.Nil(t, got.Accrual, "accrual should be nil after update")
					require.Equal(t, order.UserID, got.UserID)
					require.Equal(t, order.UploadedAt, got.UploadedAt, "should not changed")
					require.Equal(t, order.ModifiedAt, got.ModifiedAt, "modified_at must not be changed")
				})
			})
		})

	})
}
