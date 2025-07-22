package handlers

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/nkiryanov/gophermart/internal/handlers/render"
	"github.com/nkiryanov/gophermart/internal/models"
)

type userService interface {
	GetBalance(ctx context.Context, userID uuid.UUID) (models.Balance, error)
}

type BalanceHandler struct {
	userService userService
}

func NewBalance(userService userService) *BalanceHandler {
	return &BalanceHandler{userService: userService}
}

func (h *BalanceHandler) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /balance", h.balance)

	return mux
}

func (h *BalanceHandler) balance(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		render.ServiceError(w, "Internal service error", http.StatusInternalServerError)
		return
	}

	type balanceResponse struct {
		Current   float64 `json:"current"`
		Withdrawn float64 `json:"withdrawn"`
	}

	// Read order number from request body
	balance, err := h.userService.GetBalance(r.Context(), user.ID)

	switch err {
	case nil:
		current, _ := balance.Current.Float64()
		withdrawn, _ := balance.Withdrawn.Float64()
		render.JSON(w, balanceResponse{current, withdrawn})
	default:
		render.ServiceError(w, "Internal server error", http.StatusInternalServerError)
	}
}
