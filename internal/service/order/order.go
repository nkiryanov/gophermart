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

func (s *OrderService) CreateOrder(ctx context.Context, number string, user *models.User) (models.Order, error) {
	return s.orderRepo.CreateOrder(ctx, number, user.ID)
}
