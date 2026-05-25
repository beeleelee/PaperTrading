# PaperTrading

Agent-based paper trading simulator. Price is discovered through order matching — not dictated by CSV data.

## Quick start

### CLI backtester

```bash
go run ./cmd/papertrade/ -csv test/data/aapl_5min.csv -symbol AAPL -strategy ma_cross
```

Flags: `-cash`, `-fast`, `-slow`, `-qty`, `-strategy` (ma_cross, price_cross), noise config (`-noise`, `-noise-count`, `-noise-spread`, `-noise-mkt-rate`), risk constraints (`-max-drawdown`, `-max-positions`), storage (`-db` memory|path).

### API server + Web UI

```bash
# Terminal 1: backend
go run ./cmd/papertrade-server/

# Terminal 2: frontend dev server
cd web && npm run dev
```

Open `http://localhost:5173`. The Vite dev server proxies `/api` → `:8080` and `/ws` → `ws://localhost:8080`.

## How it works

- **CSV** provides the initial price and timeline (timestamps). Subsequent CSV prices are ignored.
- **Noise traders** generate random limit orders around the current mark price and place market orders — this creates baseline price movement (random walk).
- **Mark price** = last traded (fill) price, updated atomically on every trade.
- **Strategy** receives ticks with the current mark price, not CSV reference prices.
- **Order book depth** is streamed via WebSocket (`bids`/`asks` arrays) and visible in the UI.

## Commands

| Scope | Command | Description |
|---|---|---|
| Backend | `go run ./cmd/papertrade/` | CLI backtester |
| Backend | `go run ./cmd/papertrade-server/` | API server on `:8080` |
| Backend | `go test ./...` | All Go tests |
| Backend | `go vet ./...` | Static analysis |
| Frontend | `npm run dev` | Vite dev server |
| Frontend | `npm run build` | `tsc -b && vite build` → `internal/web/dist/` |

## Stack

- **Backend:** Go 1.26, stdlib routing, `modernc.org/sqlite` (pure Go, no CGO), `shopspring/decimal` for money
- **Frontend:** React 19, Vite 6, TypeScript 5.8, `lightweight-charts`
- **No** ESLint, Prettier, formatter config, CI, or pre-commit hooks
