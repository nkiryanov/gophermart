package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/handlers/render"
	"github.com/nkiryanov/gophermart/internal/models"
)

const ()

type authService interface {
	// Register user with username and password
	// Has to return apperrors.ErrUserAlreadyExists if user already exists
	Register(ctx context.Context, username string, password string) (models.TokenPair, error)

	// Login user with username and password
	// Has to return apperrors.ErrUserNotFound if user not found
	Login(ctx context.Context, username string, password string) (models.TokenPair, error)

	// Refresh tokens using refresh token
	// If token expired: has to return apperrors.ErrRefreshTokenExpired
	// If token not found: has to return apperrors.ErrRefreshTokenNotFound
	RefreshPair(ctx context.Context, refresh string) (models.TokenPair, error)

	// Set auth tokens (access, refresh) to response
	WriteTokenPair(ctx context.Context, w http.ResponseWriter, pair models.TokenPair)

	// Get refresh token from request
	GetRefreshString(r *http.Request) (string, error)
}

type AuthHandler struct {
	authService authService
}

func NewAuth(authService authService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /register", h.register)
	mux.HandleFunc("POST /login", h.login)
	mux.HandleFunc("POST /refresh", h.refresh)

	return mux
}

// Register user with username and password
func (h *AuthHandler) register(w http.ResponseWriter, r *http.Request) {
	type RegisterRequest struct {
		Login    string `json:"login" validate:"required,min=2,max=50"`
		Password string `json:"password" validate:"required,min=8"`
	}
	type RegisterSuccessResponse struct {
		Message string `json:"message"`
	}

	data, err := render.BindAndValidate[RegisterRequest](w, r)
	if err != nil {
		// Consider to log errors here
		return
	}

	pair, err := h.authService.Register(r.Context(), data.Login, data.Password)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrUserAlreadyExists):
			render.ServiceError(w, "User already exists", http.StatusConflict)
		default:
			render.ServiceError(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	h.authService.WriteTokenPair(r.Context(), w, pair)
	render.JSON(w, RegisterSuccessResponse{Message: "User registered successfully"})
}

// Login user with username and password
func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	type LoginRequest struct {
		Login    string `json:"login" validate:"required"`
		Password string `json:"password" validate:"required"`
	}
	type LoginSuccessResponse struct {
		Message string `json:"message"`
	}

	data, err := render.BindAndValidate[LoginRequest](w, r)
	if err != nil {
		// Consider to log errors here
		return
	}

	pair, err := h.authService.Login(r.Context(), data.Login, data.Password)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrUserNotFound):
			render.ServiceError(w, "User not found", http.StatusUnauthorized)
		default:
			render.ServiceError(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	h.authService.WriteTokenPair(r.Context(), w, pair)
	render.JSON(w, LoginSuccessResponse{Message: "User logged in successfully"})
}

// Refresh token pair using refresh token
func (h *AuthHandler) refresh(w http.ResponseWriter, r *http.Request) {
	type RefreshSuccessResponse struct {
		Message string `json:"message"`
	}

	refresh, err := h.authService.GetRefreshString(r)
	if err != nil {
		render.ServiceError(w, "Refresh token not found", http.StatusUnauthorized)
	}

	pair, err := h.authService.RefreshPair(r.Context(), refresh)
	if err != nil {
		// Consider to log errors here
		switch {
		case errors.Is(err, apperrors.ErrRefreshTokenExpired):
			render.ServiceError(w, "Refresh token expired", http.StatusUnauthorized)
		default:
			render.ServiceError(w, "Refresh token not found", http.StatusUnauthorized)
		}
		return
	}

	h.authService.WriteTokenPair(r.Context(), w, pair)
	render.JSON(w, RefreshSuccessResponse{Message: "Tokens refreshed successfully"})
}
