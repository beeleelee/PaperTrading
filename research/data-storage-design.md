# Data Storage Design

## Strategy: Dual-Driver with `database/sql`

| Environment | Database | Driver | Reason |
|---|---|---|---|
| **Local dev / tests** | SQLite | `modernc.org/sqlite` (pure Go, no CGO) | Zero setup, in-memory option for tests |
| **Production** | PostgreSQL | `github.com/jackc/pgx/v5` | Concurrency, durability, network access |

Both use Go's `database/sql` interface with driver-specific connection strings.
SQL is kept 99% compatible (differences noted below).

## Why This Split

- **SQLite for development**: instant setup, `:memory:` for tests, single file, no daemon.
- **PostgreSQL for production**: proper concurrent access, roles, backups, networking, battle-tested for financial data.
- **`database/sql`** abstracts the difference — repos depend on `*sql.DB`, not the driver.

## Drivers

```go
import (
    "database/sql"
    _ "modernc.org/sqlite"       // SQLite driver (pure Go, no CGO)
    _ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver via database/sql
)

// Local: in-memory SQLite
db, _ := sql.Open("sqlite", ":memory:")

// Local: file-based SQLite
db, _ := sql.Open("sqlite", "./data/trading.db?_journal_mode=WAL")

// Production: PostgreSQL
db, _ := sql.Open("pgx", "postgres://user:pass@localhost:5432/papertrading?sslmode=require")
```

## Schema (PostgreSQL-First)

Designed for PostgreSQL but compatible with SQLite's subset.
Where SQLite lacks a feature, we avoid it or provide an alternative.

```sql
-- Orders table: append-only audit log of every order
CREATE TABLE orders (
    id            TEXT PRIMARY KEY,
    portfolio_id  TEXT NOT NULL,
    symbol        TEXT NOT NULL,
    side          TEXT NOT NULL CHECK (side IN ('buy', 'sell')),
    type          TEXT NOT NULL CHECK (type IN ('market', 'limit', 'stop', 'stop_limit')),
    status        TEXT NOT NULL CHECK (status IN ('created', 'submitted', 'partially_filled', 'filled', 'cancelled', 'rejected')),
    price         NUMERIC NOT NULL,      -- decimal type in PG; TEXT in SQLite
    stop_price    NUMERIC,               -- null for non-stop orders
    quantity      BIGINT NOT NULL,
    filled_qty    BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL,   -- TIMESTAMP in SQLite
    updated_at    TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_orders_portfolio ON orders(portfolio_id);
CREATE INDEX idx_orders_status ON orders(status);

-- Fills table: one row per fill event
CREATE TABLE fills (
    id            TEXT PRIMARY KEY,
    order_id      TEXT NOT NULL REFERENCES orders(id),
    symbol        TEXT NOT NULL,
    side          TEXT NOT NULL,
    price         NUMERIC NOT NULL,
    quantity      BIGINT NOT NULL,
    commission    NUMERIC NOT NULL DEFAULT 0,
    timestamp     TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_fills_order ON fills(order_id);
CREATE INDEX idx_fills_timestamp ON fills(timestamp);

-- Portfolio snapshots: point-in-time equity values
CREATE TABLE portfolio_snapshots (
    id            BIGSERIAL PRIMARY KEY,   -- INTEGER PRIMARY KEY AUTOINCREMENT in SQLite
    portfolio_id  TEXT NOT NULL,
    cash          NUMERIC NOT NULL,
    equity        NUMERIC NOT NULL,
    timestamp     TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_snapshots_portfolio ON portfolio_snapshots(portfolio_id);
CREATE INDEX idx_snapshots_time ON portfolio_snapshots(timestamp);

-- Positions: current open positions (updated, not appended)
CREATE TABLE positions (
    portfolio_id    TEXT NOT NULL,
    symbol          TEXT NOT NULL,
    quantity        BIGINT NOT NULL,
    avg_entry_price NUMERIC NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (portfolio_id, symbol)
);
```

## SQL Compatibility Notes

