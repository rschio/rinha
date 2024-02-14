// Package db provides support to access a PostgreSQL database.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rschio/rinha/internal/logger"
	"github.com/rschio/rinha/internal/web"
	"go.opentelemetry.io/otel/attribute"
)

const (
	uniqueViolation = pgerrcode.UniqueViolation
	undefinedTable  = pgerrcode.UndefinedTable
)

// Set of error variables for CRUD operations.
var (
	ErrDBNotFound        = sql.ErrNoRows
	ErrDBDuplicatedEntry = errors.New("duplicated entry")
	ErrUndefinedTable    = errors.New("undefined table")
)

// Config is the required properties to use the database.
type Config struct {
	User       string
	Password   string
	Host       string
	Name       string
	Schema     string
	DisableTLS bool
}

// ConnString creates a postgres connection string with config values.
func ConnString(cfg Config) string {
	sslMode := "require"
	if cfg.DisableTLS {
		sslMode = "disable"
	}

	q := make(url.Values)
	q.Set("sslmode", sslMode)
	q.Set("timezone", "utc")
	if cfg.Schema != "" {
		q.Set("search_path", cfg.Schema)
	}

	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     cfg.Host,
		Path:     cfg.Name,
		RawQuery: q.Encode(),
	}

	return u.String()
}

// Open knows how to open a database connection based on the configuration.
func Open(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	return OpenConnString(ctx, ConnString(cfg))
}

// OpenConnString open a database connection using the connString.
func OpenConnString(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	pgCfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, err
	}
	return pgxpool.NewWithConfig(ctx, pgCfg)
}

// StatusCheck returns nil if it can successfully talk to the database. It
// returns a non-nil error otherwise.
func StatusCheck(ctx context.Context, db *pgxpool.Pool) error {
	var pingError error
	for attempts := 1; ; attempts++ {
		pingError = db.Ping(ctx)
		if pingError == nil {
			break
		}
		time.Sleep(time.Duration(attempts) * 100 * time.Millisecond)
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Run a simple query to determine connectivity.
	// Running this query forces a round trip through the database.
	const q = `SELECT true`
	var tmp bool
	return db.QueryRow(ctx, q).Scan(&tmp)
}

// DB is an interface used to support both *pgxpool.Pool and pgx.Tx.
type DB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Exec is a helper function to execute a CUD operation with
// logging and tracing.
func Exec(ctx context.Context, log *slog.Logger, db DB, query string) error {
	return namedExec(ctx, log, db, query, struct{}{})
}

// NamedExec is a helper function to execute a CUD operation with
// logging and tracing where field replacement is necessary.
func NamedExec(ctx context.Context, log *slog.Logger, db DB, query string, data any) error {
	return namedExec(ctx, log, db, query, data)
}

func namedExec(ctx context.Context, log *slog.Logger, db DB, query string, data any) error {
	ctx, span := web.AddSpan(ctx, "internal.data.dbsql.pgx.namedExec")
	defer span.End()

	args, err := toNamedArgs(data)
	if err != nil {
		return fmt.Errorf("failed to parse arguments: %w", err)
	}

	q := queryString(query, args)
	logger.InfocCtx(ctx, log, 4, "db.namedExec", "query", q)
	span.SetAttributes(attribute.String("query", q))

	if _, err := db.Exec(ctx, query, args); err != nil {
		var pgerr *pgconn.PgError
		if errors.As(err, &pgerr) {
			switch pgerr.Code {
			case undefinedTable:
				return ErrUndefinedTable
			case uniqueViolation:
				return ErrDBDuplicatedEntry
			}
		}

		return err
	}

	return nil
}

// NamedQuerySlice is a helper function for executing queries that return a
// collection of data to be unmarshalled into a slice where field replacement is
// necessary.
func NamedQuerySlice[T any](ctx context.Context, log *slog.Logger, db DB, query string, data any) ([]T, error) {
	ctx, span := web.AddSpan(ctx, "internal.data.dbsql.pgx.NamedQuerySlice")
	defer span.End()

	args, err := toNamedArgs(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse arguments: %w", err)
	}

	q := queryString(query, args)
	logger.InfocCtx(ctx, log, 3, "db.NamedQuerySlice", "query", q)
	span.SetAttributes(attribute.String("query", q))

	rows, err := db.Query(ctx, query, args)
	if err != nil {
		var pgerr *pgconn.PgError
		if errors.As(err, &pgerr) && pgerr.Code == undefinedTable {
			return nil, ErrUndefinedTable
		}
		return nil, err
	}
	defer rows.Close()

	out, err := pgx.CollectRows(rows, pgx.RowToStructByName[T])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDBNotFound
		}
		return nil, err
	}

	return out, nil
}

func NamedQueryStruct[T any](ctx context.Context, log *slog.Logger, db DB, query string, data any) (T, error) {
	ctx, span := web.AddSpan(ctx, "internal.data.dbsql.pgx.NamedQueryStruct")
	defer span.End()

	args, err := toNamedArgs(data)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("failed to parse arguments: %w", err)
	}

	q := queryString(query, args)
	logger.InfocCtx(ctx, log, 3, "db.NamedQueryStruct", "query", q)
	span.SetAttributes(attribute.String("query", q))

	rows, err := db.Query(ctx, query, args)
	if err != nil {
		var zero T
		var pgerr *pgconn.PgError
		if errors.As(err, &pgerr) && pgerr.Code == undefinedTable {
			return zero, ErrUndefinedTable
		}
		return zero, err
	}
	defer rows.Close()

	out, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[T])
	if err != nil {
		var zero T
		if errors.Is(err, pgx.ErrNoRows) {
			return zero, ErrDBNotFound
		}
		return zero, err
	}

	return out, nil
}

func toNamedArgs(value any) (pgx.NamedArgs, error) {
	s := reflect.ValueOf(value)
	if s.Kind() == reflect.Ptr {
		s = s.Elem()
	}
	if s.Kind() != reflect.Struct {
		return nil, fmt.Errorf("invalid struct")
	}
	typ := s.Type()

	args := make(pgx.NamedArgs)

	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		structField := typ.Field(i)
		fieldTag := structField.Tag.Get("db")

		if !structField.IsExported() || fieldTag == "-" {
			continue
		}
		if fieldTag == "" {
			fieldTag = structField.Name
		}

		args[fieldTag] = f.Interface()
	}

	return args, nil
}

var reDBQueryArg = regexp.MustCompile(`@\w+`)

func queryString(query string, args map[string]any) string {
	query = reDBQueryArg.ReplaceAllStringFunc(query, func(s string) string {
		// skip '@'.
		key := s[1:]
		val, ok := args[key]
		if !ok {
			return s
		}
		switch v := val.(type) {
		case []byte, string:
			return fmt.Sprintf("'%s'", v)
		default:
			return fmt.Sprintf("%v", v)
		}
	})
	query = strings.ReplaceAll(query, "\t", "")
	query = strings.ReplaceAll(query, "\n", " ")
	return strings.TrimSpace(query)
}
