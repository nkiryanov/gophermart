package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/models"
)

type RefreshTokenRepo struct {
	DB DBTX
}

const createToken = `-- name: Store Refresh Token
INSERT INTO refresh_tokens (token, user_id, created_at, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING id`

func (r *RefreshTokenRepo) Create(ctx context.Context, token models.RefreshToken) (tokenID int64, err error) {
	rows, _ := r.DB.Query(ctx, createToken, token.Token, token.UserID, token.CreatedAt, token.ExpiresAt)
	tokenID, err = pgx.CollectOneRow(rows, pgx.RowTo[int64])
	if err != nil {
		return 0, fmt.Errorf("db error: %w", err)
	}
	return tokenID, nil
}

const getToken = `-- name: GetToken by string itself
SELECT user_id, created_at, expires_at, used_at
FROM refresh_tokens
WHERE token = $1
`

// Get token
// It should return result even it expired or used already
func (r *RefreshTokenRepo) GetToken(ctx context.Context, tokenString string) (models.RefreshToken, error) {
	rows, _ := r.DB.Query(ctx, getToken, tokenString)
	token, err := pgx.CollectOneRow(rows, func(row pgx.CollectableRow) (models.RefreshToken, error) {
		var t = models.RefreshToken{Token: tokenString}
		var usedAt pgtype.Timestamptz
		err := row.Scan(&t.UserID, &t.CreatedAt, &t.ExpiresAt, &usedAt)
		if err == nil && usedAt.Valid {
			t.UsedAt = usedAt.Time
		}
		return t, err
	})

	switch {
	case err == nil:
		return token, nil
	case errors.Is(err, pgx.ErrNoRows):
		return token, apperrors.ErrRefreshTokenNotFound
	default:
		return token, fmt.Errorf("db error: %w", err)
	}
}

const getNotExpiredToken = `-- name: Get Token by token string itself
SELECT user_id, created_at, expires_at, used_at
FROM refresh_tokens
WHERE token = $1 AND expires_at > $2`

// Get valid token by token string and expired time
// The token obviously valid if it exists, not expired and not used
func (r *RefreshTokenRepo) GetValidToken(ctx context.Context, tokenString string, expiredAfter time.Time) (models.RefreshToken, error) {
	rows, _ := r.DB.Query(ctx, getNotExpiredToken, tokenString, expiredAfter)
	token, err := pgx.CollectOneRow(rows, func(row pgx.CollectableRow) (models.RefreshToken, error) {
		var t = models.RefreshToken{Token: tokenString}
		var usedAt pgtype.Timestamptz
		err := row.Scan(&t.UserID, &t.CreatedAt, &t.ExpiresAt, &usedAt)
		if err == nil && usedAt.Valid {
			t.UsedAt = usedAt.Time
		}
		return t, err
	})

	switch {
	case err == nil:
		if token.UsedAt.IsZero() {
			return token, nil
		}
		return token, apperrors.ErrRefreshTokenIsUsed
	case errors.Is(err, pgx.ErrNoRows):
		return token, apperrors.ErrRefreshTokenNotFound
	default:
		return token, fmt.Errorf("db error: %w", err)
	}
}

const markTokenUsed = `-- name: Mark token used
UPDATE refresh_tokens
SET used_at = COALESCE(used_at, NOW())
WHERE token = $1
RETURNING used_at
`

// Mark token as used
// Should not rewrite already used tokens
func (r *RefreshTokenRepo) MarkUsed(ctx context.Context, tokenString string) (time.Time, error) {
	rows, _ := r.DB.Query(ctx, markTokenUsed, tokenString)
	usedAt, err := pgx.CollectOneRow(rows, pgx.RowTo[time.Time])

	switch {
	case err == nil:
		return usedAt, nil
	case errors.Is(err, pgx.ErrNoRows):
		return usedAt, apperrors.ErrRefreshTokenNotFound
	default:
		return usedAt, fmt.Errorf("db error: %w", err)
	}
}
