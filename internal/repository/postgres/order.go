package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/models"
)

type OrderRepo struct {
	DB DBTX
}

// Create order with provided options
// If order with the number or id already exists return it as is
const createOrder = `-- name: CreateOrder
WITH insert_order AS (
	INSERT INTO orders (id, uploaded_at, modified_at, number, user_id, status, accrual)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	ON CONFLICT DO NOTHING
	RETURNING *
)
SELECT * FROM insert_order
UNION
SELECT * FROM orders WHERE number = $4
`

func (r *OrderRepo) CreateOrder(ctx context.Context, number string, userID uuid.UUID, opts ...models.OrderOption) (models.Order, error) {
	now := time.Now()
	orderID := uuid.New()

	// Order with defaults
	o := models.Order{
		ID:         orderID,
		UploadedAt: now,
		ModifiedAt: now,
		Number:     number,
		UserID:     userID,
		Status:     models.OrderNew,
	}

	for _, option := range opts {
		option(&o)
	}

	rows, _ := r.DB.Query(ctx, createOrder, o.ID, o.UploadedAt, o.ModifiedAt, o.Number, o.UserID, o.Status, o.Accrual)
	o, err := pgx.CollectOneRow(rows, rowToOrder)

	switch {
	case err != nil:
		return o, fmt.Errorf("db error: %w", err)
	case o.ID == orderID && o.UserID == userID:
		return o, nil
	case o.UserID != userID:
		return o, apperrors.ErrOrderNumberTaken
	case o.UserID == userID && o.ID != orderID:
		return o, apperrors.ErrOrderAlreadyExists
	default:
		return o, errors.New("programming error, should never be here")
	}

}

const listOrders = `
SELECT * FROM orders
WHERE user_id = $1
ORDER BY uploaded_at DESC
`

func (r *OrderRepo) ListOrders(ctx context.Context, userID uuid.UUID) ([]models.Order, error) {
	rows, _ := r.DB.Query(ctx, listOrders, userID)
	orders, err := pgx.CollectRows(rows, rowToOrder)

	switch err {
	case nil:
		return orders, nil
	default:
		return nil, fmt.Errorf("db error: %w", err)
	}
}

func rowToOrder(row pgx.CollectableRow) (models.Order, error) {
	var o models.Order
	err := row.Scan(&o.ID, &o.UploadedAt, &o.ModifiedAt, &o.Number, &o.UserID, &o.Status, &o.Accrual)
	return o, err
}
