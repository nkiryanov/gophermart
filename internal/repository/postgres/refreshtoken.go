package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/google/uuid"
)

type RefreshTokenRepo struct {
	DB DBTX
}

const saveToken = `-- name: Save Refresh Token
INSERT INTO refresh_tokens (id, user_id, token, created_at, expires_at, used_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id`

func (r *RefreshTokenRepo) Save(ctx context.Context, token models.RefreshToken) error {
	rows, _ := r.DB.Query(ctx, saveToken, token.ID, token.UserID, token.Token, token.CreatedAt, token.ExpiresAt, token.UsedAt)
	_, err := pgx.CollectOneRow(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return fmt.Errorf("db error: %w", err)
	}
	return nil
}

const getToken = `-- name: GetToken by string itself
SELECT id, user_id, created_at, expires_at, used_at
FROM refresh_tokens
WHERE token = $1
`

// Get token
// It should return result even it expired or used already
func (r *RefreshTokenRepo) Get(ctx context.Context, tokenString string) (models.RefreshToken, error) {
	rows, _ := r.DB.Query(ctx, getToken, tokenString)
	token, err := pgx.CollectOneRow(rows, func(row pgx.CollectableRow) (models.RefreshToken, error) {
		var t = models.RefreshToken{Token: tokenString}
		err := row.Scan(&t.ID, &t.UserID, &t.CreatedAt, &t.ExpiresAt, &t.UsedAt)
		return t, err
	})

	switch {
	case err == nil:
		return token, nil
	case errors.Is(err, pgx.ErrNoRows):
		return token, fmt.Errorf("repo error: %w", apperrors.ErrRefreshTokenNotFound)
	default:
		return token, fmt.Errorf("db error: %w", err)
	}
}

const markTokenUsed = `-- name: Mark token used if it not used
UPDATE refresh_tokens
SET used_at = COALESCE(used_at, $2)
WHERE token = $1
RETURNING used_at
`

// Mark token as used
// Must be idempotent: if token is used already it should return return error
// Should not rewrite already used tokens
func (r *RefreshTokenRepo) MarkUsed(ctx context.Context, tokenString string) (time.Time, error) {
	now := time.Now()
	rows, _ := r.DB.Query(ctx, markTokenUsed, tokenString, now)
	usedAt, err := pgx.CollectOneRow(rows, pgx.RowTo[time.Time])

	switch {
	case err == nil && usedAt.Equal(now):
		return usedAt, nil
	case err == nil:  // usedAt != now == token is used
		return usedAt, fmt.Errorf("repo error: %w", apperrors.ErrRefreshTokenIsUsed)
	case errors.Is(err, pgx.ErrNoRows):
		return usedAt, fmt.Errorf("repo error: %w", apperrors.ErrRefreshTokenNotFound)
	default:
		return usedAt, fmt.Errorf("db error: %w", err)
	}
}
