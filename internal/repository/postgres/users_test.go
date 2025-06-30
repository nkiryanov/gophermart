package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/nkiryanov/gophermart/internal/domain"
)

func Test_UserRepo(t *testing.T) {
	container, err := postgres.Run(context.Background(),
		"postgres:17-alpine",
		postgres.WithDatabase("gophermart-test"),
		postgres.WithUsername("gophermart"),
		postgres.WithPassword("pwd"),
		postgres.BasicWaitStrategies(),
		testcontainers.CustomizeRequestOption(func(req *testcontainers.GenericContainerRequest) error {
			req.ExposedPorts = []string{"25432:5432"}
			return nil
		}),
	)
	defer testcontainers.CleanupContainer(t, container)
	require.NoError(t, err, "container with pg start failed")

	dsn, err := container.ConnectionString(t.Context())
	require.NoError(t, err)
	t.Logf("Container with pg started, DSN=%v", dsn)

	// Migrate and request connection pool
	dbpool, err := ConnectAndMigrate(t.Context(), dsn)
	require.NoError(t, err)
	defer dbpool.Close()

	// Helper to run tests with its own UserRepo in transaction
	// When test end rollback
	withTx := func(dbpool *pgxpool.Pool, t *testing.T, testFunc func(*UserRepo)) {
		tx, err := dbpool.Begin(t.Context())
		require.NoError(t, err)

		defer func() {
			err := tx.Rollback(context.Background())
			require.NoError(t, err)
		}()

		userRepo := &UserRepo{db: tx}
		testFunc(userRepo)
	}

	t.Run("create user ok", func(t *testing.T) {
		withTx(dbpool, t, func(r *UserRepo) {
			user, err := r.CreateUser(context.Background(), "testuser", "hashedpassword123")

			require.NoError(t, err)
			assert.Greater(t, user.ID, int64(0), "ID should be generated")
			assert.Equal(t, "testuser", user.Username)
			assert.Equal(t, "hashedpassword123", user.PasswordHash)
			assert.WithinDuration(t, time.Now(), user.CreatedAt, time.Second, "CreatedAt should be recent")
		})
	})

	t.Run("create user duplicate username fails", func(t *testing.T) {
		withTx(dbpool, t, func(r *UserRepo) {
			// Create first user
			_, err := r.CreateUser(t.Context(), "duplicateuser", "hashedpassword123")
			require.NoError(t, err)

			// Try to create second user with same username
			_, err = r.CreateUser(t.Context(), "duplicateuser", "anotherhashedpassword")
			assert.Error(t, err, "Should fail on duplicate username")
			assert.ErrorIs(t, err, domain.ErrUserAlreadyExists, "if user exists must return well defined error")
		})
	})

	t.Run("get user by id ok", func(t *testing.T) {
		withTx(dbpool, t, func(r *UserRepo) {
			// Create user first
			created, err := r.CreateUser(t.Context(), "findbyid", "hashedpassword123")
			require.NoError(t, err)

			// Get user by ID
			got, err := r.GetUserByID(t.Context(), created.ID)

			require.NoError(t, err)
			assert.Equal(t, created.ID, got.ID)
			assert.Equal(t, created.Username, got.Username)
			assert.Equal(t, created.PasswordHash, got.PasswordHash)
			assert.Equal(t, created.CreatedAt, got.CreatedAt)
		})
	})

	t.Run("get user by id not found", func(t *testing.T) {
		withTx(dbpool, t, func(r *UserRepo) {
			// Try to get non-existent user
			_, err := r.GetUserByID(t.Context(), 99999)

			assert.Error(t, err, "Should return error for non-existent user")
			assert.ErrorIs(t, err, domain.ErrUserNotFound, "should return well known error")
		})
	})

	t.Run("get user by username ok", func(t *testing.T) {
		withTx(dbpool, t, func(r *UserRepo) {
			// Create user first
			created, err := r.CreateUser(t.Context(), "findbyusername", "hashedpassword123")
			require.NoError(t, err)

			// Get user by username
			got, err := r.GetUserByUsername(t.Context(), created.Username)

			require.NoError(t, err)
			assert.Equal(t, created.ID, got.ID)
			assert.Equal(t, created.Username, got.Username)
			assert.Equal(t, created.PasswordHash, got.PasswordHash)
			assert.Equal(t, created.CreatedAt, got.CreatedAt)
		})
	})

	t.Run("get user by username not found", func(t *testing.T) {
		withTx(dbpool, t, func(r *UserRepo) {
			// Try to get non-existent user
			_, err := r.GetUserByUsername(t.Context(), "nonexistentuser")

			assert.Error(t, err, "Should return error for non-existent user")
		})
	})
}
