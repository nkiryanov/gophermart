package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nkiryanov/gophermart/internal/repository"
)

// Interface to create or compare user password hashes
type PasswordHasher interface {
	// Generate Hash from password
	Hash(password string) (string, error)

	// Compare known hashedPassword and user provided password
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

// Token pair of tokens that should be returned to the user on authentication
type TokenPair struct {
	Access  string
	Refresh string
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

func (s *AuthService) Register(ctx context.Context, username string, password string) (TokenPair, error) {
	var pair TokenPair

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

func (s *AuthService) Login(ctx context.Context, username string, password string) (TokenPair, error) {
	return TokenPair{}, nil
}
