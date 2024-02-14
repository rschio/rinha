package clientdb

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rschio/rinha/internal/core/client"
	"github.com/rschio/rinha/internal/data/dbtest"
)

func TestQueryByID(t *testing.T) {
	ctx := context.Background()
	log, database, teardown := dbtest.NewUnit(t, dbtest.WithMigrations())
	t.Cleanup(teardown)

	store := NewStore(log, database)

	c, err := store.QueryByID(ctx, 1)
	if err != nil {
		t.Fatalf("failed to query client by id[%d]: %v", 1, err)
	}

	if c.ID != 1 {
		t.Errorf("wrong id, got %d want %v", c.ID, 1)
	}
	if c.Limit != 100000 {
		t.Errorf("wrong limit, got %d want %v", c.Limit, 100000)
	}
	if c.Balance != 0 {
		t.Errorf("wrong balance, got %d want %v", c.Balance, 0)
	}
}

func TestQueryTransactions(t *testing.T) {
	ctx := context.Background()
	log, database, teardown := dbtest.NewUnit(t, dbtest.WithMigrations())
	t.Cleanup(teardown)

	store := NewStore(log, database)

	clientID := 3
	for range 25 {
		if err := store.AddTransaction(ctx, genTransaction(clientID)); err != nil {
			t.Fatalf("failed to add transaction: %v", err)
		}
	}

	ts, err := store.QueryTransactions(ctx, clientID, 1, 10)
	if err != nil {
		t.Fatalf("failed to query transactions: %v", err)
	}
	if len(ts) != 10 {
		t.Fatalf("got %d transactions, want %d", len(ts), 10)
	}
	if ts[0].Value != 750 {
		t.Errorf("wrong value got %d want %d", ts[0].Value, 750)
	}
	if ts[0].Type != "d" {
		t.Errorf("wrong type got %q want %q", ts[0].Type, "d")
	}

	clientID = 1
	ts, err = store.QueryTransactions(ctx, clientID, 1, 10)
	if err != nil {
		t.Fatalf("failed to query transactions: %v", err)
	}
	if len(ts) != 0 {
		t.Errorf("got %d should return 0 transactions", len(ts))
	}
}

func genTransaction(clientID int) client.Transaction {
	return client.Transaction{
		ID:          uuid.New(),
		ClientID:    clientID,
		Value:       750,
		Type:        "d",
		Description: "desc",
		Date:        time.Now(),
	}
}
