package service

import (
	"context"
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/nkiryanov/gophermart/internal/models"
)

type Hasher interface {
	// Generate Hash from password
	Hash(password string) (string, error)
	// Compare known hashedPassword and user provided password
	Compare(hashedPassword string, password string) error
}

type bcryptHasher struct{}

func (h bcryptHasher) Hash(password string) (string, error) {
	sum := sha256.Sum256([]byte(password))
	hash, err := bcrypt.GenerateFromPassword(sum[:], bcrypt.DefaultCost)
	return string(hash), err
}

func (h bcryptHasher) Compare(hashedPassword string, password string) error {
	sum := sha256.Sum256([]byte(password))
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), sum[:])
}

type UserRepo interface {
	CreateUser(ctx context.Context, username string, hashedPassword string) (models.User, error)
	GetUserByUsername(ctx context.Context, username string) (models.User, error)
}

type TokenPair struct{}

type TokenManager interface {
	Generate(models.User) (TokenPair, error)
}

type AuthService struct {
	hasher       Hasher
	tokenManager TokenManager
	userRepo     UserRepo
}

func NewAuthService(hasher Hasher, tokenManager TokenManager, userRepo UserRepo) *AuthService {
	if hasher == nil {
		hasher = bcryptHasher{}
	}

	return &AuthService{
		hasher:       hasher,
		tokenManager: tokenManager,
		userRepo:     userRepo,
	}
}

func (s *AuthService) Register(ctx context.Context, username string, password string) (TokenPair, error) {
	var t TokenPair

	hash, err := s.hasher.Hash(password)
	if err != nil {
		return t, fmt.Errorf("can't use this as password, error=%w", err)
	}

	user, err := s.userRepo.CreateUser(ctx, username, hash)
	if err != nil {
		return t, err
	}

	t, err = s.tokenManager.Generate(user)
	if err != nil {
		return t, fmt.Errorf("token could not generated, sorry. %w", err)
	}

	return t, nil
}
