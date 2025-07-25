package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/nkiryanov/gophermart/internal/testutil"
	"github.com/stretchr/testify/require"
)

func Test_run(t *testing.T) {
	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	port, err := testutil.RandomPort()
	require.NoError(t, err, "failed to get random port to start server")
	listenAddr := fmt.Sprintf("localhost:%d", port)

	t.Run("stop with signal", func(t *testing.T) {

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond) // Half Second
		t.Cleanup(cancel)

		err = run(ctx, os.Getenv, os.Getwd, []string{
			"--address", listenAddr,
			"--log-level", "debug",
			"--accrual", "http://localhost:3000",
			"--database", pg.DSN,
			"--secret-key", "secret",
		})

		require.NoError(t, err, "on correct stop should not return error")
	})

	t.Run("stop with srv error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond) // Half Second
		t.Cleanup(cancel)

		// Try to run without secret key. Must fail
		err := run(ctx, os.Getenv, os.Getwd, []string{
			"--address", listenAddr,
			"--log-level", "debug",
			"--accrual", "http://localhost:3000",
			"--database", pg.DSN,
		})

		require.Error(t, err, "on incorrect stop should return error")
	})
}
