package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/handlers/render"
	"github.com/nkiryanov/gophermart/internal/models"
)

// Auth service
type AuthService interface {
	Register(ctx context.Context, username string, password string) (models.TokenPair, error)
	Login(ctx context.Context, username string, password string) (models.TokenPair, error)
	Refresh(ctx context.Context, refresh string) (models.TokenPair, error)

	SetAuth(ctx context.Context, w http.ResponseWriter, tokens models.TokenPair)
	ReadRefreshToken(r *http.Request) (string, error)
}

type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type AuthHandler struct {
	auth AuthService
}

func (h *AuthHandler) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/register", h.register)
	mux.HandleFunc("/login", h.login)
	mux.HandleFunc("/refresh", h.refresh)

	return mux
}

func (h *AuthHandler) register(w http.ResponseWriter, r *http.Request) {
	type RegisterRequest struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	type RegisterSuccessResponse struct {
		Message string `json:"message"`
	}

	data, err := render.BindAndValidate[RegisterRequest](w, r)
	if err != nil {
		// Consider to log errors here
		return
	}

	pair, err := h.auth.Register(r.Context(), data.Login, data.Password)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrUserAlreadyExists):
			render.WriteServiceError(w, "User not found", http.StatusConflict)
		default:
			render.WriteServiceError(w, "Internal server error", http.StatusInternalServerError)
		}
	}

	h.auth.SetAuth(r.Context(), w, pair)
	render.JSON(w, RegisterSuccessResponse{Message: "User registered successfully"})
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	type LoginRequest struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	type LoginSuccessResponse struct {
		Message string `json:"message"`
	}

	data, err := render.BindAndValidate[LoginRequest](w, r)
	if err != nil {
		// Consider to log errors here
		return
	}

	pair, err := h.auth.Login(r.Context(), data.Login, data.Password)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrUserNotFound):
			render.WriteServiceError(w, "User not found", http.StatusUnauthorized)
		default:
			render.WriteServiceError(w, "Internal server error", http.StatusInternalServerError)
		}
	}

	h.auth.SetAuth(r.Context(), w, pair)
	render.JSON(w, LoginSuccessResponse{Message: "User logged in successfully"})
}

func (h *AuthHandler) refresh(w http.ResponseWriter, r *http.Request) {
	type RefreshSuccessResponse struct {
		Message string `json:"message"`
	}

	refresh, err := h.auth.ReadRefreshToken(r)
	if err != nil {
		render.WriteServiceError(w, "Refresh token not found", http.StatusUnauthorized)
	}

	pair, err := h.auth.Refresh(r.Context(), refresh)
	if err != nil {
		// Consider to log errors here
		switch {
		case errors.Is(err, apperrors.ErrRefreshTokenExpired):
			render.WriteServiceError(w, "Refresh token not expired", http.StatusUnauthorized)
		default:
			render.WriteServiceError(w, "Refresh token not found", http.StatusUnauthorized)
		}
	}

	h.auth.SetAuth(r.Context(), w, pair)
	render.JSON(w, RefreshSuccessResponse{Message: "Tokens refreshed successfully"})
}
