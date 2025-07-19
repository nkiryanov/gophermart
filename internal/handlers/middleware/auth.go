package middleware

import (
	"net/http"

	"github.com/nkiryanov/gophermart/internal/handlers"
	"github.com/nkiryanov/gophermart/internal/service"
)

type AuthMiddleware struct {
	authService service.Auth
}

func NewAuth(authService service.Auth) *AuthMiddleware {
	return &AuthMiddleware{authService: authService}
}

func (m *AuthMiddleware) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := m.authService.Auth(r.Context(), r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := handlers.NewContextWithUser(r.Context(), user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
