package postgres

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/models"
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

			accrualTransaction := models.Transaction{UserID: user.ID, Type: models.TransactionTypeAccrual, Amount: decimal.NewFromInt(100)}

			t.Run("accrual", func(t *testing.T) {
				inTx(t, tx, func(ttx pgx.Tx, storage repository.Storage) {
					balance, err := storage.Balance().UpdateBalance(t.Context(), accrualTransaction)
					require.NoError(t, err, "updating balance should not fail")

					require.Equal(t, user.ID, balance.UserID, "user ID should match")
					require.True(t, balance.Current.Equal(decimal.NewFromInt(100)), "current balance should be 100 after accrual")
					require.True(t, balance.Withdrawn.IsZero(), "withdrawn balance should be zero after accrual")

					storedBalance, err := storage.Balance().GetBalance(t.Context(), user.ID, false)
					require.NoError(t, err, "getting balance after accrual should not fail")
					require.True(t, balance.Current.Equal(storedBalance.Current), "current balance should match after accrual")
					require.True(t, balance.Withdrawn.IsZero(), "withdrawn balance should match after")
				})
			})

			t.Run("withdrawal", func(t *testing.T) {
				inTx(t, tx, func(ttx pgx.Tx, storage repository.Storage) {
					_, err := storage.Balance().UpdateBalance(t.Context(), accrualTransaction)
					require.NoError(t, err, "updating balance should not fail")

					balance, err := storage.Balance().UpdateBalance(t.Context(), models.Transaction{
						UserID: user.ID,
						Type:   models.TransactionTypeWithdrawal,
						Amount: decimal.NewFromInt(70),
					})
					require.NoError(t, err, "withdrawing balance should not fail")

					require.Equal(t, user.ID, balance.UserID, "user ID should match")
					require.True(t, balance.Current.Equal(decimal.NewFromInt(30)), "current balance should be 50 after withdrawal")
					require.True(t, balance.Withdrawn.Equal(decimal.NewFromInt(70)), "withdrawn balance should reflect withdrawal")

					storedBalance, err := storage.Balance().GetBalance(t.Context(), user.ID, false)
					require.NoError(t, err, "getting balance after withdrawal should not fail")
					require.True(t, storedBalance.Current.Equal(decimal.NewFromInt(30)), "current balance should match after withdrawal")
					require.True(t, storedBalance.Withdrawn.Equal(decimal.NewFromInt(70)), "withdrawn balance should match after withdrawal")
				})
			})

			t.Run("withdrawn insufficient funds", func(t *testing.T) {
				_, err := storage.Balance().UpdateBalance(t.Context(), accrualTransaction)
				require.NoError(t, err, "updating balance should not fail")

				_, err = storage.Balance().UpdateBalance(t.Context(), models.Transaction{
					UserID: user.ID,
					Type:   models.TransactionTypeWithdrawal,
					Amount: decimal.NewFromInt(200),
				})
				require.Error(t, err, "withdrawing more than available balance should fail")
				require.ErrorIs(t, err, apperrors.ErrBalanceInsufficient, "should return insufficient funds error")
			})

		})
	})
}

