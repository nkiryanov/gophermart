package userctx

import (
	"context"

	"github.com/nkiryanov/gophermart/internal/models"
)

type ctxKey string

const userKey ctxKey = "user"

// Create a new context with the user
func New(ctx context.Context, u models.User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

// Extract the user from the context
func FromContext(ctx context.Context) (models.User, bool) {
	u, ok := ctx.Value(userKey).(models.User)
	return u, ok
}
