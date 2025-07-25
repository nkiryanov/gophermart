package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	t.Run("set default option", func(t *testing.T) {
		c := NewConfig()

		require.Equal(t, "localhost:8000", c.ListenAddr, "default listen address not set")
		require.Equal(t, "info", c.LogLevel, "default log level not set")
		require.Equal(t, "localhost:3000", c.AccrualAddr, "default accrual address not set")
		require.Equal(t, "", c.DatabaseDSN, "database DSN should be empty by default")
		require.Equal(t, "", c.SecretKey, "secret key should be empty by default")
	})

	t.Run("load env", func(t *testing.T) {
		c := NewConfig()
		getenv := func(key string) string {
			switch key {
			case "RUN_ADDRESS":
				return "localhost:9000"
			case "LOG_LEVEL":
				return "debug"
			case "ACCRUAL_SYSTEM_ADDRESS":
				return "localhost:4000"
			case "DATABASE_URI":
				return "postgres://user:pass@localhost:5432/test"
			case "SECRET_KEY":
				return "secret"
			default:
				return ""
			}
		}

		c.LoadEnv(getenv)

		require.Equal(t, "localhost:9000", c.ListenAddr)
		require.Equal(t, "debug", c.LogLevel)
		require.Equal(t, "localhost:4000", c.AccrualAddr)
		require.Equal(t, "postgres://user:pass@localhost:5432/test", c.DatabaseDSN)
		require.Equal(t, "secret", c.SecretKey)
	})

	t.Run("parse flags", func(t *testing.T) {
		t.Run("valid flags", func(t *testing.T) {
			tests := []struct {
				name  string
				flags []string
			}{
				{
					name: "short",
					flags: []string{
						"-a", "localhost:9000",
						"-l", "debug",
						"-r", "localhost:4000",
						"-d", "postgres://user:pass@localhost:5432/test",
						"-s", "secret",
					},
				},
				{
					name: "long",
					flags: []string{
						"--address", "localhost:9000",
						"--log-level", "debug",
						"--accrual", "localhost:4000",
						"--database", "postgres://user:pass@localhost:5432/test",
						"--secret", "secret",
					},
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					c := NewConfig()

					err := c.ParseFlags(tt.flags)

					require.NoError(t, err, "correct flags must pursed without error")
					require.Equal(t, "localhost:9000", c.ListenAddr)
					require.Equal(t, "debug", c.LogLevel)
					require.Equal(t, "localhost:4000", c.AccrualAddr)
					require.Equal(t, "postgres://user:pass@localhost:5432/test", c.DatabaseDSN)
					require.Equal(t, "secret", c.SecretKey)

				})
			}
		})

		t.Run("invalid flags", func(t *testing.T) {
			c := NewConfig()

			err := c.ParseFlags([]string{
				"--invalid-flag", "value",
			})

			require.Error(t, err, "invalid flag should return an error")
		})
	})
}
