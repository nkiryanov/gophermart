package models

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"time"
)

const (
	OrderStatusNew        = "new"
	OrderStatusProcessing = "processing"
	OrderStatusInvalid    = "invalid"
	OrderStatusProcessed  = "processed"
)

type Order struct {
	ID         uuid.UUID
	Number     string
	UserID     uuid.UUID
	Status     string
	Accrual    decimal.Decimal
	UploadedAt time.Time
	ModifiedAt time.Time
}

type OrderOption func(*Order)

func WithOrderStatus(s string) func(*Order) {
	return func(o *Order) { o.Status = s }
}
func WithOrderAccrual(d decimal.Decimal) func(o *Order) {
	return func(o *Order) { o.Accrual = d }
}

func WithUploadedAt(t time.Time) func(*Order) {
	return func(o *Order) { o.UploadedAt = t }
}
