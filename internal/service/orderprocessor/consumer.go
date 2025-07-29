package orderprocessor

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nkiryanov/gophermart/internal/logger"
	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/service/accrual"
)

type Consumer struct {
	countWorkers int

	// Accrual client may return rate-limit errors
	// If the client is rate-limited, workers will wait until the time is up
	waitUntil atomic.Int64

	client       accrualClient
	orderService orderService
	logger       logger.Logger
}

func (c *Consumer) Consume(ctx context.Context, in <-chan models.Order) <-chan struct{} {
	idleStopped := make(chan struct{})

	var wg sync.WaitGroup
	for i := 0; i < c.countWorkers; i++ {
		wg.Add(1)
		go func() {
			c.worker(ctx, in)
			wg.Done()
		}()
	}

	go func() {
		defer close(idleStopped)
		wg.Wait()
		c.logger.Debug("Consumer stopped")
	}()

	return idleStopped
}

func (c *Consumer) worker(ctx context.Context, in <-chan models.Order) {
	for {
		// Wait unit rate limit is passed or context is done
		waitUntil := time.Unix(c.waitUntil.Load(), 0)
		if waitUntil.After(time.Now()) {
			c.logger.Debug("Worker is waiting for rate limit to reset", "wait_until", waitUntil)

			select {
			case <-ctx.Done():
				continue
			case <-time.After(time.Until(waitUntil)):
				c.logger.Debug("Worker finished waiting for rate limit to reset")
				continue
			}
		}

		select {
		case <-ctx.Done():
			return

		case order, ok := <-in:
			if !ok {
				c.logger.Debug("Consumer worker stopped, input channel closed")
				return
			}

			a, err := c.client.GetOrderAccrual(ctx, order.Number)
			var accErr *accrual.Error

			switch {
			case err == nil:
				order, err := c.orderService.SetProcessed(ctx, a.OrderNumber, a.Status, a.Accrual)
				if err != nil {
					c.logger.Error("Failed to set order as processed", "error", err, "order_number", order.Number)
				}

			case errors.As(err, &accErr):
				switch accErr.Code {
				case accrual.CodeRetryAfter:
					c.logger.Info("Rate limit exceeded, waiting", "retry_after", accErr.RetryAfter)
					c.waitUntil.Store(time.Now().Add(accErr.RetryAfter).Unix())

				case accrual.CodeNoContent:
					c.logger.Info("No content for order", "order_number", order.Number)
					order, err := c.orderService.SetProcessed(ctx, order.Number, models.OrderStatusInvalid, nil)
					if err != nil {
						c.logger.Error("Failed to set order as invalid", "error", err, "order_number", order.Number)
					}

				default:
					c.logger.Error("Unknown error from accrual service", "error", err, "order_number", order.Number)
				}

			default:
				c.logger.Error("unexpected error from accrual service", "error", err, "order_number", order.Number)
			}
		}
	}
}
