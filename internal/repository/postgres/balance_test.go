package postgres

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
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

					balance, err := storage.Balance().GetBalance(t.Context(), user.ID, false)

					require.NoError(t, err, "getting balance should not fail")
					require.NotZero(t, balance.ID)
					require.Equal(t, user.ID, balance.UserID)
					require.True(t, balance.Current.IsZero(), "current balance should be zero for new balance")
					require.True(t, balance.Withdrawn.IsZero(), "withdrawn balance should be zero for new balance")
				})
			})

			t.Run("get nonexistent balance", func(t *testing.T) {
				inTx(t, tx, func(ttx pgx.Tx, storage repository.Storage) {
					_, err := storage.Balance().GetBalance(t.Context(), uuid.New(), false)

					require.Error(t, err, "getting nonexistent balance should fail")
					require.ErrorIs(t, err, apperrors.ErrUserNotFound, "should return well known error")
				})
			})
		})
	})

	t.Run("UpdateBalance", func(t *testing.T) {
		inTx(t, pg.Pool, func(tx pgx.Tx, storage repository.Storage) {
			user, err := storage.User().CreateUser(t.Context(), "test-user", "hash")
			require.NoError(t, err)
			err = storage.Balance().CreateBalance(t.Context(), user.ID)
			require.NoError(t, err)

			t.Run("accrual", func(t *testing.T) {
				inTx(t, tx, func(ttx pgx.Tx, storage repository.Storage) {
					balance, err := storage.Balance().UpdateBalance(t.Context(), user.ID, decimal.NewFromInt(100))
					require.NoError(t, err, "updating balance should not fail")

					require.Equal(t, user.ID, balance.UserID, "user ID should match")
					require.Equal(t, decimal.NewFromInt(100), balance.Current)
					require.True(t, balance.Withdrawn.IsZero(), "withdrawn balance should be zero after accrual")

					storedBalance, err := storage.Balance().GetBalance(t.Context(), user.ID, false)
					require.NoError(t, err, "getting balance after accrual should not fail")
					require.Equal(t, balance.Current, storedBalance.Current, "current balance should match after accrual")
					require.Equal(t, balance.Withdrawn, storedBalance.Withdrawn, "withdrawn balance should match after accrual")
				})
			})

			t.Run("withdrawn", func(t *testing.T) {
				inTx(t, tx, func(ttx pgx.Tx, storage repository.Storage) {
					_, err := storage.Balance().UpdateBalance(t.Context(), user.ID, decimal.NewFromInt(100))
					require.NoError(t, err, "updating balance should not fail")

					balance, err := storage.Balance().UpdateBalance(t.Context(), user.ID, decimal.NewFromInt(-50))
					require.NoError(t, err, "withdrawing balance should not fail")

					require.Equal(t, user.ID, balance.UserID, "user ID should match")
					require.Equal(t, decimal.NewFromInt(50), balance.Current, "current balance should reflect withdrawal")
					require.Equal(t, decimal.NewFromInt(50), balance.Withdrawn, "withdrawn balance should reflect withdrawal")

					storedBalance, err := storage.Balance().GetBalance(t.Context(), user.ID, false)
					require.NoError(t, err, "getting balance after withdrawal should not fail")
					require.Equal(t, balance.Current, storedBalance.Current, "current balance should match after withdrawal")
					require.Equal(t, balance.Withdrawn, storedBalance.Withdrawn, "withdrawn balance should match after withdrawal")
				})
			})

			t.Run("withdrawn insufficient funds", func(t *testing.T) {
				_, err := storage.Balance().UpdateBalance(t.Context(), user.ID, decimal.NewFromInt(100))
				require.NoError(t, err, "updating balance should not fail")

				_, err = storage.Balance().UpdateBalance(t.Context(), user.ID, decimal.NewFromInt(-201))
				require.Error(t, err, "withdrawing more than available balance should fail")
				require.ErrorIs(t, err, apperrors.ErrBalanceInsufficient, "should return insufficient funds error")

				storedBalance, err := storage.Balance().GetBalance(t.Context(), user.ID, false)
				require.NoError(t, err, "getting balance after failed withdrawal should not fail")
				require.Equal(t, decimal.NewFromInt(100), storedBalance.Current, "current balance must not change after failed withdrawal")
				require.True(t, storedBalance.Withdrawn.IsZero(), "withdrawn balance must not change after failed withdrawal")
			})

		})
	})
}
