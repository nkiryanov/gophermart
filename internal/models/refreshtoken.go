package models

import (
	"github.com/google/uuid"
	"time"
)

type RefreshToken struct {
	ID uuid.UUID
	UserID    uuid.UUID
	Token     string
	CreatedAt time.Time
	ExpiresAt time.Time
	UsedAt    *time.Time  // nil if token not used
}
