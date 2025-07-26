package user

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nkiryanov/gophermart/internal/apperrors"
	"github.com/nkiryanov/gophermart/internal/models"
	"github.com/nkiryanov/gophermart/internal/repository"
	"github.com/shopspring/decimal"
)

var (
	DefaultHasher = BcryptHasher{}
)

// Interface to create or compare user password hashes
type PasswordHasher interface {
	// Generate Hash from password
	Hash(password string) (string, error)

	// Compare known hashedPassword and user provided password
	// Must be protected against timing attacks
	Compare(hashedPassword string, password string) error
}

type UserService struct {
	hasher  PasswordHasher
	storage repository.Storage
}

func NewService(hasher PasswordHasher, storage repository.Storage) *UserService {
	if hasher == nil {
		hasher = DefaultHasher
	}

	return &UserService{
		hasher:  hasher,
		storage: storage,
	}
}

func (s *UserService) CreateUser(ctx context.Context, username string, password string) (models.User, error) {
	var user models.User
	if password == "" {
		return user, fmt.Errorf("password can't be empty")
	}

	hash, err := s.hasher.Hash(password)
	if err != nil {
		return user, fmt.Errorf("can't use this as password, Err: %w", err)
	}

	err = s.storage.InTx(ctx, func(storage repository.Storage) error {
		user, err = s.storage.User().CreateUser(ctx, username, hash)
		if err != nil {
			return fmt.Errorf("can't create user. Err: %w", err)
		}

		err = s.storage.Balance().CreateBalance(ctx, user.ID)
		if err != nil {
			return fmt.Errorf("can't create user balance. Err: %w", err)
		}

		return nil
	})
	if err != nil {
		return user, err
	}

	return user, nil
}

func (s *UserService) Login(ctx context.Context, username string, password string) (models.User, error) {
	// Ignore error for now, but prefer to log it
	// It's safe to use user now, because it's always not empty
	user, _ := s.storage.User().GetUserByUsername(ctx, username)

	// Always compare password to prevent timing attacks
	// It will always fail if user not found
	err := s.hasher.Compare(user.HashedPassword, password)
	if err != nil {
		return user, apperrors.ErrUserNotFound
	}

	return user, nil
}

func (s *UserService) GetUserByID(ctx context.Context, userID uuid.UUID) (models.User, error) {
	return s.storage.User().GetUserByID(ctx, userID)
}

func (s *UserService) GetBalance(ctx context.Context, userID uuid.UUID) (models.Balance, error) {
	return s.storage.Balance().GetBalance(ctx, userID, false)
}

func (s *UserService) GetWithdrawals(ctx context.Context, userID uuid.UUID) ([]models.Transaction, error) {
	return s.storage.Balance().ListTransactions(ctx, userID, true)
}

// Withdraw from user balance in transaction
func (s *UserService) Withdraw(ctx context.Context, userID uuid.UUID, orderNum string, amount decimal.Decimal) (models.Balance, error) {
	var balance models.Balance

	err := s.storage.InTx(ctx, func(storage repository.Storage) error {
		existedBalance, err := s.storage.Balance().GetBalance(ctx, userID, true)
		if err != nil {
			return err
		}

		if existedBalance.Current.LessThan(amount) {
			return apperrors.ErrBalanceInsufficient
		}

		t, err := s.storage.Balance().CreateTransaction(ctx, models.Transaction{
			ID:          uuid.New(),
			ProcessedAt: time.Now(),
			UserID:      userID,
			OrderNumber: orderNum,
			Type:        models.TransactionTypeWithdrawn,
			Amount:      amount,
		})
		if err != nil {
			return err
		}

		balance, err = s.storage.Balance().UpdateBalance(ctx, userID, t.Amount.Neg())
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return balance, fmt.Errorf("can't withdraw. Err: %w", err)
	}

	return balance, nil
}
