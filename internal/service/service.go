package service

import (
	"context"
	"net/http"

	"github.com/nkiryanov/gophermart/internal/models"
)

// Auth service
type Auth interface {
	// Register user with username and password
	// Has to return apperrors.ErrUserAlreadyExists if user already exists
	Register(ctx context.Context, username string, password string) (models.TokenPair, error)

	// Login user with username and password
	// Has to return apperrors.ErrUserNotFound if user not found
	Login(ctx context.Context, username string, password string) (models.TokenPair, error)

	// Refresh tokens using refresh token
	// If token expired: has to return apperrors.ErrRefreshTokenExpired
	// If token not found: has to return apperrors.ErrRefreshTokenNotFound
	Refresh(ctx context.Context, refresh string) (models.TokenPair, error)

	// Set auth tokens (access, refresh, csrf if any)
	SetTokens(ctx context.Context, w http.ResponseWriter, tokens models.TokenPair)

	// Symmetric to 'SetTokens': extracts refresh token form request
	GetRefresh(r *http.Request) (string, error)

	// Symmetric to 'SetTokens': extract and validate access token and return authenticated user
	Auth(ctx context.Context, r *http.Request) (models.User, error)
}
