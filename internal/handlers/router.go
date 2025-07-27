package handlers

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/nkiryanov/gophermart/internal/handlers/middleware"
	"github.com/nkiryanov/gophermart/internal/logger"
	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/repository"
)

// chain applies middlewares in the given order: m1(m2(...(h)))
func chain(h http.Handler, mds ...func(next http.Handler) http.Handler) http.Handler {
	for i := len(mds) - 1; i >= 0; i-- {
		h = mds[i](h)
	}
	return h
}

func NewRouter(
	authService authService,
	orderService orderService,
	userService userService,
	logger logger.Logger,
) http.Handler {
	authMiddleware := middleware.AuthMiddleware(authService)
	withAuth := func(h http.Handler) http.Handler {
		return authMiddleware(h)
	}

	apiuser := http.NewServeMux()

	apiuser.Handle("/login", handleLogin(authService, logger))
	apiuser.Handle("/register", handleRegister(authService, logger))
	apiuser.Handle("/refresh", handleTokenRefresh(authService, logger))

	apiuser.Handle("POST /orders", withAuth(handleCreateOrder(orderService, logger)))
	apiuser.Handle("GET /orders", withAuth(handleListOrder(orderService, logger)))
	apiuser.Handle("GET /balance", withAuth(handleUserBalance(userService, logger)))
	apiuser.Handle("POST /balance/withdraw", withAuth(handleWithdraw(userService, logger)))
	apiuser.Handle("GET /withdrawals", withAuth(handleListWithdrawals(userService, logger)))
	apiuser.Handle("GET /me", withAuth(handleUserMe()))

	root := http.NewServeMux()
	root.Handle("/api/user/", http.StripPrefix("/api/user", apiuser))

	handler := chain(root,
		middleware.LoggerMiddleware(logger),
	)

	return handler
}

type authService interface {
	// Register user with username and password
	// Has to return apperrors.ErrUserAlreadyExists if user already exists
	Register(ctx context.Context, username string, password string) (models.TokenPair, error)

	// Login user with username and password
	// Has to return apperrors.ErrUserNotFound if user not found
	Login(ctx context.Context, username string, password string) (models.TokenPair, error)

	// Refresh tokens using refresh token
	// If token expired: has to return apperrors.ErrRefreshTokenExpired
	// If token not found: has to return apperrors.ErrRefreshTokenNotFound
	RefreshPair(ctx context.Context, refresh string) (models.TokenPair, error)

	// Set auth tokens (access, refresh) to response
	SetTokenPairToResponse(w http.ResponseWriter, pair models.TokenPair)

	// Get refresh token from request
	GetRefreshString(r *http.Request) (string, error)

	// Get request and return user if it authenticated or error
	GetUserFromRequest(ctx context.Context, r *http.Request) (models.User, error)
}

type orderService interface {
	CreateOrder(ctx context.Context, number string, user *models.User, opts ...repository.CreateOrderOption) (models.Order, error)
	ListOrders(ctx context.Context, user *models.User) ([]models.Order, error)
}

type userService interface {
	GetBalance(ctx context.Context, userID uuid.UUID) (models.Balance, error)
	Withdraw(ctx context.Context, userID uuid.UUID, orderNum string, amount decimal.Decimal) (models.Balance, error)
	GetWithdrawals(ctx context.Context, userID uuid.UUID) ([]models.Transaction, error)
}
