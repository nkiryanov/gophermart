package postgres

import (
	"testing"
	"time"

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

	// Create transaction and repository for on the transaction
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
					_, err = repos.OrderRepo.CreateOrder(t.Context(), "123", user.ID, repository.WithOrderStatus(models.OrderProcessed))

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
					_, err = repos.OrderRepo.CreateOrder(t.Context(), "123", yaUser.ID, repository.WithOrderStatus(models.OrderProcessed))

					require.Error(t, err, "crating same order must failed")
					require.ErrorIs(t, err, apperrors.ErrOrderNumberTaken, "should return well known error")
				})
			})

		})
	})
}
