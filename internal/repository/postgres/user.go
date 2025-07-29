package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/models"
)

type UserRepo struct {
	DB DBTX
}

func (r *UserRepo) CreateUser(ctx context.Context, username string, hashedPassword string) (models.User, error) {
	const createUser = `
	INSERT INTO users (username, password_hash)
	VALUES ($1, $2)
	RETURNING id, created_at, username, password_hash
	`

	rows, _ := r.DB.Query(ctx, createUser, username, hashedPassword)
	user, err := pgx.CollectOneRow(rows, rowToUser)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return user, apperrors.ErrUserAlreadyExists
		}

		return user, fmt.Errorf("db error: %w", err)
	}

	return user, nil
}

func (r *UserRepo) GetUserByID(ctx context.Context, id uuid.UUID) (models.User, error) {
	const getUserByID = `
	SELECT * FROM users
	WHERE id = $1
	`

	rows, _ := r.DB.Query(ctx, getUserByID, id)
	user, err := pgx.CollectOneRow(rows, rowToUser)

	switch {
	case err == nil:
		return user, nil
	case errors.Is(err, pgx.ErrNoRows):
		return user, apperrors.ErrUserNotFound
	default:
		return user, fmt.Errorf("db error: %w", err)
	}
}

func (r *UserRepo) GetUserByUsername(ctx context.Context, username string) (models.User, error) {
	const getUserByUsername = `
	SELECT * FROM users
	WHERE username = $1
	`
	rows, _ := r.DB.Query(ctx, getUserByUsername, username)
	user, err := pgx.CollectOneRow(rows, rowToUser)

	switch {
	case err == nil:
		return user, nil
	case errors.Is(err, pgx.ErrNoRows):
		return user, apperrors.ErrUserNotFound
	default:
		return user, fmt.Errorf("db error: %w", err)
	}
}

func rowToUser(row pgx.CollectableRow) (models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.CreatedAt, &u.Username, &u.HashedPassword)
	return u, err
}
