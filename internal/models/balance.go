package models

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"time"
)

const (
	TransactionTypeAccrual    = "accrual"
	TransactionTypeWithdrawal = "withdrawal"
)

type Balance struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Current   decimal.Decimal
	Withdrawn decimal.Decimal
}

type Transaction struct {
	ID          uuid.UUID
	ProcessedAt time.Time
	UserID      uuid.UUID
	OrderNumber string
	Type        string
	Amount      decimal.Decimal
}
