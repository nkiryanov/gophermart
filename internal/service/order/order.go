package order

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/repository"
	"github.com/nkiryanov/gophermart/internal/service/validate"
)

type OrderService struct {
	// Repository to access long term data
	storage repository.Storage
}

func NewService(storage repository.Storage) *OrderService {
	return &OrderService{
		storage: storage,
	}
}

type OrderOption func(*models.Order)

func (s *OrderService) CreateOrder(ctx context.Context, number string, user *models.User, opts ...repository.CreateOrderOption) (models.Order, error) {
	err := validate.Luhn(number)
	if err != nil {
		return models.Order{}, apperrors.ErrOrderNumberInvalid
	}
	return s.storage.Order().CreateOrder(ctx, number, user.ID, opts...)
}

func (s *OrderService) ListOrders(ctx context.Context, opts repository.ListOrdersOpts) ([]models.Order, error) {
	return s.storage.Order().ListOrders(ctx, opts)
}

func (s *OrderService) SetProcessed(ctx context.Context, number string, newStatus string, accrual decimal.Decimal) (models.Order, error) {
	var order models.Order

	if accrual.IsNegative() {
		return order, errors.New("accrual can't be negative")
	}

	err := s.storage.InTx(ctx, func(storage repository.Storage) error {
		var err error

		// lock order and order to update
		order, err = storage.Order().GetOrder(ctx, number, true)
		if err != nil {
			return err
		}
		_, err = storage.Balance().GetBalance(ctx, order.UserID, true)
		if err != nil {
			return err
		}

		if order.Status == models.OrderStatusProcessed || order.Status == models.OrderStatusInvalid {
			return apperrors.ErrOrderAlreadyProcessed
		}

		// Update order and related objects
		t, err := storage.Balance().CreateTransaction(ctx, models.Transaction{
			ID:          uuid.New(),
			ProcessedAt: time.Now(),
			UserID:      order.UserID,
			OrderNumber: order.Number,
			Type:        models.TransactionTypeAccrual,
			Amount:      accrual,
		})
		if err != nil {
			return err
		}
		_, err = storage.Balance().UpdateBalance(ctx, t)
		if err != nil {
			return err
		}
		order, err = storage.Order().UpdateOrder(ctx, number, &newStatus, &accrual)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return order, err
	}

	return order, nil
}
