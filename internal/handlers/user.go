package handlers

import (
	"github.com/google/uuid"
	"net/http"

	"github.com/nkiryanov/gophermart/internal/handlers/render"
	"github.com/nkiryanov/gophermart/internal/handlers/userctx"
)

func handleUserMe() http.Handler {
	type response struct {
		ID       uuid.UUID `json:"id"`
		Username string    `json:"username"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := userctx.FromContext(r.Context())
		render.JSON(w, response{ID: user.ID, Username: user.Username})
	})
}
