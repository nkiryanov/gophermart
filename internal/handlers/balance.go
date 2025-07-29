package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/nkiryanov/gophermart/internal/apperrors"
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
			return
		default:
			l.Error("Failed to get balance", "error", err)
			render.ServiceError(w, "Internal server error", http.StatusInternalServerError)
		}
	})

}

func handleWithdraw(userService userService, l logger.Logger) http.Handler {
	type request struct {
		OrderNumber string          `json:"order"`
		Sum         decimal.Decimal `json:"sum"`
	}

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

		withdraw, err := render.BindAndValidate[request](w, r)
		if err != nil {
			return
		}

		balance, err := userService.Withdraw(r.Context(), user.ID, withdraw.OrderNumber, withdraw.Sum)

		switch {
		case err == nil:
			current, _ := balance.Current.Float64()
			withdrawn, _ := balance.Withdrawn.Float64()
			render.JSON(w, response{current, withdrawn})
			return
		case errors.Is(err, apperrors.ErrBalanceInsufficient):
			render.ServiceError(w, "Insufficient balance", http.StatusPaymentRequired)
		case errors.Is(err, apperrors.ErrOrderNumberInvalid):
			render.ServiceError(w, "Invalid order number", http.StatusUnprocessableEntity)
		default:
			l.Error("Failed to get balance", "error", err)
			render.ServiceError(w, "Internal server error", http.StatusInternalServerError)
		}
	})
}

func handleListWithdrawals(userService userService, l logger.Logger) http.Handler {
	type withdrawal struct {
		Order       string    `json:"order"`
		Sum         float64   `json:"sum"`
		ProcessedAt time.Time `json:"processed_at"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := userctx.FromContext(r.Context())
		if !ok {
			render.ServiceError(w, "Internal service error", http.StatusInternalServerError)
			return
		}

		tr, err := userService.GetWithdrawals(r.Context(), user.ID)

		switch err {
		case nil:
			withdrawals := make([]withdrawal, 0, len(tr))
			for _, t := range tr {
				sum, _ := t.Amount.Float64()
				withdrawals = append(withdrawals, withdrawal{
					Order:       t.OrderNumber,
					Sum:         sum,
					ProcessedAt: t.ProcessedAt,
				})
			}
			render.JSON(w, withdrawals)
			return
		default:
			l.Error("Failed to get withdrawals", "error", err)
			render.ServiceError(w, "Internal server error", http.StatusInternalServerError)
		}
	})
}
