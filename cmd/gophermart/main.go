package main

import (
	"context"
	"fmt"

	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/nkiryanov/gophermart/internal/logger"
)

func main() {
	ctx := context.Background()
	log := logger.NewDefault()

	err := run(ctx, os.Getenv, os.Getwd, os.Args[1:])
	if err != nil {
		log.Error("Application error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string, getwd func() (string, error), args []string) error {
	// Load configuration from environment variables and flags
	config := NewConfig()
	err := config.LoadDotEnv(getwd)
	if err != nil {
		return fmt.Errorf("error while loading .env file: %w", err)
	}
	config.LoadEnv(getenv)
	err = config.ParseFlags(args)
	if err != nil {
		return fmt.Errorf("error while parsing flags: %w", err)
	}

	// Initialize context that cancelled on SIGTERM
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize server application
	srv, err := NewServerApp(ctx, config)
	if err != nil {
		return fmt.Errorf("error while initializing app: %w", err)
	}

	// Run server
	err = srv.Run(ctx)
	if err != http.ErrServerClosed {
		return fmt.Errorf("server stopped with error: %w", err)
	}

	return nil
}
