package tokenmanager

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

	withTx := func(dbpool *pgxpool.Pool, t *testing.T, accessTTL time.Duration, refreshTTL time.Duration, fn func(m *TokenManager)) {
		testutil.InTx(dbpool, t, func(tx pgx.Tx) {
			cfg := Config{
				SecretKey:  "test-secret-key",
				AccessTTL:  accessTTL,
				RefreshTTL: refreshTTL,
			}
			storage := postgres.NewStorage(tx)

			tokenManager, err := New(cfg, storage)
			require.NoError(t, err, "token manager should be created without errors")

			fn(tokenManager)
		})
	}

	t.Run("new defaults", func(t *testing.T) {
		m, err := New(Config{SecretKey: "secret"}, nil)
		require.NoError(t, err, "token manager should be created without errors")

		require.Equal(t, "secret", m.key, "secret key should be set")
		require.Equal(t, defaultAccessTokenTTL, m.accessTTL, "default access token TTL should be set")
		require.Equal(t, defaultRefreshTokenTTL, m.refreshTTL, "default refresh token TTL")
		require.Equal(t, defaultSigningMethod, m.alg.Alg(), "default signing method should be set")
	})

	t.Run("GeneratePair", func(t *testing.T) {
		t.Run("return token pair", func(t *testing.T) {
			withTx(pg.Pool, t, 15*time.Minute, 24*time.Hour,
				func(tokenManager *TokenManager) {
					pair, err := tokenManager.GeneratePair(t.Context(), testUser)

					require.NoError(t, err)

					assert.NotEmpty(t, pair.Access.Value, "access token should not be empty")
					assert.WithinDuration(t, time.Now().Add(15*time.Minute), pair.Access.ExpiresAt, time.Second)
					assert.NotEmpty(t, pair.Refresh.Value, "refresh token should not be empty")
					assert.WithinDuration(t, time.Now().Add(24*time.Hour), pair.Refresh.ExpiresAt, time.Second)
				},
			)
		})

		t.Run("access claims", func(t *testing.T) {
			withTx(pg.Pool, t, 15*time.Minute, 24*time.Hour,
				func(tokenManager *TokenManager) {
					pair, err := tokenManager.GeneratePair(t.Context(), testUser)
					require.NoError(t, err)

					// Parse and verify the access token
					token, err := jwt.ParseWithClaims(pair.Access.Value, &AccessTokenClaims{}, func(token *jwt.Token) (any, error) {
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

					assert.WithinDuration(t, pair.Access.ExpiresAt, claims.ExpiresAt.Time, 0, "access expires at should match token pair")
				},
			)
		})

		t.Run("generate different tokens", func(t *testing.T) {
			withTx(pg.Pool, t, 15*time.Minute, 24*time.Hour,
				func(tokenManager *TokenManager) {
					pair1, err := tokenManager.GeneratePair(t.Context(), testUser)
					require.NoError(t, err)

					pair2, err := tokenManager.GeneratePair(t.Context(), testUser)
					require.NoError(t, err)

					assert.NotEqual(t, pair1.Refresh.Value, pair2.Refresh.Value, "refresh tokens should be different")
					assert.NotEqual(t, pair1.Access.Value, pair2.Access.Value, "access tokens should be different")
				},
			)
		})
	})

	t.Run("UseRefresh", func(t *testing.T) {
		t.Run("use token once", func(t *testing.T) {
			withTx(pg.Pool, t, 15*time.Minute, 24*time.Hour,
				func(tokenManager *TokenManager) {
					pair, err := tokenManager.GeneratePair(t.Context(), testUser)
					require.NoError(t, err)

					token, err := tokenManager.UseRefresh(t.Context(), pair.Refresh.Value)
					require.NoError(t, err, "using refresh token should not return an error")

					require.Equal(t, testUser.ID, token.UserID)
					require.WithinDuration(t, pair.Refresh.ExpiresAt, token.ExpiresAt, 1, "refresh token expiration should match expected value")
				},
			)
		})

		t.Run("use token twice", func(t *testing.T) {
			withTx(pg.Pool, t, 15*time.Minute, 24*time.Hour,
				func(tokenManager *TokenManager) {
					pair, err := tokenManager.GeneratePair(t.Context(), testUser)
					require.NoError(t, err)

					// Use the token once
					_, err = tokenManager.UseRefresh(t.Context(), pair.Refresh.Value)
					require.NoError(t, err, "using refresh token should not return an error")

					// Try to use the same token again
					_, err = tokenManager.UseRefresh(t.Context(), pair.Refresh.Value)
					require.Error(t, err, "using the same refresh token again should return an error")
				},
			)
		})

		t.Run("use expired token", func(t *testing.T) {
			withTx(pg.Pool, t, 1*time.Second, 1*time.Second,
				func(tokenManager *TokenManager) {
					pair, err := tokenManager.GeneratePair(t.Context(), testUser)
					require.NoError(t, err)

					// Wait for the token to expire
					time.Sleep(time.Second)

					// Verify refresh token exists in database
					_, err = tokenManager.UseRefresh(t.Context(), pair.Refresh.Value)
					require.Error(t, err, "using expired refresh token should return an error")
				},
			)
		})
	})

	t.Run("ParseAccess", func(t *testing.T) {
		t.Run("valid token", func(t *testing.T) {
			withTx(pg.Pool, t, 15*time.Minute, 24*time.Hour,
				func(tokenManager *TokenManager) {
					pair, err := tokenManager.GeneratePair(t.Context(), testUser)
					require.NoError(t, err, "token pair should be generated without errors")

					userID, err := tokenManager.ParseAccess(t.Context(), pair.Access.Value)
					require.NoError(t, err, "valid token should be parsed without errors")
					require.Equal(t, testUser.ID, userID)
				},
			)
		})

		t.Run("not a token", func(t *testing.T) {
			withTx(pg.Pool, t, 15*time.Minute, 24*time.Hour,
				func(tokenManager *TokenManager) {
					// Parse the valid token
					_, err := tokenManager.ParseAccess(t.Context(), "invalid token")
					require.Error(t, err, "parsing even not a token should return an error")
				},
			)
		})

		t.Run("expired token", func(t *testing.T) {
			withTx(pg.Pool, t, 1*time.Second, 1*time.Second,
				func(tokenManager *TokenManager) {
					pair, err := tokenManager.GeneratePair(t.Context(), testUser)
					require.NoError(t, err)

					// Wait for the token to expire
					time.Sleep(time.Second)

					_, err = tokenManager.ParseAccess(t.Context(), pair.Access.Value)
					require.Error(t, err, "token has to become expired")
				},
			)
		})

		t.Run("not signed token", func(t *testing.T) {
			withTx(pg.Pool, t, 15*time.Minute, 24*time.Hour,
				func(tokenManager *TokenManager) {
					// Create valid but unsigned token
					token := jwt.NewWithClaims(
						jwt.SigningMethodNone,
						AccessTokenClaims{
							RegisteredClaims: jwt.RegisteredClaims{
								ID:        uuid.NewString(),
								IssuedAt:  jwt.NewNumericDate(time.Now()),
								ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
							},
							UserID: testUser.ID,
						},
					)
					access, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
					require.NoError(t, err)

					_, err = tokenManager.ParseAccess(t.Context(), access)
					require.Error(t, err, "Valid token with empty alg must fail")
				},
			)
		})
	})
}
