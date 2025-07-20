package handlers

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/nkiryanov/gophermart/internal/handlers/render"
)

type middleware func(next http.Handler) http.Handler

func NewRouter(authHandler *AuthHandler, authMiddleware middleware) http.Handler {
	withAuth := func(h http.HandlerFunc) http.Handler {
		return authMiddleware(h)
	}

	// Root /
	mux := http.NewServeMux()

	// Set /auth/ routes
	mux.Handle("/auth/", http.StripPrefix("/auth", authHandler.Handler()))
	mux.Handle("GET /auth/me", withAuth(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, _ := UserFromContext(r.Context())
			render.JSON(w, struct {
				ID       uuid.UUID `json:"id"`
				Username string    `json:"username"`
			}{ID: user.ID, Username: user.Username})
		}),
	))

	return mux
}
