package handlers

import (
	"context"

	"github.com/nkiryanov/gophermart/internal/models"
)

type ctxKey string

const userKey ctxKey = "user"

func NewContextWithUser(ctx context.Context, u models.User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

func UserFromContext(ctx context.Context) (models.User, bool) {
	u, ok := ctx.Value(userKey).(models.User)
	return u, ok
}
