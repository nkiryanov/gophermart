package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/nkiryanov/gophermart/internal/models"
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
	return pgx.CollectOneRow(rows, rowToUser)
}

const getUserByID = `-- name: getUserByID
SELECT * FROM users
WHERE id = $1
`

func (r *UserRepo) GetUserByID(ctx context.Context, id int64) (models.User, error) {
	rows, _ := r.db.Query(ctx, getUserByID, id)
	return pgx.CollectOneRow(rows, rowToUser)
}

const getUserByUsername = `-- name: getUserByUsername
SELECT * FROM users
WHERE username = $1
`

func (r *UserRepo) GetUserByUsername(ctx context.Context, username string) (models.User, error) {
	rows, _ := r.db.Query(ctx, getUserByUsername, username)
	return pgx.CollectOneRow(rows, rowToUser)
}

func rowToUser(row pgx.CollectableRow) (models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.CreatedAt, &u.Username, &u.PasswordHash)
	return u, err
}