| Feature | PostgreSQL | SQLite | Workaround |
|---|---|---|---|
| Decimal type | `NUMERIC` | `TEXT` | Option 1: use `TEXT` in both, parse in Go. Option 2: use `NUMERIC` in PG migrations, `TEXT` in SQLite migrations with separate files. **Recommendation**: use `TEXT` in both — simplest, most portable, and the `shopspring/decimal` library reads text natively. |
| Auto-increment | `BIGSERIAL` | `INTEGER PRIMARY KEY AUTOINCREMENT` | Avoid auto-increment for domain IDs (use UUIDs). Only `portfolio_snapshots.id` uses it — use separate migration files. |
| Timestamp | `TIMESTAMPTZ` | `TEXT` (ISO 8601) | Use `TEXT` in SQLite, `TIMESTAMPTZ` in PG. Format as RFC 3339 in Go. |
| Upsert | `INSERT ... ON CONFLICT DO UPDATE` | `INSERT ... ON CONFLICT DO UPDATE` | **Same syntax** since SQLite 3.24.0 (2018). |

**Recommendation**: Use separate migration files per driver under `migrations/{sqlite,postgres}/`.

## Decimal Storage

Two options:

### Option A: TEXT everywhere (recommended for portability)

Store monetary values as **text** (string representation of decimal). This works identically in both databases and is human-readable. The `shopspring/decimal` library parses these strings correctly.

### Option B: NUMERIC in PG, TEXT in SQLite

More "native" but requires maintaining two schema files. Only worthwhile if you need PostgreSQL-side calculations (e.g., `SUM(price * quantity)` in SQL).

## Repository Implementation

### Interface (domain layer)

```go
package order

import "context"

type Repository interface {
    Save(ctx context.Context, o *Order) error
    FindByID(ctx context.Context, id OrderID) (*Order, error)
    FindByStatus(ctx context.Context, status core.OrderStatus) ([]*Order, error)
}
```

### PostgreSQL implementation

```go
package postgres

import (
    "context"
    "database/sql"
    "papertrading/internal/domain/order"
)

type OrderRepository struct {
    db *sql.DB
}

func (r *OrderRepository) Save(ctx context.Context, o *order.Order) error {
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO orders (id, portfolio_id, symbol, side, type, status,
                           price, stop_price, quantity, filled_qty, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
        ON CONFLICT(id) DO UPDATE SET
            status = EXCLUDED.status,
            filled_qty = EXCLUDED.filled_qty,
            updated_at = EXCLUDED.updated_at
    `, o.ID, /* portfolio_id */, o.Symbol, o.Side.String(), o.Type.String(),
       o.Status.String(), o.Price.String(), /* stopPrice */, o.Quantity, o.FilledQty,
       o.CreatedAt.UTC(), o.UpdatedAt.UTC())
    return err
}
```

### SQLite implementation

```go
package sqlite

import (
    "context"
    "database/sql"
    "papertrading/internal/domain/order"
)

type OrderRepository struct {
    db *sql.DB
}