func TestTransactions(t *testing.T) {
	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	inTx := func(t *testing.T, outerTx DBTX, fn func(pgx.Tx, repository.Storage)) {
		testutil.InTx(outerTx, t, func(innerTx pgx.Tx) {
			storage := NewStorage(innerTx)
			fn(innerTx, storage)
		})
	}

	t.Run("CreateTransaction", func(t *testing.T) {
		inTx(t, pg.Pool, func(tx pgx.Tx, storage repository.Storage) {
			user, err := storage.User().CreateUser(t.Context(), "testuser", "hashedpassword")
			require.NoError(t, err)

			t.Run("create transaction not existed user", func(t *testing.T) {
				inTx(t, tx, func(ttx pgx.Tx, storage repository.Storage) {
					transaction := models.Transaction{
						ID:          uuid.New(),
						ProcessedAt: time.Now(),
						UserID:      uuid.New(), // Non-existent user
						OrderNumber: "12345",
						Type:        models.TransactionTypeAccrual,
						Amount:      decimal.NewFromInt(100),
					}

					_, err := storage.Balance().CreateTransaction(t.Context(), transaction)
					require.Error(t, err, "creating transaction for non-existent user should fail")

					require.ErrorIs(t, err, apperrors.ErrUserNotFound, "should return well known error")
				})
			})

			t.Run("create accrual transaction", func(t *testing.T) {
				inTx(t, tx, func(ttx pgx.Tx, storage repository.Storage) {
					transaction := models.Transaction{
						ID:          uuid.New(),
						ProcessedAt: time.Now(),
						UserID:      user.ID,
						OrderNumber: "12345",
						Type:        models.TransactionTypeAccrual,
						Amount:      decimal.NewFromInt(100),
					}

					got, err := storage.Balance().CreateTransaction(t.Context(), transaction)

					require.NoError(t, err, "creating accrual transaction should not fail")
					require.Equal(t, transaction.ID, got.ID)
					require.Equal(t, transaction.UserID, got.UserID)
					require.Equal(t, transaction.OrderNumber, got.OrderNumber)
					require.Equal(t, transaction.Type, got.Type)
					require.True(t, got.Amount.Equal(transaction.Amount), "amount should match")
				})
			})

			t.Run("create withdrwal transaction", func(t *testing.T) {
				inTx(t, tx, func(ttx pgx.Tx, storage repository.Storage) {
					transaction := models.Transaction{
						ID:          uuid.New(),
						ProcessedAt: time.Now(),
						UserID:      user.ID,
						OrderNumber: "67890",
						Type:        models.TransactionTypeWithdrawal,
						Amount:      decimal.NewFromInt(50),
					}

					createdTransaction, err := storage.Balance().CreateTransaction(t.Context(), transaction)

					require.NoError(t, err, "creating withdrawal transaction should not fail")
					require.Equal(t, transaction.ID, createdTransaction.ID)
					require.Equal(t, transaction.UserID, createdTransaction.UserID)
					require.Equal(t, transaction.OrderNumber, createdTransaction.OrderNumber)
					require.Equal(t, transaction.Type, createdTransaction.Type)
					require.True(t, createdTransaction.Amount.Equal(transaction.Amount), "amount should match")
				})
			})
		})
	})

	t.Run("ListTransactions", func(t *testing.T) {
		inTx(t, pg.Pool, func(tx pgx.Tx, storage repository.Storage) {
			user, err := storage.User().CreateUser(t.Context(), "test-user", "hashedpassword")
			require.NoError(t, err)

			// Create test transactions
			accrualTx := models.Transaction{
				ID:          uuid.New(),
				ProcessedAt: time.Now().Add(-2 * time.Hour),
				UserID:      user.ID,
				OrderNumber: "12345",
				Type:        models.TransactionTypeAccrual,
				Amount:      decimal.NewFromInt(100),
			}

			withdrawnTx := models.Transaction{
				ID:          uuid.New(),
				ProcessedAt: time.Now().Add(-1 * time.Hour),
				UserID:      user.ID,
				OrderNumber: "67890",
				Type:        models.TransactionTypeWithdrawal,
				Amount:      decimal.NewFromInt(50),
			}

			_, err = storage.Balance().CreateTransaction(t.Context(), accrualTx)
			require.NoError(t, err)
			_, err = storage.Balance().CreateTransaction(t.Context(), withdrawnTx)
			require.NoError(t, err)

			t.Run("list all transactions", func(t *testing.T) {
				inTx(t, tx, func(ttx pgx.Tx, storage repository.Storage) {
					transactions, err := storage.Balance().ListTransactions(t.Context(), user.ID, nil)

					require.NoError(t, err, "listing all transactions should not fail")
					require.Len(t, transactions, 2, "should return all transactions")

					// Check ordering (should be DESC by processed_at)
					require.Equal(t, withdrawnTx.ID, transactions[0].ID, "first transaction should be the most recent")
					require.Equal(t, accrualTx.ID, transactions[1].ID, "second transaction should be the older one")
				})
			})

			t.Run("list withdrawals transactions only", func(t *testing.T) {
				inTx(t, tx, func(ttx pgx.Tx, storage repository.Storage) {
					transactions, err := storage.Balance().ListTransactions(t.Context(), user.ID, []string{models.TransactionTypeWithdrawal})

					require.NoError(t, err, "listing withdrawn transactions should not fail")
					require.Len(t, transactions, 1, "should return only withdrawn transactions")
					require.Equal(t, withdrawnTx.ID, transactions[0].ID)
					require.Equal(t, withdrawnTx.Type, transactions[0].Type, "transaction type should be withdrawal")
				})
			})

			t.Run("list transactions for nonexistent user", func(t *testing.T) {
				inTx(t, tx, func(ttx pgx.Tx, storage repository.Storage) {
					transactions, err := storage.Balance().ListTransactions(t.Context(), uuid.New(), nil)

					require.NoError(t, err, "listing transactions for nonexistent user should not fail")
					require.Empty(t, transactions, "should return empty list for nonexistent user")
				})
			})
		})
	})
}
