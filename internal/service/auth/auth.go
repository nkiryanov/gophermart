package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/repository"
)

const (
	defaultAccessHeaderName = "Authorization"
	defaultAccessAuthScheme = "Bearer"
	defaultAccessTokenTTL   = 15 * time.Minute
	defaultSigningMethod    = "HS256"

	defaultRefreshTokenTTL   = 24 * time.Hour
	defaultRefreshCookieName = "refreshtoken"
)

// Interface to create or compare user password hashes
type PasswordHasher interface {
	// Generate Hash from password
	Hash(password string) (string, error)

	// Compare known hashedPassword and user provided password
	// Must be protected against timing attacks
	Compare(hashedPassword string, password string) error
}

type AuthServiceConfig struct {
	// Secret key to sign user access token payload
	SecretKey string

	// Hasher to user during user registration or login process
	Hasher PasswordHasher

	// Access token lifetime, header name, auth scheme (like Bearer)
	AccessTokenTTL   time.Duration
	AccessHeaderName string
	AccessAuthScheme string

	// Refresh token lifetime, cookie name
	RefreshTokenTTL   time.Duration
	RefreshCookieName string
}

// Auth service
type AuthService struct {
	accessHeaderName  string
	accessAuthScheme  string
	refreshCookieName string

	// Manager to issue token pairs (access and refresh)
	token TokenManager

	// hasher to hash or compare user passwords
	hasher PasswordHasher

	// Repository to access long term data
	userRepo repository.UserRepo
}

func NewService(cfg AuthServiceConfig, userRepo repository.UserRepo, refreshRepo repository.RefreshTokenRepo) (*AuthService, error) {
	if refreshRepo == nil || userRepo == nil {
		return nil, errors.New("repos must not be nil")
	}

	if cfg.SecretKey == "" {
		return nil, errors.New("secret key must not be empty")
	}

	setDefaultString := func(fld *string, def string) {
		if *fld == "" {
			*fld = def
		}
	}
	setDefaultDuration := func(fld *time.Duration, def time.Duration) {
		if *fld == 0 {
			*fld = def
		}
	}

	setDefaultString(&cfg.AccessHeaderName, defaultAccessHeaderName)
	setDefaultString(&cfg.AccessAuthScheme, defaultAccessAuthScheme)
	setDefaultString(&cfg.RefreshCookieName, defaultRefreshCookieName)
	setDefaultDuration(&cfg.AccessTokenTTL, defaultAccessTokenTTL)
	setDefaultDuration(&cfg.RefreshTokenTTL, defaultRefreshTokenTTL)

	// Set default bcrypt hasher if not user provided by user
	hasher := cfg.Hasher
	if hasher == nil {
		hasher = BcryptHasher{}
	}

	tokenManager := TokenManager{
		key:         cfg.SecretKey,
		alg:         jwt.GetSigningMethod(defaultSigningMethod),
		accessTTL:   cfg.AccessTokenTTL,
		refreshTTL:  cfg.RefreshTokenTTL,
		refreshRepo: refreshRepo,
	}

	return &AuthService{
		accessHeaderName:  cfg.AccessHeaderName,
		accessAuthScheme:  cfg.AccessAuthScheme,
		refreshCookieName: cfg.RefreshCookieName,

		token:    tokenManager,
		hasher:   hasher,
		userRepo: userRepo,
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

	pair, err = s.token.GeneratePair(ctx, user)
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

	pair, err = s.token.GeneratePair(ctx, user)
	if err != nil {
		return pair, fmt.Errorf("token could not be generated, sorry. Err: %w", err)
	}

	return pair, nil
}

func (s *AuthService) Refresh(ctx context.Context, refresh string) (models.TokenPair, error) {
	var pair models.TokenPair

	// Mark token as used
	// Always fail if token is not valid or not found
	token, err := s.token.UseRefreshToken(ctx, refresh)
	if err != nil {
		return pair, fmt.Errorf("token could not be refreshed. Err: %w", err)
	}

	// Check whether user is still exists
	user, err := s.userRepo.GetUserByID(ctx, token.UserID)
	if err != nil {
		return pair, fmt.Errorf("token could not be refreshed. Err: %w", err)
	}

	pair, err = s.token.GeneratePair(ctx, user)
	if err != nil {
		return pair, fmt.Errorf("token could not generated, sorry. Err: %w", err)
	}

	return pair, nil
}

func (s *AuthService) SetTokens(ctx context.Context, w http.ResponseWriter, pair models.TokenPair) {
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

func (s *AuthService) GetRefresh(r *http.Request) (string, error) {
	cookie, err := r.Cookie(s.refreshCookieName)
	if err != nil {
		return "", fmt.Errorf("can't read refresh token from cookie: %w", err)
	}

	return cookie.Value, nil
}

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

	userID, err := s.token.ParseAccess(ctx, token)
	if err != nil {
		return u, fmt.Errorf("token is not valid. Err: %w", err)
	}

	u, err = s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return u, fmt.Errorf("user not found. Err: %w", err)
	}

	return u, err
}
