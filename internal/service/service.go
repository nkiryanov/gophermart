package service

import (
	"context"

)

type Token struct {
	Type string
	ID: 


type TokenPair struct {
	Access  string
	Refresh string
}

type AuthService interface {
	// Register new user and get login TokenPair
	Register(ctx context.Context, username string, password string) (TokenPair, error)

	// Login with existed user and get fresh TokenPair
	Login(ctx context.Context, username string, password string) (TokenPair, error)

	// Refresh user tokens with valid refresh token
	Refresh(ctx context.Context, refresh string) (TokenPair, error)
}
