package clientdb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/rschio/rinha/internal/core/client"
	db "github.com/rschio/rinha/internal/data/dbsql/pgx"
	"github.com/rschio/rinha/internal/web"
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
		c.balance
	FROM
		clients AS c
	WHERE
		c.id = @id
	FOR UPDATE`

	c, err := db.NamedQueryStruct[dbClient](ctx, s.log, s.db, q, data)
	if err != nil {
		if errors.Is(err, db.ErrDBNotFound) {
			return client.Client{}, client.ErrNotFound
		}
		return client.Client{}, err
	}

	return toClient(c), nil
}

func (s *Store) QueryTransactions(ctx context.Context, clientID, pageNumber, rowsPerPage int) ([]client.Transaction, error) {
	data := struct {
		ID          int `db:"id"`
		Offset      int `db:"offset"`
		RowsPerPage int `db:"rows_per_page"`
	}{
		ID:          clientID,
		Offset:      (pageNumber - 1) * rowsPerPage,
		RowsPerPage: rowsPerPage,
	}

	const q = `
	SELECT
		*
	FROM
		transactions t
	WHERE
		t.client_id = @id
	ORDER BY
		date_created DESC
	OFFSET @offset ROWS FETCH NEXT @rows_per_page ROWS ONLY`

	dbTs, err := db.NamedQuerySlice[dbTransaction](ctx, s.log, s.db, q, data)
	if err != nil {
		return nil, err
	}

	return toTransactions(dbTs), nil
}

func (s *Store) UpdateClientBalance(ctx context.Context, clientID, balance int) (client.Client, error) {
	data := struct {
		ID          int       `db:"id"`
		Balance     int       `db:"balance"`
		DateUpdated time.Time `db:"date_updated"`
	}{
		ID:          clientID,
		Balance:     balance,
		DateUpdated: web.GetTime(ctx).Round(time.Microsecond),
	}

	const q = `	
	UPDATE
		clients
	SET
		balance = @balance,
		date_updated = @date_updated
	WHERE
		id = @id
	RETURNING
		id, credit_limit, balance`

	c, err := db.NamedQueryStruct[dbClient](ctx, s.log, s.db, q, data)
	if err != nil {
		return client.Client{}, err
	}

	return toClient(c), nil
}

func (s *Store) AddTransaction(ctx context.Context, t client.Transaction) error {
	const q = `
	INSERT INTO transactions(
		id,
		client_id,
		value,
		type,
		description,
		date_created)
	VALUES (
		@id,
		@client_id,
		@value,
		@type,
		@description,
		@date_created);`

	if err := db.NamedExec(ctx, s.log, s.db, q, toDBTransaction(t)); err != nil {
		return fmt.Errorf("failed to add transaction: %w", err)
	}

	return nil
}
