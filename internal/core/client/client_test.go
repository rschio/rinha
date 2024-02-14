package client_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/rschio/rinha/internal/core/client"
	"github.com/rschio/rinha/internal/core/client/store/clientdb"
	"github.com/rschio/rinha/internal/data/dbtest"
)

func TestAddTransaction(t *testing.T) {
	ctx := context.Background()
	log, database, teardown := dbtest.NewUnit(t, dbtest.WithMigrations())
	t.Cleanup(teardown)

	core := client.NewCore(clientdb.NewStore(log, database))

	clientID := 2
	c, err := core.QueryByID(ctx, clientID)
	if err != nil {
		t.Fatalf("failed to query clientID[%d]: %v", clientID, err)
	}

	nt := client.NewTransaction{
		Value:       100,
		Type:        "d",
		Description: "hello",
	}

	cret, err := core.AddTransaction(ctx, clientID, nt)
	if err != nil {
		t.Fatalf("adding transaction: %v", err)
	}

	c, err = core.QueryByID(ctx, clientID)
	if err != nil {
		t.Fatalf("failed to query 2nd time clientID[%d]: %v", clientID, err)
	}

	if diff := cmp.Diff(cret, c); diff != "" {
		t.Fatalf("got diferent clients: %s", diff)
	}

	if c.Balance != -100 {
		t.Fatalf("got %d balance want %d", c.Balance, -100)
	}

}
