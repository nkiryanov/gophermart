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
	db DBTX
}

const createUser = `-- name: CreateUser
INSERT INTO users (id, username, password_hash)
VALUES ($1, $2, $3)
RETURNING id, created_at, username, password_hash
`

func (r *UserRepo) CreateUser(ctx context.Context, username string, hashedPassword string) (models.User, error) {
	rows, _ := r.db.Query(ctx, createUser, uuid.New(), username, hashedPassword)
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

const getUserByID = `-- name: getUserByID
SELECT * FROM users
WHERE id = $1
`

func (r *UserRepo) GetUserByID(ctx context.Context, id uuid.UUID) (models.User, error) {
	rows, _ := r.db.Query(ctx, getUserByID, id)
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

const getUserByUsername = `-- name: getUserByUsername
SELECT * FROM users
WHERE username = $1
`

func (r *UserRepo) GetUserByUsername(ctx context.Context, username string) (models.User, error) {
	rows, _ := r.db.Query(ctx, getUserByUsername, username)
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
