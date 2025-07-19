package handlers

import (
	"errors"
	"net/http"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/handlers/render"
	"github.com/nkiryanov/gophermart/internal/service"
)

type AuthHandler struct {
	authService service.Auth
}

func NewAuth(auth service.Auth) *AuthHandler {
	return &AuthHandler{authService: auth}
}

func (h *AuthHandler) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /register", h.register)
	mux.HandleFunc("POST /login", h.login)
	mux.HandleFunc("POST /refresh", h.refresh)

	return mux
}

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

	h.authService.SetTokens(r.Context(), w, pair)
	render.JSON(w, RegisterSuccessResponse{Message: "User registered successfully"})
}

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

	h.authService.SetTokens(r.Context(), w, pair)
	render.JSON(w, LoginSuccessResponse{Message: "User logged in successfully"})
}

func (h *AuthHandler) refresh(w http.ResponseWriter, r *http.Request) {
	type RefreshSuccessResponse struct {
		Message string `json:"message"`
	}

	refresh, err := h.authService.GetRefresh(r)
	if err != nil {
		render.ServiceError(w, "Refresh token not found", http.StatusUnauthorized)
	}

	pair, err := h.authService.Refresh(r.Context(), refresh)
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

	h.authService.SetTokens(r.Context(), w, pair)
	render.JSON(w, RefreshSuccessResponse{Message: "Tokens refreshed successfully"})
}
