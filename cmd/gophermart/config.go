package main

import (
	"errors"
	"github.com/joho/godotenv"
	"github.com/spf13/pflag"
	"os"
	"path/filepath"

	"github.com/nkiryanov/gophermart/internal/logger"
)

const (
	defaultListenAddr   = "localhost:8000"
	defaultLoggingLevel = logger.LevelInfo
	defaultAccrualAddr  = "localhost:3000"
	defaultEnvironment  = logger.EnvProduction
)

type Config struct {
	// Default logging level
	LogLevel string

	// Address on which the gophermart service will be run
	ListenAddr string

	// Accrual service address to connect to
	AccrualAddr string

	// Database to connect to
	DatabaseDSN string

	// Secret key
	// Some internal parts (like signing JWT tokens) uses symmetric encryption, so this key is used for that purpose
	SecretKey string

	// Environment
	Environment string
}

func NewConfig() *Config {
	return &Config{
		LogLevel:    defaultLoggingLevel,
		ListenAddr:  defaultListenAddr,
		AccrualAddr: defaultAccrualAddr,
		Environment: defaultEnvironment,
	}
}

// Load variable from '.env' file (should be located at working directory)
func (c *Config) LoadDotEnv(getwd func() (string, error)) error {
	wd, err := getwd()
	if err != nil {
		return err
	}

	envMap, err := godotenv.Read(filepath.Join(wd, ".env"))

	switch {
	case err == nil:
		c.LoadEnv(func(key string) string {
			return envMap[key]
		})
		return nil
	case errors.Is(err, os.ErrNotExist):
		return nil
	default:
		return err
	}
}

func (c *Config) LoadEnv(getenv func(string) string) {
	// Set option to value if it not empty
	setString := func(o *string) func(value string) {
		return func(value string) {
			if value != "" {
				*o = value
			}
		}
	}

	envMap := map[string]func(string){
		"RUN_ADDRESS":            setString(&c.ListenAddr),
		"DATABASE_URI":           setString(&c.DatabaseDSN),
		"SECRET_KEY":             setString(&c.SecretKey),
		"LOG_LEVEL":              setString(&c.LogLevel),
		"ACCRUAL_SYSTEM_ADDRESS": setString(&c.AccrualAddr),
		"ENVIRONMENT":            setString(&c.Environment),
	}

	for key, parseFn := range envMap {
		parseFn(getenv(key))
	}
}

func (c *Config) ParseFlags(args []string) error {
	fs := pflag.NewFlagSet("gophermart", pflag.ContinueOnError)

	fs.StringVarP(&c.ListenAddr, "address", "a", c.ListenAddr, "Server listen address")
	fs.StringVarP(&c.DatabaseDSN, "database", "d", c.DatabaseDSN, "Database connection string")
	fs.StringVarP(&c.SecretKey, "secret-key", "s", c.SecretKey, "Secret key")
	fs.StringVarP(&c.LogLevel, "log-level", "l", c.LogLevel, "Logging level (debug, info, warn, error)")
	fs.StringVarP(&c.AccrualAddr, "accrual", "r", c.AccrualAddr, "Accrual service address")
	fs.StringVarP(&c.Environment, "environment", "e", c.Environment, "Environment (dev, prod)")

	return fs.Parse(args)
}
