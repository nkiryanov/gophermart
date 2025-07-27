package orderprocessor

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	"github.com/nkiryanov/gophermart/internal/logger"
	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/repository"
	"github.com/nkiryanov/gophermart/internal/service/accrual"
)

const (
	defaultCountWorkers    = 10               // Number of workers to process orders
	defaultProduceInterval = 10 * time.Second // Interval for producing orders
)

type accrualClient interface {
	GetOrderAccrual(ctx context.Context, number string) (accrual.OrderAccrual, error)
}

type orderService interface {
	SetProcessed(ctx context.Context, number string, newStatus string, accrual *decimal.Decimal) (models.Order, error)
	ListOrders(ctx context.Context, opts repository.ListOrdersOpts) ([]models.Order, error)
}

type Processor struct {
	consumer *Consumer
	producer *Producer
}

func New(accrualAddr string, logger logger.Logger, orderService orderService) *Processor {
	client := accrual.NewClient(accrualAddr, logger)

	return &Processor{
		consumer: &Consumer{
			countWorkers: defaultCountWorkers,
			client:       client,
			orderService: orderService,
			logger:       logger,
		},
		producer: &Producer{
			interval:     defaultProduceInterval,
			orderService: orderService,
			logger:       logger,
		},
	}
}

func (op *Processor) Process(ctx context.Context) <-chan struct{} {
	idleStopped := make(chan struct{})

	orderChan := make(chan models.Order)

	// Start producer to produce orders
	producerStopped := op.producer.Produce(ctx, orderChan)

	// Start consumer to process orders
	consumerStopped := op.consumer.Consume(ctx, orderChan)

	go func() {
		defer close(idleStopped)
		defer close(orderChan)
		<-producerStopped
		<-consumerStopped
		op.consumer.logger.Debug("OrderProcessor stopped")
	}()

	return idleStopped
}
