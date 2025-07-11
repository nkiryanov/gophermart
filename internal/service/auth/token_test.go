package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/repository/postgres"
	"github.com/nkiryanov/gophermart/internal/testutil"
)

func mustParseTime(value string) time.Time {
	dt, err := time.Parse("2006-01-02 15:04:05Z07:00", value)
	if err != nil {
		panic(err)
	}
	return dt
}

func Test_TokenManager(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgresContainer(t)
	t.Cleanup(pg.Terminate)

	testUser := models.User{
		ID:             uuid.New(),
		CreatedAt:      mustParseTime("2024-01-01 19:00:01Z"),
		Username:       "testuser",
		HashedPassword: "hashed_password",
	}

	t.Run("generate pair ok", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			refreshRepo := postgres.RefreshTokenRepo{DB: tx}
			tokenManager := TokenManager{
				key:         "test-secret-key",
				alg:         "HS256",
				accessTTL:   15 * time.Minute,
				refreshTTL:  24 * time.Hour,
				refreshRepo: &refreshRepo,
			}

			pair, err := tokenManager.GeneratePair(t.Context(), testUser)

			require.NoError(t, err)
			assert.NotEmpty(t, pair.Access, "access token should not be empty")
			assert.NotEmpty(t, pair.Refresh, "refresh token should not be empty")
		})
	})

	t.Run("access token has correct claims", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			refreshRepo := postgres.RefreshTokenRepo{DB: tx}
			tokenManager := TokenManager{
				key:         "test-secret-key",
				alg:         "HS256",
				accessTTL:   15 * time.Minute,
				refreshTTL:  24 * time.Hour,
				refreshRepo: &refreshRepo,
			}

			pair, err := tokenManager.GeneratePair(t.Context(), testUser)
			require.NoError(t, err)

			// Parse and verify the access token
			token, err := jwt.ParseWithClaims(pair.Access, &AccessTokenClaims{}, func(token *jwt.Token) (any, error) {
				return []byte("test-secret-key"), nil
			})
			require.NoError(t, err)
			require.True(t, token.Valid, "access token should be valid")

			claims, ok := token.Claims.(*AccessTokenClaims)
			require.True(t, ok, "claims should be of type AccessTokenClaims")
			assert.Equal(t, testUser.ID, claims.UserID, "user ID in token should match")
			assert.NotEmpty(t, claims.ID, "token has to has jti")
			assert.WithinDuration(t, time.Now(), claims.IssuedAt.Time, time.Second, "issued at should be close to now")
			assert.WithinDuration(t, time.Now().Add(15*time.Minute), claims.ExpiresAt.Time, time.Second, "expires at should be 15 minutes from now")
		})
	})

	t.Run("refresh token stored in database", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			refreshRepo := postgres.RefreshTokenRepo{DB: tx}
			tokenManager := TokenManager{
				key:         "test-secret-key",
				alg:         "HS256",
				accessTTL:   15 * time.Minute,
				refreshTTL:  24 * time.Hour,
				refreshRepo: &refreshRepo,
			}

			pair, err := tokenManager.GeneratePair(t.Context(), testUser)
			require.NoError(t, err)

			// Verify refresh token exists in database
			storedToken, err := refreshRepo.GetToken(t.Context(), pair.Refresh)
			require.NoError(t, err)
			assert.Equal(t, pair.Refresh, storedToken.Token, "stored token should match generated token")
			assert.Equal(t, testUser.ID, storedToken.UserID, "stored token should have correct user ID")
			assert.WithinDuration(t, time.Now(), storedToken.CreatedAt, time.Second, "created at should be close to now")
			assert.WithinDuration(t, time.Now().Add(24*time.Hour), storedToken.ExpiresAt, time.Second, "expires at should be 24 hours from now")
			assert.True(t, storedToken.UsedAt.IsZero(), "token should not be marked as used initially")
		})
	})

	t.Run("several tokens different", func(t *testing.T) {
		testutil.WithTx(pg.Pool, t, func(tx pgx.Tx) {
			refreshRepo := postgres.RefreshTokenRepo{DB: tx}
			tokenManager := TokenManager{
				key:         "test-secret-key",
				alg:         "HS256",
				accessTTL:   15 * time.Minute,
				refreshTTL:  24 * time.Hour,
				refreshRepo: &refreshRepo,
			}

			pair1, err := tokenManager.GeneratePair(t.Context(), testUser)
			require.NoError(t, err)

			pair2, err := tokenManager.GeneratePair(t.Context(), testUser)
			require.NoError(t, err)

			assert.NotEqual(t, pair1.Refresh, pair2.Refresh, "refresh tokens should be different")
			assert.NotEqual(t, pair1.Access, pair2.Access, "access tokens should be different")
		})
	})
}
