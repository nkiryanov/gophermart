package models

import (
	"github.com/google/uuid"
	"time"
)

type RefreshToken struct {
	Token     string
	UserID    uuid.UUID
	CreatedAt time.Time
	ExpiresAt time.Time
	UsedAt    time.Time // zero value means the token is not used
}
