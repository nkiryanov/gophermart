package middleware

import (
	"context"
	"net/http"

	"github.com/nkiryanov/gophermart/internal/handlers"
	"github.com/nkiryanov/gophermart/internal/handlers/render"
	"github.com/nkiryanov/gophermart/internal/models"
)

type authService interface {
	Auth(ctx context.Context, r *http.Request) (models.User, error)
}

func AuthMiddleware(as authService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := as.Auth(r.Context(), r)
			if err != nil {
				render.ServiceError(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := handlers.NewContextWithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
