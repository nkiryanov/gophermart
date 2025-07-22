package postgres

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/repository"
	"github.com/nkiryanov/gophermart/internal/testutil"
)

func TestBalance(t *testing.T) {
	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	inTx := func(t *testing.T, outerTx DBTX, fn func(pgx.Tx, repository.Storage)) {
		testutil.InTx(outerTx, t, func(innerTx pgx.Tx) {
			storage := NewStorage(innerTx)
			fn(innerTx, storage)
		})
	}

	t.Run("CreateBalance", func(t *testing.T) {
		inTx(t, pg.Pool, func(tx pgx.Tx, storage repository.Storage) {
			user, err := storage.User().CreateUser(t.Context(), "testuser", "hashedpassword")
			require.NoError(t, err)

			t.Run("create ok", func(t *testing.T) {
				inTx(t, tx, func(ttx pgx.Tx, storage repository.Storage) {
					err := storage.Balance().CreateBalance(t.Context(), user.ID)

					require.NoError(t, err, "balance has to be created ok")
				})
			})

			t.Run("create duplicate", func(t *testing.T) {
				inTx(t, tx, func(ttx pgx.Tx, storage repository.Storage) {
					err := storage.Balance().CreateBalance(t.Context(), user.ID)
					require.NoError(t, err, "first balance creation should be ok")

					err = storage.Balance().CreateBalance(t.Context(), user.ID)

					require.Error(t, err, "creating balance twice should fail")
					require.Contains(t, err.Error(), "user balance already exists")
				})
			})
		})
	})

	t.Run("GetBalance", func(t *testing.T) {
		inTx(t, pg.Pool, func(tx pgx.Tx, storage repository.Storage) {
			user, err := storage.User().CreateUser(t.Context(), "testuser", "hashedpassword")
			require.NoError(t, err)

			t.Run("get existing balance", func(t *testing.T) {
				inTx(t, tx, func(ttx pgx.Tx, storage repository.Storage) {
					err := storage.Balance().CreateBalance(t.Context(), user.ID)
					require.NoError(t, err)

					balance, err := storage.Balance().GetBalance(t.Context(), user.ID)

					require.NoError(t, err, "getting balance should not fail")
					require.NotZero(t, balance.ID)
					require.Equal(t, user.ID, balance.UserID)
					require.True(t, balance.Current.IsZero(), "current balance should be zero for new balance")
					require.True(t, balance.Withdrawn.IsZero(), "withdrawn balance should be zero for new balance")
				})
			})

			t.Run("get nonexistent balance", func(t *testing.T) {
				inTx(t, tx, func(ttx pgx.Tx, storage repository.Storage) {
					_, err := storage.Balance().GetBalance(t.Context(), uuid.New())

					require.Error(t, err, "getting nonexistent balance should fail")
					require.ErrorIs(t, err, apperrors.ErrUserNotFound, "should return well known error")
				})
			})
		})
	})
}
