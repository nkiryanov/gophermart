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
		Token:     "secret-token",
		UserID:    uuid.New(),
		CreatedAt: mustParseTime("2024-01-01 19:00:01Z"),
		ExpiresAt: mustParseTime("2200-01-01 03:00:02Z"),
	}

	t.Run("create token ok", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			repo := RefreshTokenRepo{DB: tx}

			tokenID, err := repo.Create(t.Context(), token)

			require.NoError(t, err)
			assert.Greater(t, tokenID, int64(0))
		})
	})

	t.Run("get token ok", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			repo := RefreshTokenRepo{DB: tx}
			_, err := repo.Create(t.Context(), token)
			require.NoError(t, err)

			got, err := repo.GetToken(t.Context(), token.Token)

			require.NoError(t, err)
			require.Equal(t, token.Token, got.Token)
			require.Equal(t, token.UserID, got.UserID)
			require.WithinDuration(t, token.CreatedAt, got.CreatedAt, 0)
			require.WithinDuration(t, token.ExpiresAt, got.ExpiresAt, 0)
			require.WithinDuration(t, token.UsedAt, got.UsedAt, 0)
		})
	})

	t.Run("mark token used", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			repo := RefreshTokenRepo{DB: tx}
			_, err := repo.Create(t.Context(), token)
			require.NoError(t, err)

			usedAt, err := repo.MarkUsed(t.Context(), token.Token)

			require.NoError(t, err, "No error must be happen when marking used existed token")
			require.WithinDuration(t, time.Now(), usedAt, 100*time.Millisecond, "should marked as used close to now() enough")
		})
	})

	t.Run("mark used not existed token", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			repo := RefreshTokenRepo{DB: tx}

			_, err := repo.MarkUsed(t.Context(), token.Token)

			require.Error(t, err)
			assert.ErrorIs(t, err, apperrors.ErrRefreshTokenNotFound)
		})
	})

	t.Run("mark used already used token", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			repo := RefreshTokenRepo{DB: tx}
			_, err := repo.Create(t.Context(), token)
			require.NoError(t, err)

			usedAtFirst, err := repo.MarkUsed(t.Context(), token.Token)
			require.NoError(t, err, "No error should happen on make used")

			time.Sleep(100 * time.Millisecond)
			usedAtSecond, err := repo.MarkUsed(t.Context(), token.Token)
			require.NoError(t, err, "No error must happen when trying to mark used existed but used token")

			assert.Equal(t, usedAtFirst, usedAtSecond, "should return same time for already used token")
		})
	})

	t.Run("get valid token ok", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			repo := RefreshTokenRepo{DB: tx}
			_, err := repo.Create(t.Context(), token)
			require.NoError(t, err)

			// get token from beginning of time
			got, err := repo.GetValidToken(t.Context(), token.Token, time.Time{})

			require.NoError(t, err)
			require.Equal(t, token.Token, got.Token)
			require.Equal(t, token.UserID, got.UserID)
			require.WithinDuration(t, token.CreatedAt, got.CreatedAt, 0)
			require.WithinDuration(t, token.ExpiresAt, got.ExpiresAt, 0)
			require.WithinDuration(t, token.UsedAt, got.UsedAt, 0)
		})
	})

	t.Run("fail get valid token if expired", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			repo := RefreshTokenRepo{DB: tx}
			_, err := repo.Create(t.Context(), token)
			require.NoError(t, err)

			// Try to get valid token 1 second after it expire time
			_, err = repo.GetValidToken(t.Context(), token.Token, mustParseTime("2200-01-01 03:00:03Z"))

			require.Error(t, err, "when token expired must return error")
			require.ErrorIs(t, err, apperrors.ErrRefreshTokenExpired)
		})
	})

	t.Run("fail get valid token if used", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			repo := RefreshTokenRepo{DB: tx}
			_, err := repo.Create(t.Context(), token)
			require.NoError(t, err)

			// Mark token expired
			_, err = repo.MarkUsed(t.Context(), token.Token)
			require.NoError(t, err)

			// Get token from the beginning of time
			_, err = repo.GetValidToken(t.Context(), token.Token, time.Time{})

			require.Error(t, err)
			require.ErrorIs(t, err, apperrors.ErrRefreshTokenIsUsed)
		})
	})
}
