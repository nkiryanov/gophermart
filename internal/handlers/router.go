package handlers

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/nkiryanov/gophermart/internal/handlers/render"
)

type middleware func(next http.Handler) http.Handler

// chain applies middlewares in the given order: m1(m2(...(h)))
func Chain(h http.Handler, mds ...middleware) http.Handler {
	for i := len(mds) - 1; i >= 0; i-- {
		h = mds[i](h)
	}
	return h
}

func NewRouter(
	authHandler *AuthHandler,
	orderHandler *OrderHandler,
	balanceHandler *BalanceHandler,
	authMiddleware middleware,
	mds ...middleware,
) http.Handler {
	withAuth := func(h http.Handler) http.Handler {
		return authMiddleware(h)
	}

	apiuser := http.NewServeMux()

	apiuser.Handle("/login", authHandler.Handler())
	apiuser.Handle("/register", authHandler.Handler())
	apiuser.Handle("/refresh", authHandler.Handler())

	apiuser.Handle("/orders", withAuth(orderHandler.Handler()))
	apiuser.Handle("/balance", withAuth(balanceHandler.Handler()))

	apiuser.Handle("GET /me", withAuth(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, _ := UserFromContext(r.Context())
			render.JSON(w, struct {
				ID       uuid.UUID `json:"id"`
				Username string    `json:"username"`
			}{ID: user.ID, Username: user.Username})
		}),
	))

	root := http.NewServeMux()
	root.Handle("/api/user/", http.StripPrefix("/api/user", apiuser))

	handler := Chain(root, mds...)

	return handler
}
