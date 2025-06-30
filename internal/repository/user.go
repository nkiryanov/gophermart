package repository

import (
	"context"
	"errors"

	"github.com/nkiryanov/gophermart/internal/models"
)

var (
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrUserNotFound      = errors.New("user not found")
)

type UserRepo interface {
	CreateUser(ctx context.Context, username string, passwordHash string) (models.User, error)
	GetUserByID(ctx context.Context, id int64) (models.User, error)
	GetUserByUsername(ctx context.Context, username string) (models.User, error)
}
