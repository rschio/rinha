package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rschio/rinha/internal/web"
)

// Set of errors for client API.
var (
	ErrNotFound          = errors.New("client not found")
	ErrInvalidArgument   = errors.New("client invalid argument")
	ErrInternal          = errors.New("client internal error")
	ErrTransactionDenied = errors.New("client transaction denied")
)

// Store is used to persist client's data.
type Store interface {
	// ExecUnderTx executes the fn function under a transaction. If fn returns
	// an error the transaction is rolled back and the error is returned.
	ExecUnderTx(ctx context.Context, fn func(tx Store) error) error

	QueryByID(ctx context.Context, clientID int) (Client, error)
	AddTransaction(ctx context.Context, t Transaction) error
}

// Core deals with client's business logic.
type Core struct {
	store Store
}

func (c *Core) AddTransaction(ctx context.Context, clientID int, nt NewTransaction) error {
	t := Transaction{
		ID:          uuid.New(),
		ClientID:    clientID,
		Value:       nt.Value,
		Type:        nt.Type,
		Description: nt.Description,
		Date:        web.GetTime(ctx).Round(time.Microsecond),
	}
	if err := t.validate(); err != nil {
		return err
	}

	fn := func(tx Store) error {
		client, err := tx.QueryByID(ctx, clientID)
		if err != nil {
			return err
		}

		// TODO: Debito pode usar o limite ou apenas credito?
		newBalance := client.Balance - t.Value
		if newBalance < -client.Limit {
			return ErrTransactionDenied
		}

		if err := tx.AddTransaction(ctx, t); err != nil {
			return fmt.Errorf("failed to add transaction: %w", err)
		}

		return nil
	}

	return c.store.ExecUnderTx(ctx, fn)
}

func (t Transaction) validate() error {
	switch {
	case t.ID.Variant() == uuid.Invalid:
		return ErrInternal
	case !isValidID(t.ClientID):
		return ErrNotFound
	case t.Value < 0:
		return ErrInvalidArgument
	case t.Type != "c" && t.Type != "d":
		return ErrInvalidArgument
	case len(t.Description) < 1 || len(t.Description) > 10:
		return ErrInvalidArgument
	}

	return nil
}

func isValidID(id int) bool {
	if id < 1 || id > 5 {
		return false
	}
	return true
}
