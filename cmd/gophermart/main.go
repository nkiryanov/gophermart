package main

import (
	"context"
	"log/slog"

	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx := context.Background()

	srv, err := NewServerApp(ctx)
	if err != nil {
		slog.Error("can't initialize app, sorry", "error", err.Error())
		os.Exit(1)
	}

	// Initialize context that cancelled on SIGTERM
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop
		slog.Warn("Interrupt signal")
		cancel()
	}()

	// Run server
	if err := srv.Run(ctx); err != http.ErrServerClosed {
		slog.Error("HTTP server error", "error", err.Error())
	}
}
