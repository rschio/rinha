package client

import (
	"time"

	"github.com/google/uuid"
)

type Client struct {
	ID      int
	Limit   int
	Balance int
}

type NewTransaction struct {
	Value       int
	Type        string
	Description string
}

type Transaction struct {
	ID          uuid.UUID
	ClientID    int
	Value       int
	Type        string
	Description string
	Date        time.Time
}

type Billing struct {
	Balance          int
	Limit            int
	Date             time.Time
	LastTransactions []Transaction
}
