# Event-Driven Architecture in Go

## Why Events?

- **Decoupling**: Market data producer doesn't need to know about Strategy; Strategy doesn't know about Order Execution.
- **Extensibility**: Add new consumers (metrics, logging, persistence) without touching existing code.
- **Auditability**: Every state change is recorded as an event.
- **Deterministic replay**: Same events → same state, essential for backtesting.

## Domain Events

Events are **past-tense, immutable** records of something that happened:

```go
package event

import (
    "time"
    "papertrading/internal/domain/core"
)

type Event interface {
    EventName() string
    Timestamp() time.Time
}

// ---- Market Events ----

type TickReceived struct {
    Symbol    core.Symbol
    Price     core.Money
    Volume    int64
    At        time.Time
}

func (e TickReceived) EventName() string     { return "market.tick_received" }
func (e TickReceived) Timestamp() time.Time   { return e.At }

// ---- Order Events ----

type OrderSubmitted struct {
    OrderID   string
    Symbol    core.Symbol
    Side      core.OrderSide
    Type     core.OrderType
    Price     core.Money
    Quantity  int64
    At        time.Time
}

func (e OrderSubmitted) EventName() string    { return "order.submitted" }
func (e OrderSubmitted) Timestamp() time.Time { return e.At }

type OrderFilled struct {
    OrderID   string
    FillID    string
    Symbol    core.Symbol
    Side      core.OrderSide
    Price     core.Money
    Quantity  int64
    At        time.Time
}

func (e OrderFilled) EventName() string       { return "order.filled" }
func (e OrderFilled) Timestamp() time.Time    { return e.At }

type OrderCancelled struct { ... }
type OrderRejected struct { ... }

// ---- Portfolio Events ----

type PositionOpened struct { ... }
type PositionClosed struct { ... }
type PositionUpdated struct { ... }
```

## In-Process Event Bus

Simple, no external dependency:

```go
package event

import (
    "context"
    "sync"
)

type Handler func(ctx context.Context, event Event) error

type Bus struct {
    mu       sync.RWMutex
    handlers map[string][]Handler  // event name → handlers
}

func NewBus() *Bus {
    return &Bus{
        handlers: make(map[string][]Handler),
    }
}

func (b *Bus) Subscribe(eventName string, handler Handler) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.handlers[eventName] = append(b.handlers[eventName], handler)
}

func (b *Bus) Publish(ctx context.Context, event Event) error {
    b.mu.RLock()
    handlers := b.handlers[event.EventName()]
    b.mu.RUnlock()

    for _, handler := range handlers {
        if err := handler(ctx, event); err != nil {
            return err
        }
    }
    return nil
}

// PublishAsync dispatches in a goroutine (fire-and-forget).
func (b *Bus) PublishAsync(ctx context.Context, event Event) {
    go func() {
        _ = b.Publish(ctx, event)
    }()
}
```

## Event Flow

```
                   ┌──────────────────┐
                   │   Market Feed    │
                   │  (TickProducer)  │
                   └────────┬─────────┘
                            │ TickReceived
                            ▼
                   ┌──────────────────┐
                   │   Event Bus      │
                   └──┬────┬────┬─────┘
                      │    │    │
          ┌───────────┘    │    └───────────┐
          ▼                ▼                ▼
   ┌────────────┐  ┌────────────┐  ┌──────────────┐
   │  Strategy  │  │  Metrics   │  │  Persistence │
   │  Engine    │  │  Logger    │  │  (Trade Log) │
   └─────┬──────┘  └────────────┘  └──────────────┘
         │ OrderSubmitted
         ▼
   ┌────────────┐
   │   Order    │
   │ Execution  │
   └─────┬──────┘
         │ OrderFilled / OrderCancelled
         ▼
   ┌────────────┐
   │  Portfolio │
   │  Manager   │
   └────────────┘
```

## Wiring (in `cmd/papertrade/main.go`)

```go
func main() {
    bus := event.NewBus()

    // Wire up domain services
    matchingEngine := &order.MatchingEngine{}
    portfolioManager := portfolio.NewManager(...)
    strategy := &myStrategy{}

    // Subscribe handlers to events
    bus.Subscribe("market.tick_received", strategy.OnTick)
    bus.Subscribe("order.submitted", matchingEngine.OnOrderSubmitted)
    bus.Subscribe("order.filled", portfolioManager.OnOrderFilled)
    bus.Subscribe("order.filled", metricsLogger.OnOrderFilled)
    bus.Subscribe("order.filled", persistence.OnOrderFilled)

    // Start the feed
    feed := csvfeed.New("data/sample.csv", bus)
    feed.Start(ctx)
}
```

## Considerations

| Approach | When to use |
|---|---|
| **Synchronous** (Publish) | Single-process simulation, backtesting — simpler, deterministic |
| **Async** (PublishAsync) | Live trading where latency matters, or when handlers do I/O |
| **Buffered channel** | High-throughput scenarios where backpressure is needed |
| **Watermill / NATS** | Only if you need multi-process or distributed event bus (overkill for now) |

**Recommendation**: Start with synchronous in-process bus. It's deterministic, simple to debug, and sufficient for a paper trading simulator. Add async only if latency becomes an issue.
