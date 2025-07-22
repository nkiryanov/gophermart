package postgres

import (
	"context"
	"errors"
	"fmt"

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

func (r *BalanceRepo) GetBalance(ctx context.Context, userID uuid.UUID) (models.Balance, error) {
	const getBalanceByUserID = `
	SELECT id, user_id, current, withdrawn FROM balances
	WHERE user_id = $1
	`

	rows, _ := r.DB.Query(ctx, getBalanceByUserID, userID)
	balance, err := pgx.CollectOneRow(rows, func(row pgx.CollectableRow) (models.Balance, error) {
		var b models.Balance
		err := row.Scan(&b.ID, &b.UserID, &b.Current, &b.Withdrawn)
		return b, err
	})

	switch {
	case err == nil:
		return balance, nil
	case errors.Is(err, pgx.ErrNoRows):
		return balance, apperrors.ErrUserNotFound
	default:
		return balance, fmt.Errorf("db error: %w", err)
	}
}
