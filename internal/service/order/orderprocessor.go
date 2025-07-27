package order

import (
	"context"

	"github.com/nkiryanov/gophermart/internal/logger"
	"github.com/nkiryanov/gophermart/internal/service/accrual"
)


type AccrualClient interface {
	GetOrderAccrual(ctx context.Context, number string) (accrual.OrderAccrual, error)
}


type OrderProcessor struct {
	CountWorkers int
	Client       AccrualClient

	L logger.Logger
}
