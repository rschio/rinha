// Package dbschema contains the database schema and migrations data.
package dbschema

import (
	"database/sql"
	_ "embed" // Used to embed sql files.
	"fmt"

	"github.com/ardanlabs/darwin/v3"
	"github.com/ardanlabs/darwin/v3/dialects/postgres"
	"github.com/ardanlabs/darwin/v3/drivers/generic"
)

var (
	//go:embed sql/migrations.sql
	migrations string
)

func Migrate(db *sql.DB) error {
	driver, err := generic.New(db, postgres.Dialect{})
	if err != nil {
		return err
	}

	d := darwin.New(driver, darwin.ParseMigrations(migrations))
	if err := d.Migrate(); err != nil {
		return fmt.Errorf("failed to migrate: %w", err)
	}

	return nil
}
