package order

import (
	"context"
	"errors"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/repository"
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

func (s *OrderService) CreateOrder(ctx context.Context, number string, user *models.User, opts ...models.OrderOption) (models.Order, error) {
	err := validateLuhn(number)
	if err != nil {
		return models.Order{}, apperrors.ErrOrderNumberInvalid
	}
	return s.storage.Order().CreateOrder(ctx, number, user.ID, opts...)
}

func (s *OrderService) ListOrders(ctx context.Context, user *models.User) ([]models.Order, error) {
	return s.storage.Order().ListOrders(ctx, user.ID)
}

func validateLuhn(number string) error {
	// Convert number in digits and save in slice in reverse order
	// It's ok to work with string as bytes here
	digits := make([]int, 0, len(number))
	for i := len(number) - 1; i >= 0; i-- {
		n := number[i]
		if n < '0' || n > '9' {
			return errors.New("number contains invalid characters")
		}
		digits = append(digits, int(n-'0'))
	}

	sum := 0
	for i, digit := range digits {
		position := i + 1
		if position%2 == 0 {
			digit *= 2
			if digit > 9 {
				digit = (digit % 10) + 1
			}
		}

		sum += digit
	}

	switch sum % 10 {
	case 0:
		return nil
	default:
		return errors.New("number is not valid according to Luhn algorithm")
	}
}
