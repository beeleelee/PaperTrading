# Go Project Layout (DDD + Go Conventions)

This layout follows the **Standard Go Project Layout** conventions combined with DDD bounded contexts.
No framework, no fat internal packages — just clean separation.

```
PaperTrading/
├── cmd/
│   └── papertrade/            # Application entrypoint
│       └── main.go
├── internal/
│   ├── domain/                # DDD domain layer (zero dependencies on infra)
│   │   ├── market/            # Bounded context: Market Data
│   │   │   ├── tick.go
│   │   │   ├── feed.go        # interface
│   │   │   └── errors.go
│   │   ├── order/             # Bounded context: Order Execution
│   │   │   ├── order.go       # aggregate
│   │   │   ├── fill.go        # value object
│   │   │   ├── matching.go    # domain service
│   │   │   ├── repository.go  # interface
│   │   │   └── errors.go
│   │   ├── portfolio/         # Bounded context: Portfolio Management
│   │   │   ├── portfolio.go   # aggregate root
│   │   │   ├── position.go    # entity
│   │   │   ├── cash.go        # value object
│   │   │   ├── repository.go  # interface
│   │   │   └── errors.go
│   │   ├── strategy/          # Bounded context: Strategy Engine
│   │   │   ├── strategy.go    # interface
│   │   │   └── signal.go
│   │   ├── risk/              # Bounded context: Risk Constraints
│   │   │   ├── rules.go
│   │   │   └── errors.go
│   │   ├── metrics/           # Bounded context: Metrics
│   │   │   ├── calculator.go
│   │   │   └── types.go
│   │   └── core/              # Shared kernel (used by all contexts)
│   │       ├── money.go       # Decimal-based money type
│   │       ├── symbol.go      # Ticker symbol value object
│   │       ├── types.go       # OrderType, OrderStatus enums
│   │       └── errors.go      # Domain-wide error types
│   ├── app/                   # Application layer (use cases / orchestration)
│   │   ├── simulation.go      # Orchestrates a simulation run
│   │   ├── backtest.go        # Backtest use case
│   │   └── live.go            # Live trading use case
│   ├── infra/                 # Infrastructure layer
│   │   ├── feed/              # Market data implementations
│   │   │   ├── csvfeed.go
│   │   │   └── livefeed.go
│   │   ├── storage/           # Persistence implementations
│   │   │   ├── sqlite/
│   │   │   │   ├── order_repo.go
│   │   │   │   ├── portfolio_repo.go
│   │   │   │   ├── trade_repo.go
│   │   │   │   └── migrations.go
│   │   │   └── memory/        # In-memory repos for tests
│   │   │       ├── order_repo.go
│   │   │       └── portfolio_repo.go
│   │   └── clock/             # Clock abstraction
│   │       ├── real_clock.go
│   │       └── simulated_clock.go
│   └── event/                 # Event bus (in-process)
│       ├── bus.go
│       └── events.go          # Domain event types
├── test/                      # Test data & helpers
│   ├── data/                  # Sample CSV market data
│   └── fixtures/              # Test fixtures
├── data/                      # Market data files (gitignored large files)
├── research/                  # Design research (gitignored)
├── go.mod
└── go.sum
```

## Key Decisions

| Decision | Rationale |
|---|---|
| `internal/` package | Prevents external packages from importing our internals — clean API boundary |
| Domain in `internal/domain/` | Zero dependencies on `infra/` or `app/` — can be tested in isolation |
| Interfaces **in domain**, impls **in infra** | Domain defines what it needs; infra provides the concrete implementation |
| `cmd/` only has `main.go` | Thin entrypoint that wires everything together |
| Events package separate | In-process event bus keeps contexts loosely coupled |
| No framework | std library + `go-sqlite3` driver is sufficient; avoids framework lock-in |

## Import Rules

- `domain/` → imports nothing outside `domain/`
- `app/` → imports `domain/` and `event/`
- `infra/` → imports `domain/` and optionally `event/`
- `cmd/` → imports everything, wires it up
- `event/` → imports `domain/` (event types reference domain types)
