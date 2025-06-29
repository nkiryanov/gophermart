package repository

import (
	"context"
	"embed"
	"errors"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Run embedded migrations
// Check the example at https://github.com/golang-migrate/migrate/blob/v4.18.1/source/iofs/example_test.go
// dsn: database source name in format postgres://...
func Migrate(dsn string) error {
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		return err
	}

	migrator, err := migrate.NewWithSourceInstance(
		"iofs",
		source,
		strings.Replace(dsn, "postgres://", "pgx5://", 1), // golang-migrate expects dsn in format 'pgx5://...' only, make it happy with 'postgres://...'
	)
	if err != nil {
		return err
	}

	err = migrator.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}

func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}

	return pool, err
}

func ConnectAndMigrate(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	err := Migrate(dsn)
	if err != nil {
		return nil, err
	}

	return Connect(ctx, dsn)
}
