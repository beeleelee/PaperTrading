# Concurrency Model for Paper Trading in Go

## Design Principle

A paper trading simulator is fundamentally **sequential**: ticks arrive one at a time, orders are processed one at a time, portfolio updates are applied one at a time. This is a serial pipeline, not a parallel workload.

Go's concurrency features are used for **composability and non-blocking I/O**, not parallelism.

## Pipeline Architecture

```
┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐
│  Feed    │──▶│ Strategy │──▶│  Order   │──▶│Portfolio │
│ (reader) │   │ (worker) │   │  Exec    │   │ Update   │
└──────────┘   └──────────┘   └──────────┘   └──────────┘
     │              │              │              │
     ▼              ▼              ▼              ▼
  tickCh        signalCh         fillsCh       done
```

## Channel-Based Pipeline

```go
package app

import (
    "context"
    "papertrading/internal/domain/core"
    "papertrading/internal/domain/market"
    "papertrading/internal/domain/order"
    "papertrading/internal/domain/portfolio"
)

type Simulation struct {
    feed     market.Feed
    strategy Strategy
    matching *order.MatchingEngine
    portfolio *portfolio.Portfolio
}

func (s *Simulation) Run(ctx context.Context, initialCash core.Money) error {
    s.portfolio = portfolio.NewPortfolio("sim1", initialCash)

    tickCh := make(chan market.Tick, 100)
    signalCh := make(chan order.Signal, 100)

    // Start pipeline stages concurrently
    go s.feed.Stream(ctx, tickCh)
    go s.strategy.Run(ctx, tickCh, signalCh)

    // Main loop: process signals sequentially
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case signal, ok := <-signalCh:
            if !ok {
                return nil  // feed exhausted
            }
            if err := s.processSignal(ctx, signal); err != nil {
                return err
            }
        }
    }
}

func (s *Simulation) processSignal(ctx context.Context, sig order.Signal) error {
    // 1. Convert signal to order
    ord := sig.ToOrder()

    // 2. Check risk rules
    if err := risk.Check(s.portfolio, ord); err != nil {
        return err  // or log rejection
    }

    // 3. Match order against order book
    fills, err := s.matching.Match(ord, s.orderBook)
    if err != nil {
        return err
    }

    // 4. Apply fills to order and portfolio (sequentially)
    for _, fill := range fills {
        if err := ord.ApplyFill(fill); err != nil {
            return err
        }
        if err := s.portfolio.ApplyFill(fill); err != nil {
            return err
        }
        // Persist fill
    }

    return nil
}
```

## Feed Goroutine

```go
// csvfeed.go
package csvfeed

import (
    "context"
    "encoding/csv"
    "os"
    "papertrading/internal/domain/market"
)

type Feed struct {
    path string
}

func (f *Feed) Stream(ctx context.Context, ch chan<- market.Tick) error {
    file, err := os.Open(f.path)
    if err != nil {
        return err
    }
    defer file.Close()

    reader := csv.NewReader(file)
    for {
        record, err := reader.Read()
        if err != nil {
            close(ch)
            return err  // io.EOF is expected
        }
        tick := parseTick(record)
        select {
        case <-ctx.Done():
            return ctx.Err()
        case ch <- tick:
        }
    }
}
```

## Strategy Goroutine

```go
func (s *MovingAverageCross) Run(ctx context.Context, ticks <-chan market.Tick, signals chan<- order.Signal) {
    for {
        select {
        case <-ctx.Done():
            return
        case tick, ok := <-ticks:
            if !ok {
                close(signals)
                return
            }
            signal := s.onTick(tick)
            if signal != nil {
                signals <- *signal
            }
        }
    }
}
```

## Backpressure

Channels with small buffers (100) provide natural backpressure:

- If the strategy processes slower than the feed, the feed blocks on sending to `tickCh`.
- If the main loop processes slower than the strategy, the strategy blocks on sending to `signalCh`.

This means the system runs at the speed of the **slowest stage**, which is correct behavior.

## Graceful Shutdown

```go
func (s *Simulation) Run(ctx context.Context, initialCash core.Money) error {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    // ... pipeline setup ...

    // Handle OS signals
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        <-sigCh
        cancel()
    }()

    // ... main loop ...
}
```

## Time-Accelerated Simulation

For backtesting, we want to process ticks faster than real-time. The `Clock` abstraction handles this:

```go
package clock

import "time"

type Clock interface {
    Now() time.Time
    // Advance moves the clock forward by d during simulation
    Advance(d time.Duration)
}

// SimulatedClock jumps instantly — no real time passes.
type SimulatedClock struct {
    now time.Time
}

func (c *SimulatedClock) Now() time.Time       { return c.now }
func (c *SimulatedClock) Advance(d time.Duration) { c.now = c.now.Add(d) }
```

## Concurrency for Live Trading

In live trading (future feature), the model shifts:

```
Market Feed ──▶ Tick Buffer ──▶ Strategy (goroutine)
                                   │
                            signalsCh (buffered)
                                   │
               ┌───────────────────┘
               ▼
        Main Loop (goroutine)
               │
               ▼
        Order Execution (may block on I/O)
```

The key difference in live mode: the feed and strategy run independently, and an **order manager goroutine** handles async order status updates from the broker API.

## Summary

| Aspect | Decision |
|---|---|
| **Pipeline model** | Channel-based stages, sequential within each stage |
| **Parallelism** | Only between stages (feed ≠ strategy ≠ main loop) |
| **Order execution** | Single-threaded, no race conditions |
| **Buffer size** | Small (100) for natural backpressure |
| **Clock** | Abstracted for both backtest (simulated) and live (real) |
| **Shutdown** | Context cancellation + signal handling |
