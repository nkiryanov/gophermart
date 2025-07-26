package handlers

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/handlers/render"
	"github.com/nkiryanov/gophermart/internal/handlers/userctx"
	"github.com/nkiryanov/gophermart/internal/logger"
	"github.com/nkiryanov/gophermart/internal/models"
)

type orderResponse struct {
	Number     string           `json:"number"`
	Status     string           `json:"status"`
	Accrual    *decimal.Decimal `json:"accrual,omitempty"`
	UploadedAt time.Time        `json:"uploaded_at"`
}

func orderToResponse(o *models.Order) orderResponse {
	r := orderResponse{
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

func handleCreateOrder(orderService orderService, l logger.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := userctx.FromContext(r.Context())
		if !ok {
			l.Error("Failed to get user from context", "uri", r.RequestURI)
			render.ServiceError(w, "Internal service error", http.StatusInternalServerError)
			return
		}

		// Read order number from request body
		r.Body = http.MaxBytesReader(nil, r.Body, 512)
		number, err := io.ReadAll(r.Body)
		if err != nil {
			render.ServiceError(w, "Failed to read request body", http.StatusBadRequest)
		}

		order, err := orderService.CreateOrder(r.Context(), string(number), &user)

		switch {
		case err == nil:
			render.JSONWithStatus(w, orderToResponse(&order), http.StatusAccepted)
		case errors.Is(err, apperrors.ErrOrderNumberInvalid):
			render.ServiceError(w, "Invalid order number", http.StatusUnprocessableEntity)
		case errors.Is(err, apperrors.ErrOrderAlreadyExists):
			render.JSONWithStatus(w, orderToResponse(&order), http.StatusOK)
		case errors.Is(err, apperrors.ErrOrderNumberTaken):
			render.ServiceError(w, "Order number already taken", http.StatusConflict)
		default:
			l.Error("Failed to create order", "error", err)
			render.ServiceError(w, "Internal server error", http.StatusInternalServerError)
		}
	})
}

func handleListOrder(orderService orderService, l logger.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := userctx.FromContext(r.Context())
		if !ok {
			l.Error("Failed to get user from context", "uri", r.RequestURI)
			render.ServiceError(w, "Internal service error", http.StatusInternalServerError)
			return
		}

		orders, err := orderService.ListOrders(r.Context(), &user)
		if err != nil {
			render.ServiceError(w, "Failed to list orders", http.StatusInternalServerError)
			return
		}

		if len(orders) == 0 {
			render.JSONWithStatus(w, []orderResponse{}, http.StatusNoContent)
			return
		}

		resp := make([]orderResponse, len(orders))
		for i, order := range orders {
			resp[i] = orderToResponse(&order)
		}

		render.JSON(w, resp)
	})
}
