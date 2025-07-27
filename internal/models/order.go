package models

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"time"
)

const (
	OrderStatusNew        = "NEW"
	OrderStatusProcessing = "PROCESSING"
	OrderStatusInvalid    = "INVALID"
	OrderStatusProcessed  = "PROCESSED"
)

type Order struct {
	ID         uuid.UUID
	Number     string
	UserID     uuid.UUID
	Status     string
	Accrual    *decimal.Decimal
	UploadedAt time.Time
	ModifiedAt time.Time
}
