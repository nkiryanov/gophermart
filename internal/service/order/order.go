package order

import (
	"context"

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
	return s.storage.Order().CreateOrder(ctx, number, user.ID, opts...)
}

func (s *OrderService) ListOrders(ctx context.Context, user *models.User) ([]models.Order, error) {
	return s.storage.Order().ListOrders(ctx, user.ID)
}
