package clientdb

import (
	"context"
	"errors"
	"log/slog"

	"github.com/rschio/rinha/internal/core/client"
	db "github.com/rschio/rinha/internal/data/dbsql/pgx"
)

type Store struct {
	log *slog.Logger
	db  db.DB
}

func NewStore(log *slog.Logger, database db.DB) *Store {
	return &Store{
		log: log,
		db:  database,
	}
}

func (s *Store) ExecUnderTx(ctx context.Context, fn func(txStore client.Store) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := fn(NewStore(s.log, tx)); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Store) QueryByID(ctx context.Context, clientID int) (client.Client, error) {
	data := struct {
		ID int `db:"id"`
	}{
		ID: clientID,
	}

	const q = `
	SELECT
		c.id,
		c.credit_limit,
		COALESCE(-SUM(t.value), 0) as balance
	FROM
		clients AS c
		LEFT JOIN transactions AS t ON c.id = t.client_id
	WHERE
		c.id = 1
	GROUP BY
		c.id`

	c, err := db.NamedQueryStruct[dbClient](ctx, s.log, s.db, q, data)
	if err != nil {
		if errors.Is(err, db.ErrDBNotFound) {
			return client.Client{}, client.ErrNotFound
		}
		return client.Client{}, err
	}

	return toClient(c), nil
}

func (s *Store) AddTransaction(ctx context.Context, t client.Transaction) error {
	return nil
}

// ----------------------------------------------------------------------
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
