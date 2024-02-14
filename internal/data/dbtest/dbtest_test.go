package dbtest

import (
	"context"
	"testing"

	db "github.com/rschio/rinha/internal/data/dbsql/pgx"
)

func TestNewUnit(t *testing.T) {
	ctx := context.Background()
	log, database, teardown := NewUnit(t, WithMigrations())
	t.Cleanup(teardown)
	log.Info("Hello")

	if err := db.StatusCheck(ctx, database); err != nil {
		t.Fatal(err)
	}
}
