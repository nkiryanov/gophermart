package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/repository"
)

// Interface to create or compare user password hashes
type PasswordHasher interface {
	// Generate Hash from password
	Hash(password string) (string, error)

	// Compare known hashedPassword and user provided password
	// Must be protected against timing attacks
	Compare(hashedPassword string, password string) error
}

type AuthServiceConfig struct {
	// Secret key to sign user access token payload
	SecretKey string

	// Hasher to user during user registration or login process
	Hasher PasswordHasher

	// Access and refresh token lifetimes
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// Auth service
type AuthService struct {
	// Manager to issue token pairs (access and refresh)
	token TokenManager

	// hasher to hash or compare user passwords
	hasher PasswordHasher

	// Repository to access long term data
	userRepo repository.UserRepo
}

func NewAuthService(cfg AuthServiceConfig, userRepo repository.UserRepo, refreshRepo repository.RefreshTokenRepo) (*AuthService, error) {
	// Set default bcrypt hasher if not user provided by user
	hasher := cfg.Hasher
	if hasher == nil {
		hasher = BcryptHasher{}
	}

	if refreshRepo == nil || userRepo == nil {
		return nil, errors.New("repos must not be nil")
	}

	tokenManager := TokenManager{
		key:         cfg.SecretKey,
		alg:         "HS256",
		accessTTL:   cfg.AccessTokenTTL,
		refreshTTL:  cfg.RefreshTokenTTL,
		refreshRepo: refreshRepo,
	}

	return &AuthService{
		token:    tokenManager,
		hasher:   hasher,
		userRepo: userRepo,
	}, nil
}

func (s *AuthService) Register(ctx context.Context, username string, password string) (models.TokenPair, error) {
	var pair models.TokenPair

	hash, err := s.hasher.Hash(password)
	if err != nil {
		return pair, fmt.Errorf("can't use this as password, error=%w", err)
	}

	user, err := s.userRepo.CreateUser(ctx, username, hash)
	if err != nil {
		return pair, err
	}

	pair, err = s.token.GeneratePair(ctx, user)
	if err != nil {
		return pair, fmt.Errorf("token could not generated, sorry. %w", err)
	}

	return pair, nil
}

func (s *AuthService) Login(ctx context.Context, username string, password string) (models.TokenPair, error) {
	var pair models.TokenPair

	// Ignore error for now, but prefer to log it
	// It's safe to use user now, because it's always not empty
	user, _ := s.userRepo.GetUserByUsername(ctx, username)

	// Always compare password to prevent timing attacks
	// It will always fail if user not found
	err := s.hasher.Compare(user.HashedPassword, password)
	if err != nil {
		return pair, apperrors.ErrUserNotFound
	}

	pair, err = s.token.GeneratePair(ctx, user)
	if err != nil {
		return pair, fmt.Errorf("token could not be generated, sorry. %w", err)
	}

	return pair, nil
}

func (s *AuthService) Refresh(ctx context.Context, refresh string) (models.TokenPair, error) {
	var pair models.TokenPair

	// Mark token as used
	// Always fail if token is not valid or not found
	token, err := s.token.UseRefreshToken(ctx, refresh)
	if err != nil {
		return pair, fmt.Errorf("token could not be refreshed. Err: %w", err)
	}

	// Check whether user is still exists
	user, err := s.userRepo.GetUserByID(ctx, token.UserID)
	if err != nil {
		return pair, fmt.Errorf("token could not be refreshed. Err: %w", err)
	}

	pair, err = s.token.GeneratePair(ctx, user)
	if err != nil {
		return pair, fmt.Errorf("token could not generated, sorry. %w", err)
	}

	return pair, nil
}
