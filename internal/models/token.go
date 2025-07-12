package models

import (
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Token     string
	CreatedAt time.Time
	ExpiresAt time.Time
	UsedAt    *time.Time // nil if token not used
}

type IssuedToken struct {
	Value     string
	ExpiresAt time.Time
}

// Token pair issues by TokenManager, AuthService
type TokenPair struct {
	Access  IssuedToken
	Refresh IssuedToken
}
