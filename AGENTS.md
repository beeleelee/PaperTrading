# PaperTrading — Agent Guide

## Stack
- **Backend:** Go 1.26.2, stdlib routing (Go 1.22+ `"METHOD /path"` pattern), `modernc.org/sqlite` (pure Go, no CGO), `shopspring/decimal` for money
- **Frontend:** React 19, Vite 6, TypeScript 5.8, npm
- **No** ESLint, Prettier, formatter config, CI, or pre-commit hooks

## Commands

| Scope | Command | What |
|---|---|---|
| Backend | `go run ./cmd/papertrade/` | Run CLI backtester |
| Backend | `go run ./cmd/papertrade-server/` | Run API server on `:8080` |
| Backend | `go test ./...` | All Go tests |
| Backend | `go test -run TestName ./internal/domain/order/` | Single test |
| Backend | `go vet ./...` | Static analysis |
| Frontend | `npm run dev` | Vite dev server (proxies `/api` → `:8080`, `/ws` → `ws://localhost:8080`) |
| Frontend | `npm run build` | `tsc -b && vite build`, outputs to `internal/web/dist/` |

## Architecture

- **Entrypoints:** `cmd/papertrade/` (CLI flags: `-csv`, `-symbol`, `-strategy`, `-db`, etc.) and `cmd/papertrade-server/` (flag: `-addr`)
- **DDD layers** (dependency direction): `domain/` (zero infra deps) → `event/` → `app/` → `api/`, `infra/` → `cmd/`
- **`internal/web/embed.go`** embeds built SPA via `//go:embed dist` → single binary deployment
- **API:** `POST/GET/DELETE /api/v1/backtests`, `GET /ws/backtests/{id}` (WebSocket tick + depth stream)
- **Storage:** `"memory"` or SQLite path via `-db` flag; tests use in-memory repos

## Simulation Model

The simulation is now **agent-based** — price is discovered through order matching, not dictated by CSV data:
- **CSV** provides only the initial price and the timeline (timestamps). Subsequent CSV prices are ignored.
- **Noise traders** (`market.NoiseConfig`) generate random limit orders around the current mark price and place market orders that fill against the book — this creates baseline price movement (random walk).
- **Mark price** = last traded (fill) price. Updated atomically on every fill via `atomicFill` (handles both sides of a trade).
- **Strategy** receives ticks with the current mark price (not CSV reference prices).
- **Order book depth** (`OrderBookManager.Depth()`) is streamed via WebSocket as `bids`/`asks` arrays.

### Key files
- `internal/domain/market/noise.go` — `NoiseConfig`, `GenerateOrders()` (limit + market orders), `DetectCross()` (noise-noise matching)
- `internal/app/simulation.go` — `matchAgainstBook()`, `atomicFill()`, `crossFill()`, `processTick()` with markPrice tracking
- `internal/app/depth.go` — `DepthLevel`, `DepthSnapshot`, `OrderBookManager.Depth()`

### CLI noise flags
```
-noise (default true) -noise-count (5) -noise-spread (50 BP) -noise-mkt-rate (0.3)
```

## Caveats

- **`internal/domain/order/matching.go`** (domain-level MatchingEngine) is **dead code** — the simulation uses `OrderBookManager.FindMatch` and `matchAgainstBook` in `internal/app/simulation.go`
- **No live market data** — CSV replay only (`test/data/aapl_5min.csv`)
- **Event bus** (`internal/event/`) is in-process; `Publish` is synchronous (`PublishAsync` previously caused panics)
- **Money values** must use `shopspring/decimal` (`core.Money`) — never float64
- **Tests use `rand.Seed()`** for reproducible noise — no fixed seed means flaky order counts across runs

## Documentation
- `hints.md` — design guidance and pitfalls
- `research/*.md` — architecture decisions and retrospective
