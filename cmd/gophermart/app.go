package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/nkiryanov/gophermart/internal/db"
	"github.com/nkiryanov/gophermart/internal/handlers"
	"github.com/nkiryanov/gophermart/internal/logger"
	"github.com/nkiryanov/gophermart/internal/repository/postgres"
	"github.com/nkiryanov/gophermart/internal/service/auth"
	"github.com/nkiryanov/gophermart/internal/service/auth/tokenmanager"
	"github.com/nkiryanov/gophermart/internal/service/order"
	"github.com/nkiryanov/gophermart/internal/service/orderprocessor"
	"github.com/nkiryanov/gophermart/internal/service/user"
)

type orderProcessor interface {
	Process(ctx context.Context) <-chan struct{}
}

type ServerApp struct {
	ListenAddr string
	Handler    http.Handler
	Logger     logger.Logger

	OrderProcessor orderProcessor
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
	userService := user.NewService(user.DefaultHasher, storage)
	orderService := order.NewService(storage)
	tokenManager, err := tokenmanager.New(tokenmanager.Config{SecretKey: c.SecretKey}, storage)
	if err != nil {
		return nil, fmt.Errorf("token manager initialization: %w", err)
	}
	authService, err := auth.NewService(auth.Config{}, tokenManager, userService)
	if err != nil {
		return nil, fmt.Errorf("auth service initialization: %w", err)
	}

	// Initialize order processor
	processor := orderprocessor.New(c.AccrualAddr, logger, orderService)

	mux := handlers.NewRouter(
		authService,
		orderService,
		userService,
		logger,
	)

	return &ServerApp{
		ListenAddr:     c.ListenAddr,
		Handler:        mux,
		Logger:         logger,
		OrderProcessor: processor,
	}, nil
}

// Run starts http server and closes gracefully on context cancellation
func (s *ServerApp) Run(ctx context.Context) error {
	httpServer := &http.Server{
		Addr:    s.ListenAddr,
		Handler: s.Handler,
	}

	idleSrvClosed := make(chan struct{})
	go func() {
		<-ctx.Done()

		timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(timeoutCtx); err == context.DeadlineExceeded {
			s.Logger.Error("HTTP server shutdown timeout exceeded, forcing shutdown...")
		}

		s.Logger.Info("HTTP server stopped")
		close(idleSrvClosed)
	}()

	idleProcessorClosed := s.OrderProcessor.Process(ctx)

	s.Logger.Info("Listening on address", "address", s.ListenAddr)
	err := httpServer.ListenAndServe()

	<-idleSrvClosed
	<-idleProcessorClosed
	return err
}
