package models

import (
	"time"
)

type Hasher interface {
	Hash(raw string) (string, error)
	Check(raw, hashed string) bool
}

type User struct {
	ID           int64
	CreatedAt    time.Time
	Username     string
	PasswordHash string
}

func (u *User) SetPassword(raw string, h Hasher) error {
	hash, err := h.Hash(raw)
	if err != nil {
		return err
	}
	u.PasswordHash = hash
	return nil
}

func (u *User) CheckPassword(raw string, h Hasher) bool {
	return h.Check(raw, u.PasswordHash)
}
