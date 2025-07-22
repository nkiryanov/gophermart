package postgres

import (
	"context"
	"fmt"

	"github.com/nkiryanov/gophermart/internal/repository"
)

type Storage struct {
	db DBTX
}

func NewStorage(db DBTX) repository.Storage {
	return &Storage{db: db}
}

func (s *Storage) User() repository.UserRepo {
	return &UserRepo{DB: s.db}
}

func (s *Storage) Refresh() repository.RefreshTokenRepo {
	return &RefreshTokenRepo{DB: s.db}
}

func (s *Storage) Order() repository.OrderRepo {
	return &OrderRepo{DB: s.db}
}

func (s *Storage) Balance() repository.BalanceRepo {
	return &BalanceRepo{DB: s.db}
}

func (s *Storage) InTx(ctx context.Context, fn func(repository.Storage) error) (err error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db tx error: %w", err)
	}

	defer func() {
		switch err {
		case nil:
			err = tx.Commit(ctx)
		default:
			_ = tx.Rollback(ctx)
		}
	}()

	err = fn(NewStorage(tx))

	return err
}
