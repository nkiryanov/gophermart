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

const saveToken = `-- name: Save Refresh Token
INSERT INTO refresh_tokens (id, user_id, token, created_at, expires_at, used_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, user_id, token, created_at, expires_at, used_at`

func (r *RefreshTokenRepo) Save(ctx context.Context, token models.RefreshToken) (models.RefreshToken, error) {
	var usedAt pgtype.Timestamptz

	if token.UsedAt != nil {
		usedAt.Valid = true
		usedAt.Time = token.UsedAt.Truncate(time.Microsecond)
	}

	rows, _ := r.DB.Query(ctx,
		saveToken,
		token.ID,
		token.UserID,
		token.Token,
		token.CreatedAt.Truncate(time.Microsecond),
		token.ExpiresAt.Truncate(time.Microsecond),
		usedAt,
	)
	token, err := pgx.CollectOneRow(rows, func(row pgx.CollectableRow) (models.RefreshToken, error) {
		var t models.RefreshToken
		err := row.Scan(&t.ID, &t.UserID, &t.Token, &t.CreatedAt, &t.ExpiresAt, &t.UsedAt)
		return t, err
	})
	if err != nil {
		return token, fmt.Errorf("db error: %w", err)
	}
	return token, nil
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
RETURNING id, user_id, created_at, expires_at, used_at
`

// Mark token as used
// If token is already used it must return 'apperrors.ErrRefreshTokenIsUsed' error
// If token is not found it must return 'apperrors.ErrRefreshTokenNotFound' error
func (r *RefreshTokenRepo) GetAndMarkUsed(ctx context.Context, tokenString string) (models.RefreshToken, error) {
	now := time.Now().Truncate(time.Microsecond)
	rows, _ := r.DB.Query(ctx, markTokenUsed, tokenString, now)

	token, err := pgx.CollectOneRow(rows, func(row pgx.CollectableRow) (models.RefreshToken, error) {
		var t = models.RefreshToken{Token: tokenString}
		err := row.Scan(&t.ID, &t.UserID, &t.CreatedAt, &t.ExpiresAt, &t.UsedAt)
		return t, err
	})

	switch {
	case err == nil && now.Equal(*token.UsedAt): // UsedAt != nil cause token marked used
		return token, nil
	case err == nil: // token.usedAt != now == token is used
		return token, fmt.Errorf("repo error: %w", apperrors.ErrRefreshTokenIsUsed)
	case errors.Is(err, pgx.ErrNoRows):
		return token, fmt.Errorf("repo error: %w", apperrors.ErrRefreshTokenNotFound)
	default:
		return token, fmt.Errorf("db error: %w", err)
	}
}
