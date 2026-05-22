# Domain-Driven Design Analysis for Paper Trading System

## 1. Ubiquitous Language

| Term | Definition |
|---|---|
| **Tick** | A single market data point: `(symbol, price, volume, timestamp)` |
| **Order** | An instruction to trade; typed as MARKET, LIMIT, STOP, or STOP_LIMIT |
| **Fill** | An executed trade that partially or fully satisfies an Order |
| **Position** | A holding in a given symbol, tracking quantity and average entry price |
| **Portfolio** | Aggregate root that owns cash balance and all open Positions |
| **Equity** | Mark-to-market portfolio value = cash + sum of positions × current price |
| **OrderBook** | Collection of buy/sell limit orders organized by price level |
| **Order Lifecycle** | `CREATED → SUBMITTED → PARTIALLY_FILLED → FILLED \| CANCELLED \| REJECTED` |
| **Strategy** | Domain logic that consumes Ticks and emits Orders |
| **Signal** | A computed decision from a Strategy (e.g., "buy 100 GOOG at market") |
| **Drawdown** | Peak-to-trough decline in portfolio equity |
| **PnL** | Profit and loss, split into **realized** (closed trades) and **unrealized** (open positions) |

## 2. Bounded Contexts

```
┌─────────────────────────────────────────────────────────┐
│                   Paper Trading System                   │
│                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │   Market     │  │    Order     │  │  Portfolio   │  │
│  │   Data       │──▶│  Execution   │──▶│  Management │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  │
│        │                │                  │            │
│        ▼                ▼                  ▼            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │  Strategy    │  │  Matching    │  │    Risk      │  │
│  │  Engine      │  │  Engine      │  │  Constraints │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  │
│                                                         │
│  ┌──────────────────────────────────────────────────┐   │
│  │              Metrics / Performance               │   │
│  └──────────────────────────────────────────────────┘   │
│                                                         │
│  ┌──────────────────────────────────────────────────┐   │
│  │                  Persistence                     │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

### Context Definitions

| Bounded Context | Responsibility | Key Entities |
|---|---|---|
| **Market Data** | Feed ticks from CSV/live source, normalize timestamps, replay | Tick, DataFeed, Clock |
| **Order Execution** | Manage order lifecycle, match orders, generate fills | Order, Fill, MatchingEngine, OrderBook |
| **Portfolio Management** | Track cash, positions, equity, PnL | Portfolio, Position, Cash |
| **Strategy Engine** | Consume ticks, compute signals, submit orders | Strategy, Signal, Indicator |
| **Risk Constraints** | Enforce position limits, margin, drawdown caps | RiskRule, PositionLimit |
| **Metrics** | Compute Sharpe, drawdown, win rate, equity curve | MetricsCalculator, TradeLog |
| **Persistence** | Store/load trades, orders, portfolio snapshots | Repository interfaces |

## 3. Core vs Supporting Domains

- **Core Domain**: **Order Execution** — the matching engine and order lifecycle are the most complex, most differentiated part of a paper trading system. Getting fills, slippage, and partial fills right determines simulation fidelity.
- **Supporting**: Market Data (commodity), Portfolio Management (well-understood), Strategy Engine (pluggable), Metrics (standard formulas).
- **Generic**: Persistence (SQLite is generic).

## 4. Aggregate Root Candidates

### Portfolio (Aggregate Root)
- Owns: `Cash`, `Position[]`, `TradeLog`
- Invariants:
  - `cash + sum(position.markToMarket) == equity`
  - `cash >= 0` (no borrowing without margin)
  - No duplicate positions per symbol
- Events: `PositionOpened`, `PositionClosed`, `PositionUpdated`, `CashDeposited`, `CashWithdrawn`

### Order (Aggregate Root)
- Owns: `Fills[]`
- Invariants:
  - Sum of fill quantities ≤ order quantity
  - Order status transitions are valid (cannot go from FILLED back to SUBMITTED)
- Events: `OrderSubmitted`, `OrderPartiallyFilled`, `OrderFilled`, `OrderCancelled`, `OrderRejected`

## 5. Repository Interfaces

```go
type OrderRepository interface {
    Save(ctx context.Context, order *Order) error
    FindByID(ctx context.Context, id OrderID) (*Order, error)
    FindByStatus(ctx context.Context, status OrderStatus) ([]*Order, error)
}

type PortfolioRepository interface {
    Save(ctx context.Context, portfolio *Portfolio) error
    FindByID(ctx context.Context, id PortfolioID) (*Portfolio, error)
}

type TradeRepository interface {
    Append(ctx context.Context, fill *Fill) error
    FindByDateRange(ctx context.Context, from, to time.Time) ([]*Fill, error)
}
```

## 6. Domain Services (Stateless Operations)

- **MatchingEngine**: Accepts incoming Orders + current OrderBook, produces Fills
- **PricingService**: Returns current price for a symbol (wraps OrderBook top-of-book or last tick)
- **RiskEvaluator**: Checks order against position limits, drawdown caps, margin rules
- **MetricsCalculator**: Computes Sharpe ratio, max drawdown, win rate from trade log

## 7. First Implementation Steps

1. **Define domain types** (`src/core/`) — pure Go structs with no dependencies
2. **Define domain events** — interfaces/structs for event-driven communication
3. **Define repository interfaces** — contract for persistence
4. **Implement Portfolio aggregate** — the root aggregate with cash + positions
5. **Implement Order aggregate** — with lifecycle validation
6. **Implement MatchingEngine service** — the core domain logic
7. **Wire together via main** or a simulation runner

The key DDD principle to follow: **domain logic in entities/aggregates/services has zero infrastructure dependencies** — no database calls, no network, no framework imports.
