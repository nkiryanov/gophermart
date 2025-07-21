package order

import (
	"context"

	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/repository"
)

type OrderService struct {
	// Repository to access long term data
	orderRepo repository.OrderRepo
}

func NewService(orderRepo repository.OrderRepo) *OrderService {
	return &OrderService{
		orderRepo: orderRepo,
	}
}

type OrderOption func(*models.Order)

func (s *OrderService) CreateOrder(ctx context.Context, number string, user *models.User, opts ...models.OrderOption) (models.Order, error) {
	return s.orderRepo.CreateOrder(ctx, number, user.ID, opts...)
}

func (s *OrderService) ListOrders(ctx context.Context, user *models.User) ([]models.Order, error) {
	return s.orderRepo.ListOrders(ctx, user.ID)
}
