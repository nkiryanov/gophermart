package handlers

import (
	"errors"
	"net/http"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/handlers/render"
	"github.com/nkiryanov/gophermart/internal/logger"
)

// Register user with username and password
func handleRegister(as authService, l logger.Logger) http.Handler {
	type request struct {
		Login    string `json:"login" validate:"required,min=2,max=50"`
		Password string `json:"password" validate:"required,min=8"`
	}
	type response struct {
		Message string `json:"message"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := render.BindAndValidate[request](w, r)
		if err != nil {
			return
		}

		pair, err := as.Register(r.Context(), data.Login, data.Password)
		if err != nil {
			switch {
			case errors.Is(err, apperrors.ErrUserAlreadyExists):
				render.ServiceError(w, "User already exists", http.StatusConflict)
			default:
				l.Error("Failed to register user", "error", err)
				render.ServiceError(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		as.SetTokenPairToResponse(w, pair)
		render.JSON(w, response{Message: "User registered successfully"})
	})
}

// Login user with username and password
func handleLogin(as authService, l logger.Logger) http.Handler {
	type request struct {
		Login    string `json:"login" validate:"required"`
		Password string `json:"password" validate:"required"`
	}
	type response struct {
		Message string `json:"message"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := render.BindAndValidate[request](w, r)
		if err != nil {
			// Consider to log errors here
			return
		}

		pair, err := as.Login(r.Context(), data.Login, data.Password)
		if err != nil {
			switch {
			case errors.Is(err, apperrors.ErrUserNotFound):
				render.ServiceError(w, "User not found", http.StatusUnauthorized)
			default:
				l.Error("Failed to login user", "error", err)
				render.ServiceError(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		as.SetTokenPairToResponse(w, pair)
		render.JSON(w, response{Message: "User logged in successfully"})
	})
}

// Refresh token pair using refresh token
func handleTokenRefresh(as authService, l logger.Logger) http.Handler {
	type response struct {
		Message string `json:"message"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		refresh, err := as.GetRefreshString(r)
		if err != nil {
			render.ServiceError(w, "Refresh token not found", http.StatusUnauthorized)
		}

		pair, err := as.RefreshPair(r.Context(), refresh)
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

		as.SetTokenPairToResponse(w, pair)
		render.JSON(w, response{Message: "Tokens refreshed successfully"})
	})
}
