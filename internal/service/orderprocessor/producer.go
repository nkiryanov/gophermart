package orderprocessor

import (
	"context"
	"time"

	"github.com/nkiryanov/gophermart/internal/logger"
	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/repository"
)

type Producer struct {
	interval     time.Duration
	logger       logger.Logger
	orderService orderService
}

func (p *Producer) Produce(ctx context.Context, out chan<- models.Order) <-chan struct{} {
	idleStopped := make(chan struct{})

	go func() {
		defer close(idleStopped)

		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				p.logger.Debug("Producer stopped by context")
				return

			case <-ticker.C:
				p.logger.Debug("Producer tick", "interval", p.interval)

				orders, err := p.orderService.ListOrders(ctx, repository.ListOrdersOpts{
					Statuses: []string{models.OrderStatusNew, models.OrderStatusProcessing},
					Limit:    100, // Limit to avoid overwhelming the channel
				})
				if err != nil {
					p.logger.Error("Failed to list orders", "error", err)
					continue
				}

				// Send orders to the output channel
				for _, order := range orders {
					select {
					case <-ctx.Done():
						p.logger.Debug("Producer stopped by context while sending orders")
						return
					case out <- order:
						p.logger.Debug("Order sent to channel", "orderID", order.ID)
					}
				}
			}
		}
	}()

	return idleStopped
}
