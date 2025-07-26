package middleware

import (
	"context"
	"net/http"

	"github.com/nkiryanov/gophermart/internal/handlers/render"
	"github.com/nkiryanov/gophermart/internal/handlers/userctx"
	"github.com/nkiryanov/gophermart/internal/models"
)

type authService interface {
	GetUserFromRequest(ctx context.Context, r *http.Request) (models.User, error)
}

func AuthMiddleware(authService authService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := authService.GetUserFromRequest(r.Context(), r)
			if err != nil {
				render.ServiceError(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := userctx.New(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
