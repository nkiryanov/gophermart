package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/nkiryanov/gophermart/internal/apperrors"
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
	alg jwt.SigningMethod

	// Access and refresh token lifetimes
	accessTTL  time.Duration
	refreshTTL time.Duration

	// Refresh token repo
	refreshRepo repository.RefreshTokenRepo
}

func (m TokenManager) GeneratePair(ctx context.Context, user models.User) (models.TokenPair, error) {
	var pair models.TokenPair
	now := time.Now().Truncate(time.Second)
	accessExpiresAt := now.Add(m.accessTTL)
	refreshExpiresAt := now.Add(m.refreshTTL)

	// Generate JWT access token decoded as string
	accessToken := jwt.NewWithClaims(
		m.alg,
		AccessTokenClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        uuid.NewString(),
				IssuedAt:  jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(accessExpiresAt),
			},
			UserID: user.ID,
		},
	)
	access, err := accessToken.SignedString([]byte(m.key))
	if err != nil {
		return pair, fmt.Errorf("error while signing access token. Err: %w", err)
	}

	// Generate random refresh token 16 bytes length
	b := make([]byte, 16)
	_, err = rand.Read(b)
	if err != nil {
		return pair, fmt.Errorf("error while generate refresh token. Err: %w", err)
	}
	refresh := hex.EncodeToString(b)

	_, err = m.refreshRepo.Save(ctx, models.RefreshToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		Token:     refresh,
		CreatedAt: now,
		ExpiresAt: refreshExpiresAt,
		UsedAt:    nil,
	})
	if err != nil {
		return pair, fmt.Errorf("error while saving refresh token. Err: %w", err)
	}

	return models.TokenPair{
		Access:  models.IssuedToken{Value: access, ExpiresAt: accessExpiresAt},
		Refresh: models.IssuedToken{Value: refresh, ExpiresAt: refreshExpiresAt},
	}, nil
}

// Use token: return if it valid and mark as used
func (m TokenManager) UseRefreshToken(ctx context.Context, refresh string) (models.RefreshToken, error) {
	token, err := m.refreshRepo.GetAndMarkUsed(ctx, refresh)
	if err != nil {
		return token, fmt.Errorf("error while marking token used. Err: %w", err)
	}

	if token.ExpiresAt.Before(time.Now()) {
		return token, fmt.Errorf("error while marking token used. Err: %w", apperrors.ErrRefreshTokenExpired)
	}

	return token, nil
}

// Parse and validate access token
func (m TokenManager) ParseAccess(ctx context.Context, access string) (userID uuid.UUID, err error) {
	claims := &AccessTokenClaims{}
	token, err := jwt.ParseWithClaims(
		access,
		claims,
		func(t *jwt.Token) (any, error) { return []byte(m.key), nil },
		jwt.WithValidMethods([]string{m.alg.Alg()}),
	)

	switch {
	case token.Valid:
		return claims.UserID, nil
	default:
		return uuid.Nil, fmt.Errorf("error parsing token. Err: %w", err)
	}
}
