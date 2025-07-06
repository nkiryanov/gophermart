package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID             uuid.UUID
	CreatedAt      time.Time
	Username       string
	HashedPassword string
}

type UserRepo interface {
	CreateUser(ctx context.Context, username string, hashedPassword string) (User, error)
	GetUserByID(ctx context.Context, userID uuid.UUID) (User, error)
	GetUserByUsername(ctx context.Context, username string) (User, error)
}
