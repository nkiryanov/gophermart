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

	ErrOrderNumberTaken      = errors.New("order number already exists for different user")
	ErrOrderAlreadyExists    = errors.New("order already exists for this user")
	ErrOrderNumberInvalid    = errors.New("order number is invalid")
	ErrOrderNotFound         = errors.New("order not found")
	ErrOrderAlreadyProcessed = errors.New("order already processed")

	ErrBalanceInsufficient = errors.New("insufficient balance")
)
