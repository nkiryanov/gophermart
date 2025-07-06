package domain

import (
	"context"
	"github.com/google/uuid"
	"time"
)

type RefreshToken struct {
	Token     string
	UserID    uuid.UUID
	CreatedAt time.Time
	ExpiresAt time.Time
	UsedAt    time.Time // zero value means the token is not used
}

// RefreshToken repository interface
type RefreshTokenRepo interface {
	// Create token in repository
	Create(ctx context.Context, token RefreshToken) (tokenID int64, err error)

	// Mark token as used
	// If the token is already used, must not overwrite the existing 'usedAt'
	MarkUsed(ctx context.Context, tokenString string) (usedAt time.Time, err error)

	// Return the token if it exists in the database
	GetToken(ctx context.Context, tokenString string) (RefreshToken, error)

	// Return a valid token only
	// If the token is expired, must return error ErrRefreshTokenNotFound
	// If the token is used, must return error ErrRefreshTokenIsUsed
	GetValidToken(ctx context.Context, tokenString string, expiredAfter time.Time) (RefreshToken, error)

	// It would be good idea to add methods
	// Delete expired tokens
	// Set tokens revoked for user (or something like that)
}
