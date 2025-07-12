package postgres

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/testutil"
)

func mustParseTime(value string) time.Time {
	dt, err := time.Parse("2006-01-02 15:04:05Z07:00", value)
	if err != nil {
		panic(err)
	}
	return dt
}

func Test_RefreshTokenRepo(t *testing.T) {
	t.Parallel() // It's ok to run in parallel with other tests, but not with subtests

	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)
	token := models.RefreshToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Token:     "secret-token",
		CreatedAt: mustParseTime("2024-01-01 19:00:01Z"),
		ExpiresAt: mustParseTime("2200-01-01 03:00:02Z"),
		UsedAt:    nil,
	}

	t.Run("create token ok", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			repo := RefreshTokenRepo{DB: tx}

			got, err := repo.Save(t.Context(), token)

			require.NoError(t, err)
			require.Equal(t, token.ID, got.ID)
			require.Equal(t, token.UserID, got.UserID)
			require.Equal(t, token.Token, got.Token)
			require.WithinDuration(t, token.CreatedAt, got.CreatedAt, time.Microsecond)
			require.WithinDuration(t, token.ExpiresAt, got.ExpiresAt, time.Microsecond)
			require.Nil(t, got.UsedAt, "UsedAt should be nil cause original token has UsedAt as nil")
		})
	})

	t.Run("get token ok", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			repo := RefreshTokenRepo{DB: tx}
			_, err := repo.Save(t.Context(), token)
			require.NoError(t, err)

			got, err := repo.Get(t.Context(), token.Token)

			require.NoError(t, err)
			require.Equal(t, token.Token, got.Token)
			require.Equal(t, token.UserID, got.UserID)
			require.WithinDuration(t, token.CreatedAt, got.CreatedAt, 0)
			require.WithinDuration(t, token.ExpiresAt, got.ExpiresAt, 0)
			require.Nil(t, token.UsedAt)
		})
	})

	t.Run("mark token used", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			repo := RefreshTokenRepo{DB: tx}
			_, err := repo.Save(t.Context(), token)
			require.NoError(t, err)

			got, err := repo.GetAndMarkUsed(t.Context(), token.Token)

			require.NoError(t, err, "No error must be happen when marking used existed token")
			require.NotNil(t, got.UsedAt, "token must marked used")
			require.WithinDuration(t, time.Now(), *got.UsedAt, 50*time.Millisecond, "should marked as used close to now() enough")
			require.Equal(t, token.Token, got.Token)
			require.Equal(t, token.UserID, got.UserID)
			require.WithinDuration(t, token.CreatedAt, got.CreatedAt, 0)
			require.WithinDuration(t, token.ExpiresAt, got.ExpiresAt, 0)
		})
	})

	t.Run("mark used not existed token", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			repo := RefreshTokenRepo{DB: tx}

			_, err := repo.GetAndMarkUsed(t.Context(), token.Token)

			require.Error(t, err)
			assert.ErrorIs(t, err, apperrors.ErrRefreshTokenNotFound)
		})
	})

	t.Run("mark used is idempotent", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			repo := RefreshTokenRepo{DB: tx}
			_, err := repo.Save(t.Context(), token)
			require.NoError(t, err)

			tokenFirst, err := repo.GetAndMarkUsed(t.Context(), token.Token)
			require.NoError(t, err, "No error should happen on make used")

			time.Sleep(100 * time.Millisecond)
			tokenSecond, err := repo.GetAndMarkUsed(t.Context(), token.Token)
			require.Error(t, err, "Mark used already used token has to return error")
			require.ErrorIs(t, err, apperrors.ErrRefreshTokenIsUsed, "should return ErrRefreshTokenIsUsed error")

			assert.WithinDuration(t, *tokenFirst.UsedAt, *tokenSecond.UsedAt, 0, "should return same time for already used token")
		})
	})
}
