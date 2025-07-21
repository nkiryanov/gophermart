package e2e

import (
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/handlers"
	"github.com/nkiryanov/gophermart/internal/handlers/middleware"
	"github.com/nkiryanov/gophermart/internal/repository/postgres"
	"github.com/nkiryanov/gophermart/internal/service/auth"
	"github.com/nkiryanov/gophermart/internal/service/auth/tokenmanager"
	"github.com/nkiryanov/gophermart/internal/service/order"
	"github.com/nkiryanov/gophermart/internal/service/user"
	"github.com/nkiryanov/gophermart/internal/testutil"
)

type Services struct {
	AuthService  *auth.AuthService
	OrderService *order.OrderService
	UserService  *user.UserService
}

// Create db transaction and run server in with that connection (one connection cause one transaction)
// The created transaction passed to inner function: so, you can safely use testutil.WithTx with it
func ServeWithTx(dbpool *pgxpool.Pool, t *testing.T, fn func(tx pgx.Tx, srvURL string, services Services)) {
	testutil.WithTx(dbpool, t, func(tx pgx.Tx) {
		// Initialize repositories
		userRepo := &postgres.UserRepo{DB: tx}
		refreshRepo := &postgres.RefreshTokenRepo{DB: tx}
		orderRepo := &postgres.OrderRepo{DB: tx}

		// Initialize services
		tokenManager, err := tokenmanager.New(tokenmanager.Config{SecretKey: "test-secret"}, refreshRepo)
		require.NoError(t, err, "token manager should be created without errors")

		as, err := auth.NewService(auth.Config{}, tokenManager, userRepo)
		require.NoError(t, err, "auth service starting error", err)

		os := order.NewService(orderRepo)
		us := user.NewService(auth.DefaultHasher, userRepo)

		// Initializer handlers
		authHandler := handlers.NewAuth(as)
		authMiddleware := middleware.NewAuth(as)
		orderHandler := handlers.NewOrder(os)

		// Complete all together as router
		router := handlers.NewRouter(
			authHandler,
			orderHandler,
			authMiddleware,
		)

		// Run http server with the router in transaction
		srv := httptest.NewServer(router)
		defer srv.Close()

		fn(tx, srv.URL, Services{
			AuthService:  as,
			OrderService: os,
			UserService:  us,
		})
	})
}
