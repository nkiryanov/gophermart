package testutil

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/nkiryanov/gophermart/internal/db"
)

// Return random free port on 127.0.0.1 address
func RandomPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		return 0, err
	}
	defer ln.Close() // nolint:errcheck

	addr := ln.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

type PostgresContainer struct {
	Pool      *pgxpool.Pool
	Terminate func()
}

// Start container with postgres
// Stop if error happened, so you may be sure container started ok
// Should be stopped when tests stopped
func StartPostgresContainer(t *testing.T) PostgresContainer {
	t.Helper()

	// Fail if docker rootless not found
	cmd := exec.Command("docker", "info", "--format", "{{.ServerVersion}}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("test failed: docker rootless not available or not running. Err:%s", out)
	}

	// Run postgres in docker on random port
	port, err := RandomPort()
	require.NoError(t, err, "Error happened when acquiring random port to start postgres")

	container, err := postgres.Run(t.Context(),
		"postgres:17-alpine",
		postgres.WithDatabase("gophermart-test"),
		postgres.WithUsername("gophermart"),
		postgres.WithPassword("pwd"),
		postgres.BasicWaitStrategies(),
		testcontainers.CustomizeRequestOption(func(req *testcontainers.GenericContainerRequest) error {
			req.ExposedPorts = []string{fmt.Sprintf("%d:5432", port)}
			return nil
		}),
	)
	require.NoError(t, err, "Error happened when starting container with postgres, deal with it please")

	dsn, err := container.ConnectionString(t.Context())
	require.NoError(t, err, "Error happened when getting connection string from container with postgres")
	t.Logf("Container with pg started, DSN=%v", dsn)

	// Migrate and request connection pool
	dbpool, err := db.ConnectAndMigrate(t.Context(), dsn)
	require.NoError(t, err, "Error happened when connecting to postgres and migrating schema")

	return PostgresContainer{
		Pool: dbpool,
		Terminate: func() {
			dbpool.Close()
			testcontainers.CleanupContainer(t, container)
		},
	}
}

type dbtx interface {
	Begin(context.Context) (pgx.Tx, error)
}

// Create db transaction and rollback at test end
// So you may be sure db remains unchanged when test stops
func WithTx(dbtx dbtx, t *testing.T, testFunc func(tx pgx.Tx)) {
	tx, err := dbtx.Begin(t.Context())
	require.NoError(t, err)

	defer func() {
		err := tx.Rollback(t.Context())
		require.NoError(t, err)
	}()

	testFunc(tx)
}
