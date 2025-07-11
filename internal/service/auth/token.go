package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/repository"
)

type AccessTokenClaims struct {
	jwt.RegisteredClaims
	UserID uuid.UUID `json:"uid"`
}

type TokenManager struct {
	// Secret key to sign access token
	key string

	// JWT MAC (Message Authentication Code) algorithm
	alg string

	// Access and refresh token lifetimes
	accessTTL  time.Duration
	refreshTTL time.Duration

	// Refresh token repo
	refreshRepo repository.RefreshTokenRepo
}

func (m TokenManager) GeneratePair(ctx context.Context, user models.User) (TokenPair, error) {
	createdAt := time.Now()

	// Generate JWT access token decoded as string
	accessToken := jwt.NewWithClaims(
		jwt.GetSigningMethod(m.alg),
		AccessTokenClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        uuid.NewString(),
				IssuedAt:  jwt.NewNumericDate(createdAt),
				ExpiresAt: jwt.NewNumericDate(createdAt.Add(m.accessTTL)),
			},
			UserID: user.ID,
		},
	)
	access, err := accessToken.SignedString([]byte(m.key))
	if err != nil {
		return TokenPair{}, fmt.Errorf("error while signing access token. Err: %w", err)
	}

	// Generate random refresh token 16 bytes length
	b := make([]byte, 16)
	_, err = rand.Read(b)
	if err != nil {
		return TokenPair{}, fmt.Errorf("error while generate refresh token. Err: %w", err)
	}
	refresh := hex.EncodeToString(b)

	_, err = m.refreshRepo.Create(ctx, models.RefreshToken{
		Token:     refresh,
		UserID:    user.ID,
		CreatedAt: createdAt,
		ExpiresAt: createdAt.Add(m.refreshTTL),
	})
	if err != nil {
		return TokenPair{}, fmt.Errorf("error while saving refresh token. Err: %w", err)
	}

	return TokenPair{
		Access:  access,
		Refresh: refresh,
	}, nil
}

// Use token: return if it valid and mark as used
func (m TokenManager) UseToken(ctx context.Context, refresh string) (models.RefreshToken, error) {
	token, err := m.refreshRepo.GetValidToken(ctx, refresh, time.Now())
	if err != nil {
		return token, fmt.Errorf("no valid refresh token. Err: %w", err)
	}

	_, err = m.refreshRepo.MarkUsed(ctx, refresh)
	if err != nil {
		return token, fmt.Errorf("error while marking token used. Err: %w", err)
	}

	return token, nil
}
