package auth

import (
	"crypto/sha256"

	"golang.org/x/crypto/bcrypt"
)

// Bcrypt password hasher
// Will be used as default one if user not provide it's own
type BcryptHasher struct{}

func (h BcryptHasher) Hash(password string) (string, error) {
	sum := sha256.Sum256([]byte(password))
	hash, err := bcrypt.GenerateFromPassword(sum[:], bcrypt.DefaultCost)
	return string(hash), err
}

func (h BcryptHasher) Compare(hashedPassword string, password string) error {
	sum := sha256.Sum256([]byte(password))
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), sum[:])
}
