package clientdb

import (
	"context"
	"testing"

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
