package internal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type Dialect string

const (
	DialectSQLite   Dialect = "sqlite"
	DialectPostgres Dialect = "postgres"
)

type Options struct {
	DatabaseURL     string
	DatabaseSchema  string
	EnableSQLiteWAL bool
	PoolSize        int
	PoolRecycle     time.Duration
	OpenTimeout     time.Duration
}

type Handle struct {
	DB          *sql.DB
	Dialect     Dialect
	DatabaseURL string
	Schema      string
}

func Open(ctx context.Context, opts Options) (*Handle, error) {
	if strings.TrimSpace(opts.DatabaseURL) == "" {
		return nil, errors.New("database url is required")
	}

	driverName, dsn, dialect, err := resolveDriver(opts.DatabaseURL)
	if err != nil {
		return nil, err
	}

	if dialect == DialectSQLite {
		if err := os.MkdirAll(filepath.Dir(dsn), 0o755); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}

	if opts.PoolSize > 0 {
		db.SetMaxOpenConns(opts.PoolSize)
		db.SetMaxIdleConns(opts.PoolSize)
	}
	if opts.PoolRecycle > 0 {
		db.SetConnMaxLifetime(opts.PoolRecycle)
	}

	pingCtx := ctx
	if opts.OpenTimeout > 0 {
		var cancel context.CancelFunc
		pingCtx, cancel = context.WithTimeout(ctx, opts.OpenTimeout)
		defer cancel()
	}

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, err
	}

	if dialect == DialectSQLite {
		journalMode := "DELETE"
		if opts.EnableSQLiteWAL {
			journalMode = "WAL"
		}
		if _, err := db.ExecContext(ctx, "PRAGMA journal_mode="+journalMode); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	return &Handle{
		DB:          db,
		Dialect:     dialect,
		DatabaseURL: opts.DatabaseURL,
		Schema:      opts.DatabaseSchema,
	}, nil
}

func resolveDriver(databaseURL string) (string, string, Dialect, error) {
	normalized := strings.TrimSpace(databaseURL)
	if strings.HasPrefix(normalized, "postgres://") {
		normalized = "postgresql://" + strings.TrimPrefix(normalized, "postgres://")
	}

	switch {
	case strings.HasPrefix(normalized, "sqlite+sqlcipher://"):
		return "", "", "", fmt.Errorf("unsupported database url: %s", normalized)
	case strings.HasPrefix(normalized, "sqlite:///"):
		return "sqlite", strings.TrimPrefix(normalized, "sqlite:///"), DialectSQLite, nil
	case strings.HasPrefix(normalized, "sqlite://"):
		return "sqlite", strings.TrimPrefix(normalized, "sqlite://"), DialectSQLite, nil
	case strings.HasPrefix(normalized, "postgresql://"):
		return "pgx", normalized, DialectPostgres, nil
	default:
		return "", "", "", fmt.Errorf("unsupported database url: %s", normalized)
	}
}

func (h *Handle) Close() error {
	if h == nil || h.DB == nil {
		return nil
	}
	return h.DB.Close()
}

func (h *Handle) TableExists(ctx context.Context, table string) (bool, error) {
	switch h.Dialect {
	case DialectSQLite:
		var exists int
		err := h.DB.QueryRowContext(
			ctx,
			"SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ? LIMIT 1",
			table,
		).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return true, nil
	case DialectPostgres:
		schema := h.Schema
		query := "SELECT 1 FROM information_schema.tables WHERE table_name = $1 AND table_schema = current_schema() LIMIT 1"
		args := []any{table}
		if schema != "" {
			query = "SELECT 1 FROM information_schema.tables WHERE table_name = $1 AND table_schema = $2 LIMIT 1"
			args = append(args, schema)
		}
		var exists int
		err := h.DB.QueryRowContext(ctx, query, args...).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return true, nil
	default:
		return false, fmt.Errorf("unsupported dialect: %s", h.Dialect)
	}
}
