// Package dbtest contains supporting code for running tests that hit the DB.
package dbtest

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"runtime/debug"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // Postgres stdlib driver, used for migrations.
	"github.com/rschio/rinha/internal/data/dbschema"
	db "github.com/rschio/rinha/internal/data/dbsql/pgx"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type dbContainer struct {
	Container  *postgres.PostgresContainer
	ConnString string
}

func (c *dbContainer) DumpLogs() string {
	logs, err := c.Container.Logs(context.Background())
	if err != nil {
		return fmt.Sprintf("failed to dump comtainer logs: %v", err)
	}
	b, err := io.ReadAll(logs)
	if err != nil {
		return fmt.Sprintf("failed to read comtainer logs: %v", err)
	}
	return string(b)
}

func (c *dbContainer) shutdown() error {
	return c.Container.Terminate(context.Background())
}

func startDB() (*dbContainer, error) {
	ctx := context.Background()

	c, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(20*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("run container err: %w", err)
	}

	connStr, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("connString err: %w", err)
	}

	return &dbContainer{
		Container:  c,
		ConnString: connStr,
	}, nil
}

// NewUnit creates a test database inside a Docker container. It gives options
// to migrate and seed the database. It returns the database to use as well as
// a function to call at the end of the test.
func NewUnit(t *testing.T, options ...Option) (*slog.Logger, *pgxpool.Pool, func()) {
	t.Helper()

	c, err := startDB()
	if err != nil {
		t.Fatalf("starting DB container: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var buf bytes.Buffer
	logHandler := slog.NewTextHandler(&buf, nil)
	log := slog.New(logHandler)

	database, err := db.OpenConnString(ctx, c.ConnString)
	if err != nil {
		t.Fatalf("Opening database connection: %v", err)
	}

	for _, option := range options {
		if err := option(ctx, t, database, c); err != nil {
			t.Logf("Logs for %s\n%s:", c.Container.GetContainerID(), c.DumpLogs())
			t.Fatal(err)

		}
	}

	t.Log("Ready for testing...")

	// teardown is the function that should be invoked when the caller is done
	// with the database.
	teardown := func() {
		if r := recover(); r != nil {
			t.Log(r)
			t.Error(string(debug.Stack()))
		}

		t.Helper()
		database.Close()
		c.shutdown()

		fmt.Println("******************** LOGS ********************")
		fmt.Print(buf.String())
		fmt.Println("******************** LOGS ********************")
	}

	return log, database, teardown
}

type Option func(context.Context, *testing.T, *pgxpool.Pool, *dbContainer) error

func WithMigrations() Option {
	return func(ctx context.Context, t *testing.T, _ *pgxpool.Pool, c *dbContainer) error {
		t.Log("Migrating database...")

		db, err := sql.Open("pgx", c.ConnString)
		if err != nil {
			return fmt.Errorf("failed to open DB for migration: %w", err)
		}
		defer db.Close()

		if err := dbschema.Migrate(db); err != nil {
			return fmt.Errorf("migrating error: %w", err)
		}

		return nil
	}
}
