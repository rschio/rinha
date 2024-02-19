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

	// QueryByID returns information about a client.
	QueryByID(ctx context.Context, clientID int) (Client, error)

	// QueryTransactions returns the most recent client's transactions.
	QueryTransactions(ctx context.Context, clientID, pageNumber, rowsPerPage int) ([]Transaction, error)

	// AddTransaction add a transaction associated with a client.
	AddTransaction(ctx context.Context, t Transaction) error

	UpdateClientBalance(ctx context.Context, clientID, balance int) (Client, error)
}

// Core deals with client's business logic.
type Core struct {
	store Store
}

func NewCore(s Store) *Core {
	return &Core{store: s}
}

func (c *Core) QueryByID(ctx context.Context, clientID int) (Client, error) {
	return c.store.QueryByID(ctx, clientID)
}

// Billing returns info about a client and the 10 most recent transactions of
// this client.
func (c *Core) Billing(ctx context.Context, clientID int) (Billing, error) {
	var b Billing
	fn := func(tx Store) error {
		ctx, span := web.AddSpan(ctx, "internal.core.client.Core.Billing.Tx.Inside")
		defer span.End()

		c, err := tx.QueryByID(ctx, clientID)
		if err != nil {
			return err
		}

		page := 1
		rows := 10
		transactions, err := tx.QueryTransactions(ctx, clientID, page, rows)
		if err != nil {
			return err
		}

		b.Balance = c.Balance
		b.Limit = c.Limit
		//b.Date = web.GetTime(ctx)
		b.Date = time.Now().UTC().Round(time.Microsecond)
		b.LastTransactions = transactions

		return nil
	}

	ctx, span := web.AddSpan(ctx, "internal.core.client.Core.Billing.Tx")
	defer span.End()

	if err := c.store.ExecUnderTx(ctx, fn); err != nil {
		return Billing{}, err
	}

	return b, nil
}

func (c *Core) AddTransaction(ctx context.Context, clientID int, nt NewTransaction) (Client, error) {
	t := Transaction{
		ID:          uuid.New(),
		ClientID:    clientID,
		Value:       nt.Value,
		Type:        nt.Type,
		Description: nt.Description,
		//		Date:        web.GetTime(ctx).Round(time.Microsecond),
	}
	if err := t.validate(); err != nil {
		return Client{}, err
	}

	var client Client
	fn := func(tx Store) error {
		ctx, span := web.AddSpan(ctx, "internal.core.client.Core.AddTransaction.Tx.Inside")
		defer span.End()

		// Set time only when inside the transaction.
		// This is necessary to ensure the Date is set when the transaction
		// is processed, not when it was received.
		t.Date = time.Now().UTC().Round(time.Microsecond)

		var err error
		client, err = tx.QueryByID(ctx, clientID)
		if err != nil {
			return err
		}

		value := t.Value
		if t.Type == "d" {
			value = -value
		}

		newBalance := client.Balance + value
		if newBalance < -client.Limit {
			return ErrTransactionDenied
		}

		if err := tx.AddTransaction(ctx, t); err != nil {
			return fmt.Errorf("failed to add transaction: %w", err)
		}

		client, err = tx.UpdateClientBalance(ctx, client.ID, newBalance)
		if err != nil {
			return fmt.Errorf("failed to update balance: %w", err)
		}

		return nil
	}

	ctx, span := web.AddSpan(ctx, "internal.core.client.Core.AddTransaction.Tx")
	defer span.End()

	if err := c.store.ExecUnderTx(ctx, fn); err != nil {
		return Client{}, err
	}

	return client, nil
}

func (t Transaction) validate() error {
	switch {
	case t.ID.Variant() == uuid.Invalid:
		return ErrInternal
	case t.ClientID < 1:
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
