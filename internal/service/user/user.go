package user

import (
	"context"
	"fmt"

	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/repository"
	"github.com/nkiryanov/gophermart/internal/service/auth"
)

type UserService struct {
	hasher   auth.PasswordHasher
	userRepo repository.UserRepo
}

func NewService(hasher auth.PasswordHasher, userRepo repository.UserRepo) *UserService {
	if hasher == nil {
		hasher = auth.DefaultHasher
	}

	return &UserService{
		hasher:   hasher,
		userRepo: userRepo,
	}
}

func (s *UserService) CreateUser(ctx context.Context, username string, password string) (models.User, error) {
	var user models.User
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return user, fmt.Errorf("can't use this as password, Err: %w", err)
	}

	user, err = s.userRepo.CreateUser(ctx, username, hash)
	if err != nil {
		return user, fmt.Errorf("can't create user. Err: %w", err)
	}

	return user, nil
}
