package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/models"
)

type BalanceRepo struct {
	DB DBTX
}

func (r *BalanceRepo) CreateBalance(ctx context.Context, userID uuid.UUID) error {
	const createBalance = `
	INSERT INTO balances (user_id, current, withdrawn)
	VALUES ($1, 0, 0)
	RETURNING id
	`

	_, err := r.DB.Exec(ctx, createBalance, userID)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return fmt.Errorf("user balance already exists: %w", err)
		}

		return fmt.Errorf("db error: %w", err)
	}

	return nil
}

// Get user's balance by userID
// If lock set to true run select query with lock
func (r *BalanceRepo) GetBalance(ctx context.Context, userID uuid.UUID, lock bool) (models.Balance, error) {
	const getBalanceByUserID = `
	SELECT id, user_id, current, withdrawn FROM balances
	WHERE user_id = $1
	`

	const getBalanceByUserIDForUpdate = `
	SELECT id, user_id, current, withdrawn FROM balances
	WHERE user_id = $1
	FOR UPDATE
	`

	var query string

	switch lock {
	case true:
		query = getBalanceByUserIDForUpdate
	default:
		query = getBalanceByUserID
	}

	rows, _ := r.DB.Query(ctx, query, userID)
	return r.collectBalance(rows)
}

// Update user balance
func (r *BalanceRepo) UpdateBalance(ctx context.Context, userID uuid.UUID, delta decimal.Decimal) (models.Balance, error) {
	const withdrawn = `
	UPDATE balances
	SET current = current - $2, withdrawn = withdrawn + $2
	WHERE user_id = $1
	RETURNING id, user_id, current, withdrawn
	`

	const accrual = `
	UPDATE balances
	SET current = current + $2
	WHERE user_id = $1
	RETURNING id, user_id, current, withdrawn
	`

	var query string

	switch delta.IsNegative() {
	case true:
		query = withdrawn
	default:
		query = accrual
	}

	rows, _ := r.DB.Query(ctx, query, userID, delta.Abs())
	return r.collectBalance(rows)
}

func (r *BalanceRepo) collectBalance(rows pgx.Rows) (models.Balance, error) {
	balance, err := pgx.CollectOneRow(rows, func(row pgx.CollectableRow) (models.Balance, error) {
		var b models.Balance
		err := row.Scan(&b.ID, &b.UserID, &b.Current, &b.Withdrawn)
		return b, err
	})

	var pgErr *pgconn.PgError

	switch {
	case err == nil:
		return balance, nil
	case errors.Is(err, pgx.ErrNoRows):
		return balance, apperrors.ErrUserNotFound
	case errors.As(err, &pgErr) && pgErr.Code == pgerrcode.CheckViolation:
		return balance, apperrors.ErrBalanceInsufficient
	default:
		return balance, fmt.Errorf("db error: %w", err)
	}
}
