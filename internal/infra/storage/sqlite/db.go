package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	d := &DB{db}
	if err := d.migrate(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}

func (d *DB) migrate(ctx context.Context) error {
	if _, err := d.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS _migrations (
			version   INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)
	`); err != nil {
		return err
	}

	migrations := []struct {
		version int
		sql     string
	}{
		{1, `
			CREATE TABLE IF NOT EXISTS orders (
				id          TEXT PRIMARY KEY,
				portfolio_id TEXT NOT NULL,
				symbol      TEXT NOT NULL,
				side        INTEGER NOT NULL,
				type        INTEGER NOT NULL,
				status      INTEGER NOT NULL,
				price       TEXT NOT NULL,
				stop_price  TEXT NOT NULL,
				quantity    INTEGER NOT NULL,
				filled_qty  INTEGER NOT NULL,
				created_at  TEXT NOT NULL,
				updated_at  TEXT NOT NULL
			);
			CREATE TABLE IF NOT EXISTS fills (
				id          TEXT PRIMARY KEY,
				order_id    TEXT NOT NULL REFERENCES orders(id),
				price       TEXT NOT NULL,
				quantity    INTEGER NOT NULL,
				commission  TEXT NOT NULL,
				timestamp   TEXT NOT NULL
			);
			CREATE TABLE IF NOT EXISTS portfolios (
				id           TEXT PRIMARY KEY,
				cash         TEXT NOT NULL,
				realized_pnl TEXT NOT NULL DEFAULT '0',
				created_at   TEXT NOT NULL,
				updated_at   TEXT NOT NULL
			);
			CREATE TABLE IF NOT EXISTS positions (
				portfolio_id  TEXT NOT NULL REFERENCES portfolios(id),
				symbol        TEXT NOT NULL,
				quantity      INTEGER NOT NULL,
				avg_entry_price TEXT NOT NULL,
				opened_at     TEXT NOT NULL,
				PRIMARY KEY (portfolio_id, symbol)
			);
			CREATE TABLE IF NOT EXISTS closed_trades (
				id          INTEGER PRIMARY KEY AUTOINCREMENT,
				portfolio_id TEXT NOT NULL REFERENCES portfolios(id),
				symbol      TEXT NOT NULL,
				entry_price TEXT NOT NULL,
				exit_price  TEXT NOT NULL,
				quantity    INTEGER NOT NULL,
				pnl         TEXT NOT NULL,
				opened_at   TEXT NOT NULL,
				closed_at   TEXT NOT NULL
			);
		`},
	}

	var currentVersion int
	if err := d.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM _migrations`).Scan(&currentVersion); err != nil {
		return fmt.Errorf("read migration version: %w", err)
	}

	for _, m := range migrations {
		if m.version > currentVersion {
			tx, err := d.BeginTx(ctx, nil)
			if err != nil {
				return fmt.Errorf("begin migration %d: %w", m.version, err)
			}
			if _, err := tx.ExecContext(ctx, m.sql); err != nil {
				tx.Rollback()
				return fmt.Errorf("apply migration %d: %w", m.version, err)
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO _migrations (version, applied_at) VALUES (?, ?)`,
				m.version, time.Now().UTC().Format(time.RFC3339),
			); err != nil {
				tx.Rollback()
				return fmt.Errorf("record migration %d: %w", m.version, err)
			}
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("commit migration %d: %w", m.version, err)
			}
		}
	}
	return nil
}
