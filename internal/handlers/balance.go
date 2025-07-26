package handlers

import (
	"net/http"

	"github.com/nkiryanov/gophermart/internal/handlers/render"
	"github.com/nkiryanov/gophermart/internal/handlers/userctx"
	"github.com/nkiryanov/gophermart/internal/logger"
)

func handleUserBalance(userService userService, l logger.Logger) http.Handler {
	type response struct {
		Current   float64 `json:"current"`
		Withdrawn float64 `json:"withdrawn"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := userctx.FromContext(r.Context())
		if !ok {
			render.ServiceError(w, "Internal service error", http.StatusInternalServerError)
			return
		}

		// Read order number from request body
		balance, err := userService.GetBalance(r.Context(), user.ID)

		switch err {
		case nil:
			current, _ := balance.Current.Float64()
			withdrawn, _ := balance.Withdrawn.Float64()
			render.JSON(w, response{current, withdrawn})
		default:
			l.Error("Failed to get balance", "error", err)
			render.ServiceError(w, "Internal server error", http.StatusInternalServerError)
		}
	})

}
