package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/handlers/render"
	"github.com/nkiryanov/gophermart/internal/models"
)

type orderService interface {
	CreateOrder(ctx context.Context, number string, user *models.User) (models.Order, error)
}

type OrderHandler struct {
	orderService orderService
}

type OrderResponse struct {
	Number     string           `json:"number"`
	Status     string           `json:"status"`
	Accrual    *decimal.Decimal `json:"accrual,omitempty"`
	UploadedAt time.Time        `json:"uploaded_at"`
}

func NewOrder(orderService orderService) *OrderHandler {
	return &OrderHandler{orderService: orderService}
}

func (h *OrderHandler) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders", h.create)

	return mux
}

func (h *OrderHandler) create(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		render.ServiceError(w, "Internal service error", http.StatusInternalServerError)
		return
	}

	// Read order number from request body
	r.Body = http.MaxBytesReader(nil, r.Body, 512)
	number, err := io.ReadAll(r.Body)
	if err != nil {
		render.ServiceError(w, "Failed to read request body", http.StatusBadRequest)
	}

	order, err := h.orderService.CreateOrder(r.Context(), string(number), &user)
	res := OrderResponse{
		Number:     order.Number,
		Status:     order.Status,
		Accrual:    nil,
		UploadedAt: order.UploadedAt,
	}
	if !order.Accrual.IsZero() {
		res.Accrual = &order.Accrual
	}

	switch {
	case err == nil:
		render.JSONWithStatus(w, res, http.StatusAccepted)
	case errors.Is(err, apperrors.ErrOrderAlreadyExists):
		render.JSONWithStatus(w, res, http.StatusOK)
	case errors.Is(err, apperrors.ErrOrderNumberTaken):
		render.ServiceError(w, "Order number already taken", http.StatusConflict)
	default:
		render.ServiceError(w, "Internal server error", http.StatusInternalServerError)
	}
}
