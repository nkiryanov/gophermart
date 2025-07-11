package apperrors

import (
	"errors"
)

var (
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrUserNotFound      = errors.New("user not found")

	ErrRefreshTokenNotFound = errors.New("refresh token not found")
	ErrRefreshTokenIsUsed   = errors.New("refresh token is used")
	ErrRefreshTokenExpired  = errors.New("refresh token is expired")
)
