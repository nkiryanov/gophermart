package accrual

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/nkiryanov/gophermart/internal/logger"
)

const (
	CodeRetryAfter = "retry-after"
	CodeNoContent  = "no-content"
	CodeUnknown    = "unknown"
)

type AccrualError struct {
	Code string

	RetryAfter time.Duration
	Err        error
}

func (ra *AccrualError) Error() string {
	return fmt.Sprintf("code: %s, retry_after: %d, error: %v", ra.Code, ra.RetryAfter, ra.Err)
}

func NewAccrualError(code string, retryAfter int, err error) *AccrualError {
	return &AccrualError{
		Code:       code,
		RetryAfter: time.Duration(retryAfter) * time.Second,
		Err:        err,
	}
}

type OrderAccrual struct {
	OrderNumber string           `json:"order"`
	Status      string           `json:"status"`
	Accrual     *decimal.Decimal `json:"accrual,omitempty"`
}

type Client struct {
	AccrualAddr string

	client *http.Client
	logger logger.Logger
}

func NewClient(addr string) *Client {
	return &Client{
		AccrualAddr: addr,
		client:      &http.Client{},
	}
}

func (c *Client) GetOrderAccrual(ctx context.Context, number string) (OrderAccrual, error) {
	var accrual OrderAccrual

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.AccrualAddr+"/api/orders/"+number, nil)
	if err != nil {
		return accrual, NewAccrualError(CodeUnknown, 0, fmt.Errorf("failed to create request: %w", err))
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return accrual, NewAccrualError(CodeUnknown, 0, fmt.Errorf("failed to send request: %w", err))
	}
	defer resp.Body.Close() // nolint:errcheck

	switch resp.StatusCode {
	case http.StatusOK:
		return c.processSuccess(resp)
	case http.StatusTooManyRequests:
		return c.processTooManyRequest(resp)
	case http.StatusNoContent:
		return accrual, NewAccrualError(CodeNoContent, 0, fmt.Errorf("no content for order %s", number))
	default:
		c.logger.Warn("Failed to get order", "status_code", resp.StatusCode, "order_number", number)
		return accrual, NewAccrualError(CodeUnknown, 0, fmt.Errorf("unknown status code %d for order %s", resp.StatusCode, number))
	}
}

func (c *Client) processSuccess(resp *http.Response) (OrderAccrual, error) {
	var a OrderAccrual
	err := json.NewDecoder(resp.Body).Decode(&a)
	if err != nil {
		c.logger.Warn("Failed to decode response", "error", err)
		return a, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debug("Accrual response", "order", a.OrderNumber, "status", a.Status, "accrual", a.Accrual)
	return a, nil
}

func (c *Client) processTooManyRequest(resp *http.Response) (OrderAccrual, error) {
	header := resp.Header.Get("Retry-After")
	retryAfter, err := strconv.Atoi(strings.TrimSpace(header))
	if err != nil {
		retryAfter = 60 // default to 60 seconds if parsing fails
	}

	c.logger.Warn("Accrual service throttled", "retry_after", retryAfter)
	return OrderAccrual{}, NewAccrualError(CodeRetryAfter, retryAfter, fmt.Errorf("retry after %d seconds", retryAfter))
}