func (r *OrderRepository) Save(ctx context.Context, o *order.Order) error {
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO orders (id, portfolio_id, symbol, side, type, status,
                           price, stop_price, quantity, filled_qty, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            status = excluded.status,
            filled_qty = excluded.filled_qty,
            updated_at = excluded.updated_at
    `, o.ID, /* portfolio_id */, o.Symbol, o.Side.String(), o.Type.String(),
       o.Status.String(), o.Price.String(), /* stopPrice */, o.Quantity, o.FilledQty,
       o.CreatedAt.Format(time.RFC3339Nano), o.UpdatedAt.Format(time.RFC3339Nano))
    return err
}
```

Only differences: placeholder syntax (`$1` vs `?`) and timestamp format (Go `time.Time` vs RFC 3339 string).
Extract a shared SQL constants file to minimize duplication.

## Transactional Writes

Portfolio updates (cash + position) must be atomic in both databases:

```go
func (r *PortfolioRepository) ApplyFill(ctx context.Context, fill order.Fill, order order.Order) error {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // 1. Update cash
    cost := fill.Price.Mul(fill.Quantity)
    if _, err := tx.ExecContext(ctx, `UPDATE portfolios SET cash = cash - $1 WHERE id = $2`,
        cost.String(), ...); err != nil {
        return err
    }

    // 2. Update position (UPSERT)
    newAvg := calculateNewAvgPrice(/* ... */)
    if _, err := tx.ExecContext(ctx, `
        INSERT INTO positions (portfolio_id, symbol, quantity, avg_entry_price, updated_at)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT(portfolio_id, symbol) DO UPDATE SET
            quantity = positions.quantity + EXCLUDED.quantity,
            avg_entry_price = $6,
            updated_at = EXCLUDED.updated_at
    `, ...); err != nil {
        return err
    }

    // 3. Record fill
    if _, err := tx.ExecContext(ctx, `INSERT INTO fills (...) VALUES (...)`); err != nil {
        return err
    }

    return tx.Commit()
}
```

Note: `avg_entry_price` recalculation:

```
new_avg = (old_qty * old_avg + fill_qty * fill_price) / (old_qty + fill_qty)
```

This is computed in Go before the UPSERT.

## Wiring (in `cmd/papertrade/main.go`)

```go
package main

import (
    "database/sql"
    "flag"
    _ "modernc.org/sqlite"
    _ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
    dbURL := flag.String("db", "sqlite://./data/trading.db", "Database URL")
    flag.Parse()

    db := openDB(*dbURL)
    defer db.Close()
    // ...
}

func openDB(dsn string) *sql.DB {
    switch {
    case strings.HasPrefix(dsn, "sqlite://"):
        path := strings.TrimPrefix(dsn, "sqlite://")
        db, _ := sql.Open("sqlite", path)
        return db
    case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"):
        db, _ := sql.Open("pgx", dsn)
        return db
    default:
        // fallback
    }
}
```

## Migration Strategy

Separate migrations per driver, embedded in the binary:

```
internal/infra/storage/
├── migrations/
│   ├── sqlite/
│   │   └── 001_initial.sql
│   └── postgres/
│       └── 001_initial.sql
```

```go
package storage

import (
    "database/sql"
    "embed"
    "fmt"
)

//go:embed migrations/sqlite/*.sql
var sqliteMigrations embed.FS

//go:embed migrations/postgres/*.sql
var postgresMigrations embed.FS

func Migrate(db *sql.DB, driver string) error {
    var fs embed.FS
    switch driver {
    case "sqlite":
        fs = sqliteMigrations
    case "pgx":
        fs = postgresMigrations
    default:
        return fmt.Errorf("unknown driver: %s", driver)
    }
    // Read and apply migrations in order
}
```

## In-Memory Repository (for Testing)

Fast unit tests without any database:

```go
package memory

import (
    "context"
    "sync"
    "papertrading/internal/domain/order"
)

type OrderRepository struct {
    mu     sync.RWMutex
    orders map[order.OrderID]*order.Order
}

func NewOrderRepository() *OrderRepository {
    return &OrderRepository{orders: make(map[order.OrderID]*order.Order)}
}

func (r *OrderRepository) Save(_ context.Context, o *order.Order) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    clone := *o
    r.orders[o.ID] = &clone
    return nil
}
```

## Decision Summary

| Decision | Rationale |
|---|---|
| **Dual-driver** (`database/sql`) | One interface, two impls; local dev speed + production robustness |
| **TEXT for decimal** | Portable between PG and SQLite; exact precision; `shopspring/decimal` reads it |
| **UUIDs for IDs** | No auto-increment issues across DBs; safe for distributed use |
| **Separate migration files** | PG and SQLite have different type systems; explicit is better |
| **In-memory repos for tests** | Fastest tests; no DB setup; complements SQLite integration tests |
| **No ORM** | `database/sql` is sufficient; ORMs hide SQL complexity that matters for financial data |
| **pgx driver** | Best PostgreSQL driver for Go; supports `database/sql` stdlib interface |
| **modernc.org/sqlite** | Pure Go, no CGO; works with `database/sql` |
