package postgres

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/testutil"
)

func Test_UserRepo(t *testing.T) {
	t.Parallel() // It's ok to run in parallel with other tests, but not with subtests

	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	t.Run("create user ok", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			r := UserRepo{DB: tx}

			user, err := r.CreateUser(t.Context(), "testuser", "hashedpassword123")

			require.NoError(t, err)
			assert.Equal(t, "testuser", user.Username)
			assert.Equal(t, "hashedpassword123", user.HashedPassword)
			assert.WithinDuration(t, time.Now(), user.CreatedAt, time.Second, "CreatedAt should be recent")
		})
	})

	t.Run("get user by id ok", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			r := UserRepo{DB: tx}
			// Create user first
			created, err := r.CreateUser(t.Context(), "findbyid", "hashedpassword123")
			require.NoError(t, err)

			// Get user by ID
			got, err := r.GetUserByID(t.Context(), created.ID)

			require.NoError(t, err)
			assert.Equal(t, created.ID, got.ID)
			assert.Equal(t, created.Username, got.Username)
			assert.Equal(t, created.HashedPassword, got.HashedPassword)
			assert.Equal(t, created.CreatedAt, got.CreatedAt)
		})
	})

	t.Run("get user by id not found", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			r := UserRepo{DB: tx}
			// Try to get non-existent user
			_, err := r.GetUserByID(t.Context(), uuid.New())

			assert.Error(t, err, "Should return error for non-existent user")
			assert.ErrorIs(t, err, apperrors.ErrUserNotFound, "should return well known error")
		})
	})

	t.Run("get user by username ok", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			r := UserRepo{DB: tx}
			// Create user first
			created, err := r.CreateUser(t.Context(), "findbyusername", "hashedpassword123")
			require.NoError(t, err)

			// Get user by username
			got, err := r.GetUserByUsername(t.Context(), created.Username)

			require.NoError(t, err)
			assert.Equal(t, created.ID, got.ID)
			assert.Equal(t, created.Username, got.Username)
			assert.Equal(t, created.HashedPassword, got.HashedPassword)
			assert.Equal(t, created.CreatedAt, got.CreatedAt)
		})
	})

	t.Run("get user by username not found", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			r := UserRepo{DB: tx}

			// Try to get non-existent user
			_, err := r.GetUserByUsername(t.Context(), "nonexistentuser")

			assert.Error(t, err, "Should return error for non-existent user")
		})
	})
}
