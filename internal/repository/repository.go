package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nkiryanov/gophermart/internal/models"
)

// User repository interface
type UserRepo interface {
	// Create user
	// If user with username exists already has to return error apperrors.ErrUserAlreadyExists
	CreateUser(ctx context.Context, username string, hashedPassword string) (models.User, error)

	// Get user by it's id or username
	// If user not found must return apperrors.ErrUserNotExists
	GetUserByID(ctx context.Context, userID uuid.UUID) (models.User, error)
	GetUserByUsername(ctx context.Context, username string) (models.User, error)
}

// RefreshToken repository interface
type RefreshTokenRepo interface {
	// Create token in repository
	Create(ctx context.Context, token models.RefreshToken) (tokenID int64, err error)

	// Mark token as used
	// If the token is already used, must not overwrite the existing 'usedAt'
	MarkUsed(ctx context.Context, tokenString string) (usedAt time.Time, err error)

	// Return the token if it exists in the database
	GetToken(ctx context.Context, tokenString string) (models.RefreshToken, error)

	// Return a valid token only
	// If the token is expired, must return error apperrors.ErrRefreshTokenNotFound
	// If the token is used, must return error apperrors.ErrRefreshTokenIsUsed
	GetValidToken(ctx context.Context, tokenString string, expiredAfter time.Time) (models.RefreshToken, error)

	// It would be good idea to add methods
	// Delete expired tokens
	// Set tokens revoked for user (or something like that)
}
