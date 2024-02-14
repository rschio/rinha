package clientdb

import (
	"time"

	"github.com/google/uuid"
	"github.com/rschio/rinha/internal/core/client"
)

type dbClient struct {
	ID      int `db:"id"`
	Limit   int `db:"credit_limit"`
	Balance int `db:"balance"`
}

func toClient(c dbClient) client.Client {
	return client.Client{
		ID:      c.ID,
		Limit:   c.Limit,
		Balance: c.Balance,
	}
}

type dbTransaction struct {
	ID          uuid.UUID `db:"id"`
	ClientID    int       `db:"client_id"`
	Value       int       `db:"value"`
	Type        string    `db:"type"`
	Description string    `db:"description"`
	Date        time.Time `db:"date_created"`
}

func toDBTransaction(t client.Transaction) dbTransaction {
	dbt := dbTransaction(t)

	// Store debit as negative values to make
	// database SUM operations easier.
	if t.Type == "d" {
		dbt.Value = -t.Value
	}

	return dbt
}

func toTransactions(ts []dbTransaction) []client.Transaction {
	slice := make([]client.Transaction, len(ts))
	for i, t := range ts {
		slice[i] = toTransaction(t)
	}
	return slice
}

func toTransaction(t dbTransaction) client.Transaction {
	ct := client.Transaction(t)

	// Client transactions are always positive.
	// The transaction type is used as signal.
	if ct.Value < 0 {
		ct.Value = -ct.Value
	}

	return ct
}
