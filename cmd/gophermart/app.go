package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/nkiryanov/gophermart/internal/db"
	"github.com/nkiryanov/gophermart/internal/handlers"
	"github.com/nkiryanov/gophermart/internal/handlers/middleware"
	"github.com/nkiryanov/gophermart/internal/logger"
	"github.com/nkiryanov/gophermart/internal/repository/postgres"
	"github.com/nkiryanov/gophermart/internal/service/auth"
	"github.com/nkiryanov/gophermart/internal/service/auth/tokenmanager"
	"github.com/nkiryanov/gophermart/internal/service/order"
	"github.com/nkiryanov/gophermart/internal/service/user"
)

type ServerApp struct {
	ListenAddr string
	Handler    http.Handler
}

func NewServerApp(ctx context.Context, c *Config) (*ServerApp, error) {
	// Initialize logger
	logger, err := logger.New(c.Environment, c.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("error while initializing logger: %w", err)
	}

	// Connect to the database and run migrations
	pool, err := db.ConnectAndMigrate(ctx, c.DatabaseDSN)
	if err != nil {
		return nil, fmt.Errorf("error while connecting to db. Err: %w", err)
	}

	// Initialize repositories
	storage := postgres.NewStorage(pool)

	// Initialize services
	tokenManager, err := tokenmanager.New(tokenmanager.Config{SecretKey: c.SecretKey}, storage)
	if err != nil {
		return nil, fmt.Errorf("error while creating token manager")
	}
	userService := user.NewService(user.DefaultHasher, storage)
	orderService := order.NewService(storage)
	authService, err := auth.NewService(auth.Config{}, tokenManager, userService)
	if err != nil {
		return nil, fmt.Errorf("error while creating auth service. Err: %w", err)
	}

	// Initialize auth handler
	authHandler := handlers.NewAuth(authService)
	orderHandler := handlers.NewOrder(orderService)
	balanceHandler := handlers.NewBalance(userService)
	authMiddleware := middleware.AuthMiddleware(authService)

	mux := handlers.NewRouter(
		authHandler,
		orderHandler,
		balanceHandler,
		authMiddleware,
		middleware.LoggerMiddleware(logger),
	)

	return &ServerApp{
		ListenAddr: c.ListenAddr,
		Handler:    mux,
	}, nil
}

// Run starts http server and closes gracefully on context cancellation
func (s *ServerApp) Run(ctx context.Context) error {
	httpServer := &http.Server{
		Addr:    s.ListenAddr,
		Handler: s.Handler,
	}

	idleConnsClosed := make(chan struct{})
	srvCtx, srvCtxCancel := context.WithCancel(ctx)
	defer srvCtxCancel()

	go func() {
		<-srvCtx.Done()

		timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(timeoutCtx); err == context.DeadlineExceeded {
			// Consider to user logger dependency
			slog.Error("HTTP server shutdown timeout exceeded, forcing shutdown...")
		}
		// Consider to user logger dependency
		slog.Info("HTTP server stopped")
		close(idleConnsClosed)
	}()

	// Listen and serve until context is cancelled; then close gracefully connections
	slog.Info("Starting server")
	err := httpServer.ListenAndServe()
	srvCtxCancel()
	<-idleConnsClosed

	return err
}
