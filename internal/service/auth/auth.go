package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nkiryanov/gophermart/internal/models"
)

const (
	defaultAccessHeaderName  = "Authorization"
	defaultAccessAuthScheme  = "Bearer"
	defaultRefreshCookieName = "refreshtoken"
)

type TokenManager interface {
	// GeneratePair generates access and refresh tokens for user
	GeneratePair(ctx context.Context, user models.User) (models.TokenPair, error)

	// UseRefresh marks refresh token as used and returns it
	UseRefresh(ctx context.Context, refresh string) (models.RefreshToken, error)

	// ParseAccess parses access token and returns user ID
	ParseAccess(ctx context.Context, access string) (userID uuid.UUID, err error)
}

type userService interface {
	// Create user with username and password
	CreateUser(ctx context.Context, username string, password string) (models.User, error)

	// Login user with username and password
	// Has to return apperrors.ErrUserNotFound if user not found
	Login(ctx context.Context, username string, password string) (models.User, error)

	// Get user by ID
	GetUserByID(ctx context.Context, userID uuid.UUID) (models.User, error)
}

// AuthService config with sensible defaults
// All fields are optional: if not set, default values will be used
type Config struct {
	AccessHeaderName  string
	AccessAuthScheme  string
	RefreshCookieName string
}

// Auth service
type AuthService struct {
	accessHeaderName  string
	accessAuthScheme  string
	refreshCookieName string

	// Manager to issue token pairs (access and refresh)
	tokenManager TokenManager

	// Service to create and get users
	userService userService
}

func NewService(cfg Config, tokenManager TokenManager, userService userService) (*AuthService, error) {
	setDefaultString := func(field *string, def string) {
		if *field == "" {
			*field = def
		}
	}
	setDefaultString(&cfg.AccessHeaderName, defaultAccessHeaderName)
	setDefaultString(&cfg.AccessAuthScheme, defaultAccessAuthScheme)
	setDefaultString(&cfg.RefreshCookieName, defaultRefreshCookieName)

	return &AuthService{
		accessHeaderName:  cfg.AccessHeaderName,
		accessAuthScheme:  cfg.AccessAuthScheme,
		refreshCookieName: cfg.RefreshCookieName,
		tokenManager:      tokenManager,
		userService:       userService,
	}, nil
}

func (s *AuthService) Register(ctx context.Context, username string, password string) (models.TokenPair, error) {
	var pair models.TokenPair

	user, err := s.userService.CreateUser(ctx, username, password)
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

	user, err := s.userService.Login(ctx, username, password)
	if err != nil {
		return pair, fmt.Errorf("can't login user. Err: %w", err)
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
	user, err := s.userService.GetUserByID(ctx, token.UserID)
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

// Authenticate and get user from request or return error
func (s *AuthService) GetUserFromRequest(ctx context.Context, r *http.Request) (models.User, error) {
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

	u, err = s.userService.GetUserByID(ctx, userID)
	if err != nil {
		return u, fmt.Errorf("user not found. Err: %w", err)
	}

	return u, err
}
