package repository

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
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
			params := CreateUserParams{
				Username:     "testuser",
				PasswordHash: "hashedpassword123",
			}

			user, err := r.CreateUser(context.Background(), params)

			require.NoError(t, err)
			assert.Greater(t, user.ID, int64(0), "ID should be generated")
			assert.Equal(t, params.Username, user.Username)
			assert.Equal(t, params.PasswordHash, user.PasswordHash)
			assert.WithinDuration(t, time.Now(), user.CreatedAt, time.Second, "CreatedAt should be recent")
		})
	})

	t.Run("create user duplicate username fails", func(t *testing.T) {
		withTx(dbpool, t, func(r *UserRepo) {
			params := CreateUserParams{
				Username:     "duplicateuser",
				PasswordHash: "hashedpassword123",
			}

			// Create first user
			_, err := r.CreateUser(t.Context(), params)
			require.NoError(t, err)

			// Try to create second user with same username
			_, err = r.CreateUser(t.Context(), params)
			assert.Error(t, err, "Should fail on duplicate username")
		})
	})

	t.Run("get user by id ok", func(t *testing.T) {
		withTx(dbpool, t, func(r *UserRepo) {
			// Create user first
			params := CreateUserParams{
				Username:     "findbyid",
				PasswordHash: "hashedpassword123",
			}
			created, err := r.CreateUser(t.Context(), params)
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
		})
	})

	t.Run("get user by username ok", func(t *testing.T) {
		withTx(dbpool, t, func(r *UserRepo) {
			// Create user first
			params := CreateUserParams{
				Username:     "findbyusername",
				PasswordHash: "hashedpassword123",
			}
			created, err := r.CreateUser(t.Context(), params)
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
