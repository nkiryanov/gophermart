package repository

import (
	"context"

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
	// Save token in repository
	Save(ctx context.Context, token models.RefreshToken) (models.RefreshToken, error)

	// Return the token if it exists in the database
	Get(ctx context.Context, tokenString string) (models.RefreshToken, error)

	// Mark token as used
	// If the token is already used, must return apperrors.ErrTokenAlreadyUsed and time when token was used
	GetAndMarkUsed(ctx context.Context, tokenString string) (models.RefreshToken, error)

	// It would be good idea to add methods
	// Delete expired tokens
	// Set tokens revoked for user (or something like that)
}
type OrderRepo interface {
	CreateOrder(ctx context.Context, number string, userID uuid.UUID, opts ...models.OrderOption) (models.Order, error)
	ListOrders(ctx context.Context, userID uuid.UUID) ([]models.Order, error)
}

type BalanceRepo interface {
	CreateBalance(ctx context.Context, userID uuid.UUID) error
	GetBalance(ctx context.Context, userID uuid.UUID, lock bool) (models.Balance, error)
	UpdateBalance(ctx context.Context, t models.Transaction) (models.Balance, error)
	CreateTransaction(ctx context.Context, t models.Transaction) (models.Transaction, error)
	ListTransactions(ctx context.Context, userID uuid.UUID, types []string) ([]models.Transaction, error)
}

type Storage interface {
	User() UserRepo
	Refresh() RefreshTokenRepo
	Order() OrderRepo
	Balance() BalanceRepo

	// InTx starts a transaction, executes the provided function, and commits or rolls back based on the function's error.
	InTx(ctx context.Context, fn func(Storage) error) error
}
