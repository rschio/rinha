package client

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/google/uuid"
	goredislib "github.com/redis/go-redis/v9"
	"github.com/rschio/rinha/internal/web"
	"go.opentelemetry.io/otel/trace"
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
}

// Core deals with client's business logic.
type Core struct {
	store Store
	rs    *redsync.Redsync
}

func NewCore(s Store, r *goredislib.Client) *Core {
	redisPool := goredis.NewPool(r)
	rs := redsync.New(redisPool)

	return &Core{store: s, rs: rs}
}

func (c *Core) QueryByID(ctx context.Context, clientID int) (Client, error) {
	if !isValidID(clientID) {
		return Client{}, ErrNotFound
	}

	return c.store.QueryByID(ctx, clientID)
}

// Billing returns info about a client and the 10 most recent transactions of
// this client.
func (c *Core) Billing(ctx context.Context, clientID int) (Billing, error) {
	if !isValidID(clientID) {
		return Billing{}, ErrNotFound
	}

	var b Billing
	fn := func(tx Store) error {
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
		b.Date = web.GetTime(ctx)
		b.LastTransactions = transactions

		return nil
	}

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
		Date:        web.GetTime(ctx).Round(time.Microsecond),
	}
	if err := t.validate(); err != nil {
		return Client{}, err
	}

	var span trace.Span
	ctx, span = web.AddSpan(ctx, "internal.core.Client.AddTransactions.WaitingMutex")

	mu := c.rs.NewMutex(strconv.Itoa(clientID))
	mu.Lock()
	defer mu.Unlock()

	span.End()

	client, err := c.store.QueryByID(ctx, clientID)
	if err != nil {
		return Client{}, err
	}

	value := t.Value
	if t.Type == "d" {
		value = -value
	}

	newBalance := client.Balance + value
	if newBalance < -client.Limit {
		return Client{}, ErrTransactionDenied
	}

	if err := c.store.AddTransaction(ctx, t); err != nil {
		return Client{}, fmt.Errorf("failed to add transaction: %w", err)
	}

	client.Balance = newBalance

	return client, nil
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
