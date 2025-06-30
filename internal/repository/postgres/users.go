package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/repository"
)

type UserRepo struct {
	db DBTX
}

const createUser = `-- name: CreateUser
INSERT INTO users (username, password_hash)
VALUES ($1, $2)
RETURNING id, created_at, username, password_hash
`

type CreateUserParams struct {
	Username     string
	PasswordHash string
}

func (r *UserRepo) CreateUser(ctx context.Context, arg CreateUserParams) (models.User, error) {
	rows, _ := r.db.Query(ctx, createUser, arg.Username, arg.PasswordHash)
	user, err := pgx.CollectOneRow(rows, rowToUser)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgerrcode.IsIntegrityConstraintViolation(pgErr.Code) {
			return user, repository.ErrUserAlreadyExists
		}
	}

	return user, err
}

const getUserByID = `-- name: getUserByID
SELECT * FROM users
WHERE id = $1
`

func (r *UserRepo) GetUserByID(ctx context.Context, id int64) (models.User, error) {
	rows, _ := r.db.Query(ctx, getUserByID, id)
	user, err := pgx.CollectOneRow(rows, rowToUser)

	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		return user, repository.ErrUserNotFound
	}

	return user, err
}

const getUserByUsername = `-- name: getUserByUsername
SELECT * FROM users
WHERE username = $1
`

func (r *UserRepo) GetUserByUsername(ctx context.Context, username string) (models.User, error) {
	rows, _ := r.db.Query(ctx, getUserByUsername, username)
	user, err := pgx.CollectOneRow(rows, rowToUser)

	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		return user, repository.ErrUserNotFound
	}

	return user, err
}

func rowToUser(row pgx.CollectableRow) (models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.CreatedAt, &u.Username, &u.PasswordHash)
	return u, err
}
