package client_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
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

func TestConsistency(t *testing.T) {
	ctx := context.Background()
	log, database, teardown := dbtest.NewUnit(t, dbtest.WithMigrations())
	t.Cleanup(teardown)

	core := client.NewCore(clientdb.NewStore(log, database))

	n := 50
	nts := make([]testNT, n)
	for i := 0; i < n; i++ {
		nts[i] = randomNewTransaction()
	}

	for _, tt := range nts {
		t.Run(fmt.Sprint(tt), func(t *testing.T) {
			t.Parallel()

			out := make(chan billingErr)
			go func() {
				b, err := core.Billing(ctx, tt.clientID)
				out <- billingErr{b, err}
			}()

			c, err := core.AddTransaction(ctx, tt.clientID, tt.nt)
			if err != nil {
				if !errors.Is(err, client.ErrTransactionDenied) {
					t.Fatalf("transaction err: %v", err)
				}
			}

			if c.Balance < -c.Limit {
				t.Errorf("insconsistency found on AddTransaction: %+v", c)
			}

			ret := <-out
			if ret.err != nil {
				t.Fatalf("billing error: %v", err)
			}
			if ret.billing.Balance < -ret.billing.Limit {
				t.Errorf("insconsistency found on Billing: %+v", ret.billing)
			}
		})
	}
}

type billingErr struct {
	billing client.Billing
	err     error
}

type testNT struct {
	clientID int
	nt       client.NewTransaction
}

func randomNewTransaction() testNT {
	return testNT{
		clientID: rand.N(5) + 1,
		nt: client.NewTransaction{
			Value:       rand.N(5000) * 100,
			Type:        []string{"c", "d"}[rand.N(2)],
			Description: "some",
		},
	}
}
