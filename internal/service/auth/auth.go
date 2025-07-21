package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/repository"
)

const (
	defaultAccessHeaderName  = "Authorization"
	defaultAccessAuthScheme  = "Bearer"
	defaultRefreshCookieName = "refreshtoken"
)

var (
	DefaultHasher = BcryptHasher{}
)

// Interface to create or compare user password hashes
type PasswordHasher interface {
	// Generate Hash from password
	Hash(password string) (string, error)

	// Compare known hashedPassword and user provided password
	// Must be protected against timing attacks
	Compare(hashedPassword string, password string) error
}

type TokenManager interface {
	// GeneratePair generates access and refresh tokens for user
	GeneratePair(ctx context.Context, user models.User) (models.TokenPair, error)

	// UseRefresh marks refresh token as used and returns it
	UseRefresh(ctx context.Context, refresh string) (models.RefreshToken, error)

	// ParseAccess parses access token and returns user ID
	ParseAccess(ctx context.Context, access string) (userID uuid.UUID, err error)
}

// AuthService config with sensible defaults
// All fields are optional: if not set, default values will be used
type Config struct {
	AccessHeaderName  string
	AccessAuthScheme  string
	RefreshCookieName string

	Hasher PasswordHasher
}

// Auth service
type AuthService struct {
	accessHeaderName  string
	accessAuthScheme  string
	refreshCookieName string

	// Manager to issue token pairs (access and refresh)
	tokenManager TokenManager

	// hasher to hash or compare user passwords
	hasher PasswordHasher

	// Repository to access long term data
	userRepo repository.UserRepo
}

func NewService(cfg Config, tokenManager TokenManager, userRepo repository.UserRepo) (*AuthService, error) {
	setDefaultString := func(field *string, def string) {
		if *field == "" {
			*field = def
		}
	}
	setDefaultString(&cfg.AccessHeaderName, defaultAccessHeaderName)
	setDefaultString(&cfg.AccessAuthScheme, defaultAccessAuthScheme)
	setDefaultString(&cfg.RefreshCookieName, defaultRefreshCookieName)

	// Set default bcrypt hasher if not user provided by user
	if cfg.Hasher == nil {
		cfg.Hasher = DefaultHasher
	}

	return &AuthService{
		accessHeaderName:  cfg.AccessHeaderName,
		accessAuthScheme:  cfg.AccessAuthScheme,
		refreshCookieName: cfg.RefreshCookieName,
		tokenManager:      tokenManager,
		hasher:            cfg.Hasher,
		userRepo:          userRepo,
	}, nil
}

func (s *AuthService) Register(ctx context.Context, username string, password string) (models.TokenPair, error) {
	var pair models.TokenPair

	hash, err := s.hasher.Hash(password)
	if err != nil {
		return pair, fmt.Errorf("can't use this as password, Err: %w", err)
	}

	user, err := s.userRepo.CreateUser(ctx, username, hash)
	if err != nil {
		return pair, fmt.Errorf("can't register user. Err: %w", err)
	}

	pair, err = s.tokenManager.GeneratePair(ctx, user)
	if err != nil {
		return pair, fmt.Errorf("token could not generated, sorry. Err: %w", err)
	}

	return pair, nil
}

func (s *AuthService) Login(ctx context.Context, username string, password string) (models.TokenPair, error) {
	var pair models.TokenPair

	// Ignore error for now, but prefer to log it
	// It's safe to use user now, because it's always not empty
	user, _ := s.userRepo.GetUserByUsername(ctx, username)

	// Always compare password to prevent timing attacks
	// It will always fail if user not found
	err := s.hasher.Compare(user.HashedPassword, password)
	if err != nil {
		return pair, apperrors.ErrUserNotFound
	}

	pair, err = s.tokenManager.GeneratePair(ctx, user)
	if err != nil {
		return pair, fmt.Errorf("token could not be generated, sorry. Err: %w", err)
	}

	return pair, nil
}

// Refresh token pair with valid refresh token
func (s *AuthService) RefreshPair(ctx context.Context, refresh string) (models.TokenPair, error) {
	var pair models.TokenPair

	// Mark token as used
	// Always fail if token is not valid or not found
	token, err := s.tokenManager.UseRefresh(ctx, refresh)
	if err != nil {
		return pair, fmt.Errorf("token could not be refreshed. Err: %w", err)
	}

	// Check whether user is still exists
	user, err := s.userRepo.GetUserByID(ctx, token.UserID)
	if err != nil {
		return pair, fmt.Errorf("token could not be refreshed. Err: %w", err)
	}

	pair, err = s.tokenManager.GeneratePair(ctx, user)
	if err != nil {
		return pair, fmt.Errorf("token could not generated, sorry. Err: %w", err)
	}

	return pair, nil
}

// Set valid token pair to response
// It actually sets access token to header and refresh token to cookie
func (s *AuthService) SetTokenPairToResponse(w http.ResponseWriter, pair models.TokenPair) {
	w.Header().Set(s.accessHeaderName, fmt.Sprintf("%s %s", s.accessAuthScheme, pair.Access.Value))
	http.SetCookie(w, &http.Cookie{
		Name:     s.refreshCookieName,
		Value:    pair.Refresh.Value,
		Path:     "/",
		MaxAge:   int(time.Until(pair.Refresh.ExpiresAt).Seconds()),
		Expires:  pair.Refresh.ExpiresAt,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
	})
}

// Set valid token pair to request
// It actually sets access token to header and refresh token to cookie
func (s *AuthService) SetTokenPairToRequest(r *http.Request, pair models.TokenPair) {
	r.Header.Set(s.accessHeaderName, fmt.Sprintf("%s %s", s.accessAuthScheme, pair.Access.Value))
	r.AddCookie(&http.Cookie{
		Name:     s.refreshCookieName,
		Value:    pair.Refresh.Value,
		Path:     "/",
		MaxAge:   int(time.Until(pair.Refresh.ExpiresAt).Seconds()),
		Expires:  pair.Refresh.ExpiresAt,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
	})
}

// Get refresh token from request
func (s *AuthService) GetRefreshString(r *http.Request) (string, error) {
	cookie, err := r.Cookie(s.refreshCookieName)
	if err != nil {
		return "", fmt.Errorf("can't read refresh token from cookie: %w", err)
	}

	return cookie.Value, nil
}

// Authenticate user using access token from request
func (s *AuthService) Auth(ctx context.Context, r *http.Request) (models.User, error) {
	var u models.User
	var scheme = fmt.Sprintf("%s ", s.accessAuthScheme)

	auth := r.Header.Get(s.accessHeaderName)
	if auth == "" {
		return u, errors.New("auth header doesn't set")
	}
	if !strings.HasPrefix(auth, scheme) {
		return u, errors.New("invalid auth header scheme")
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, scheme))
	if token == "" {
		return u, errors.New("empty auth token")
	}

	userID, err := s.tokenManager.ParseAccess(ctx, token)
	if err != nil {
		return u, fmt.Errorf("token is not valid. Err: %w", err)
	}

	u, err = s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return u, fmt.Errorf("user not found. Err: %w", err)
	}

	return u, err
}
