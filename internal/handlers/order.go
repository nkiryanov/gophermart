package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/handlers/render"
	"github.com/nkiryanov/gophermart/internal/models"
)

type orderService interface {
	CreateOrder(ctx context.Context, number string, user *models.User, opts ...models.OrderOption) (models.Order, error)
	ListOrders(ctx context.Context, user *models.User) ([]models.Order, error)
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
	mux.HandleFunc("GET /orders", h.list)

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
	resp := orderToResponse(&order)

	switch {
	case err == nil:
		render.JSONWithStatus(w, resp, http.StatusAccepted)
	case errors.Is(err, apperrors.ErrOrderAlreadyExists):
		render.JSONWithStatus(w, resp, http.StatusOK)
	case errors.Is(err, apperrors.ErrOrderNumberTaken):
		render.ServiceError(w, "Order number already taken", http.StatusConflict)
	default:
		render.ServiceError(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *OrderHandler) list(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		render.ServiceError(w, "Internal service error", http.StatusInternalServerError)
		return
	}

	orders, err := h.orderService.ListOrders(r.Context(), &user)
	if err != nil {
		render.ServiceError(w, "Failed to list orders", http.StatusInternalServerError)
		return
	}

	if len(orders) == 0 {
		render.JSONWithStatus(w, []OrderResponse{}, http.StatusNoContent)
		return
	}

	resp := make([]OrderResponse, len(orders))
	for i, order := range orders {
		resp[i] = orderToResponse(&order)
	}

	render.JSON(w, resp)
}

func orderToResponse(o *models.Order) OrderResponse {
	r := OrderResponse{
		Number:     o.Number,
		Status:     strings.ToUpper(o.Status),
		Accrual:    nil,
		UploadedAt: o.UploadedAt,
	}
	if !o.Accrual.IsZero() {
		r.Accrual = &o.Accrual
	}
	return r
}
