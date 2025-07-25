package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func getTempDir(t *testing.T) (wd string, getwd func() (string, error)) {
	workDir, err := os.MkdirTemp("", "gophermart-*")
	require.NoError(t, err)

	t.Cleanup(func() {
		err := os.RemoveAll(workDir)
		require.NoError(t, err, "error while removing temporary file")
	})

	getwdFn := func() (string, error) {
		return workDir, nil
	}

	return workDir, getwdFn
}

func TestConfig(t *testing.T) {
	t.Run("set default option", func(t *testing.T) {
		c := NewConfig()

		require.Equal(t, defaultListenAddr, c.ListenAddr, "listen address should be default")
		require.Equal(t, defaultLoggingLevel, c.LogLevel, "logging level should be default")
		require.Equal(t, defaultAccrualAddr, c.AccrualAddr, "accrual address should be default")
		require.Equal(t, "", c.DatabaseDSN, "database DSN should be empty by default")
		require.Equal(t, "", c.SecretKey, "secret key should be empty by default")
	})

	t.Run("load dot env", func(t *testing.T) {
		t.Run("load correct file", func(t *testing.T) {
			workDir, getwdFn := getTempDir(t)
			fileContent := `
RUN_ADDRESS=localhost:9000
LOG_LEVEL=debug
ACCRUAL_SYSTEM_ADDRESS=localhost:4000
DATABASE_URI=postgres://user:pass@localhost:5432/test
SECRET_KEY=secret
`
			err := os.WriteFile(filepath.Join(workDir, ".env"), []byte(fileContent), 0644)
			require.NoError(t, err, "error while preparing .env file")

			c := NewConfig()
			err = c.LoadDotEnv(getwdFn)

			require.NoError(t, err, "error while loading .env file")
			require.Equal(t, "localhost:9000", c.ListenAddr)
			require.Equal(t, "debug", c.LogLevel)
			require.Equal(t, "localhost:4000", c.AccrualAddr)
			require.Equal(t, "postgres://user:pass@localhost:5432/test", c.DatabaseDSN)
			require.Equal(t, "secret", c.SecretKey)
		})

		t.Run("not fail if no file", func(t *testing.T) {
			_, getwdFn := getTempDir(t)
			c := NewConfig()

			// There is no .env file in directory
			err := c.LoadDotEnv(getwdFn)

			require.NoError(t, err, "should not fail if .env file does not exist")
			require.Equal(t, defaultListenAddr, c.ListenAddr, "should be default value")
			require.Equal(t, defaultLoggingLevel, c.LogLevel)
			require.Equal(t, defaultAccrualAddr, c.AccrualAddr)
			require.Equal(t, "", c.DatabaseDSN)
			require.Equal(t, "", c.SecretKey)
		})
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
						"--secret-key", "secret",
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
