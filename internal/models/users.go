package models

import (
	"time"
)

type User struct {
	ID           int64
	CreatedAt    time.Time
	Username     string
	PasswordHash string
}
