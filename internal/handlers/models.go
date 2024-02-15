package handlers

import (
	"time"

	"github.com/rschio/rinha/internal/core/client"
)

type TransactionsReq struct {
	Value       int    `json:"valor"`
	Type        string `json:"tipo"`
	Description string `json:"descricao"`
}

type TransactionsResp struct {
	Limit   int `json:"limite"`
	Balance int `json:"saldo"`
}

type Balance struct {
	Total int       `json:"total"`
	Limit int       `json:"limite"`
	Date  time.Time `json:"data_extrato"`
}

type BillingResp struct {
	Balance          Balance       `json:"saldo"`
	LastTransactions []Transaction `json:"ultimas_transacoes"`
}

type Transaction struct {
	Value       int       `json:"valor"`
	Type        string    `json:"tipo"`
	Description string    `json:"descricao"`
	Date        time.Time `json:"realizada_em"`
}

func toBillingResp(b client.Billing) BillingResp {
	return BillingResp{
		Balance: Balance{
			Total: b.Balance,
			Limit: b.Limit,
			Date:  b.Date,
		},
		LastTransactions: toTransactions(b.LastTransactions),
	}
}

func toTransactions(ts []client.Transaction) []Transaction {
	slice := make([]Transaction, len(ts))
	for i, t := range ts {
		slice[i] = toTransaction(t)
	}
	return slice
}

func toTransaction(t client.Transaction) Transaction {
	return Transaction{
		Value:       t.Value,
		Type:        t.Type,
		Description: t.Description,
		Date:        t.Date,
	}
}
