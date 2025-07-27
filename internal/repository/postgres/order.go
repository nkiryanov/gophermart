package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/nkiryanov/gophermart/internal/repository"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/models"
)

type OrderRepo struct {
	DB DBTX
}

func (r *OrderRepo) CreateOrder(ctx context.Context, number string, userID uuid.UUID, opts ...repository.CreateOrderOption) (models.Order, error) {
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

	now := time.Now()
	orderID := uuid.New()

	// Order with defaults
	o := models.Order{
		ID:         orderID,
		UploadedAt: now,
		ModifiedAt: now,
		Number:     number,
		UserID:     userID,
		Status:     models.OrderStatusNew,
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

func (r *OrderRepo) ListOrders(ctx context.Context, params repository.ListOrdersParams) ([]models.Order, error) {
	args := []any{}
	argPos := 1
	whereParams := 0

	b := &strings.Builder{}
	fmt.Fprint(b, "SELECT * FROM orders\n")

	if params.UserID != nil {
		fmt.Fprintf(b, "WHERE user_id = $%d\n", argPos)
		args = append(args, *params.UserID)
		argPos++
		whereParams++
	}

	if len(params.Statuses) > 0 {
		if whereParams > 0 {
			fmt.Fprint(b, "AND ")
		} else {
			fmt.Fprint(b, "WHERE ")
		}
		fmt.Fprintf(b, "status = ANY($%d)\n", argPos)
		args = append(args, params.Statuses)
		argPos++
	}

	fmt.Fprint(b, "ORDER BY uploaded_at DESC\n")

	if params.Limit != nil {
		fmt.Fprintf(b, "LIMIT $%d\n", argPos)
		args = append(args, *params.Limit)
		argPos++
	}

	if params.Offset != nil {
		fmt.Fprintf(b, "OFFSET $%d\n", argPos)
		args = append(args, *params.Offset)
	}

	rows, _ := r.DB.Query(ctx, b.String(), args...)
	orders, err := pgx.CollectRows(rows, rowToOrder)

	switch err {
	case nil:
		return orders, nil
	default:
		return nil, fmt.Errorf("db error: %w", err)
	}
}

func (r OrderRepo) GetOrder(ctx context.Context, number string, lock bool) (models.Order, error) {
	const getOrder = `
	SELECT * FROM orders
	WHERE number = $1
	`

	const getOrderForUpdate = `
	SELECT * FROM orders
	WHERE number = $1
	FOR UPDATE
	`

	var query string

	switch lock {
	case true:
		query = getOrderForUpdate
	default:
		query = getOrder
	}

	rows, _ := r.DB.Query(ctx, query, number)
	order, err := pgx.CollectOneRow(rows, rowToOrder)

	switch {
	case err == nil:
		return order, nil
	case errors.Is(err, pgx.ErrNoRows):
		return order, apperrors.ErrOrderNotFound
	default:
		return order, fmt.Errorf("db error: %w", err)
	}
}

func (r *OrderRepo) UpdateOrder(ctx context.Context, number string, status *string, accrual *decimal.Decimal) (models.Order, error) {
	const updateOrder = `
	UPDATE orders
	SET status = coalesce($2, status), accrual = coalesce($3, accrual), modified_at = coalesce($4, modified_at)
	WHERE number = $1
	RETURNING *
	`
	var modifiedAt *time.Time

	if status != nil || accrual != nil {
		t := time.Now()
		modifiedAt = &t
	}

	rows, _ := r.DB.Query(ctx, updateOrder, number, status, accrual, modifiedAt)
	order, err := pgx.CollectOneRow(rows, rowToOrder)

	switch {
	case err == nil:
		return order, nil
	case errors.Is(err, pgx.ErrNoRows):
		return order, apperrors.ErrOrderNotFound
	default:
		return order, fmt.Errorf("db error: %w", err)
	}
}

func rowToOrder(row pgx.CollectableRow) (models.Order, error) {
	var o models.Order
	err := row.Scan(&o.ID, &o.UploadedAt, &o.ModifiedAt, &o.Number, &o.UserID, &o.Status, &o.Accrual)
	return o, err
}
