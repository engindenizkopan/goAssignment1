package postgres

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func Connect(ctx context.Context, dsn string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pgxpool: %w", err)
	}
	return &DB{Pool: pool}, nil
}

func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}

func (db *DB) Ready(ctx context.Context) error {
	var one int
	return db.Pool.QueryRow(ctx, "select 1").Scan(&one)
}

// RunMigration executes a single SQL file (MVP) to keep dependencies zero.
func (db *DB) RunMigration(ctx context.Context, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open migration: %w", err)
	}
	defer f.Close()
	sqlBytes, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	_, err = db.Pool.Exec(ctx, string(sqlBytes))
	if err != nil {
		return fmt.Errorf("exec migration: %w", err)
	}
	return nil
}
