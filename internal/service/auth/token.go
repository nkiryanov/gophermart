package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	var pair TokenPair
	createdAt := time.Now()

	accessToken := jwt.NewWithClaims(
		jwt.GetSigningMethod(m.alg),
		AccessTokenClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				IssuedAt:  jwt.NewNumericDate(createdAt),
				ExpiresAt: jwt.NewNumericDate(createdAt.Add(m.accessTTL)),
			},
			UserID: user.ID,
		},
	)

	access, err := accessToken.SignedString([]byte(m.key))
	if err != nil {
		return pair, err
	}

	refresh, err := m.GenerateRandomString(16)
	if err != nil {
		return pair, err
	}

	_, err = m.refreshRepo.Create(ctx, models.RefreshToken{
		Token:     refresh,
		UserID:    user.ID,
		CreatedAt: createdAt,
		ExpiresAt: createdAt.Add(m.refreshTTL),
	})
	if err != nil {
		return pair, err
	}

	pair.Access = access
	pair.Refresh = refresh
	return pair, nil
}

func (m TokenManager) GenerateRandomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
