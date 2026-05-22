# Testing Strategy

## Test Pyramid for a Paper Trading System

```
         ╱─────╲
        ╱  E2E  ╲         1-2 full simulation runs
       ╱  (few)  ╲         (smoke test the whole pipeline)
      ╰───────────╯
       ╱─────────╲
      ╱Integration╲       10-20 tests: storage + repositories,
     ╱  (some)    ╲       clock + feed integration
    ╰───────────────╯
     ╱───────────────╲
    ╱   Unit Tests    ╲    50+ tests: matching engine, portfolio,
   ╱   (most)         ╲    order lifecycle, risk rules, metrics
  ╰─────────────────────╯
```

## Deterministic Testing Principle

The same input MUST produce the same output. No time.Now(), no random in domain logic.

```go
// Domain types accept time as parameter, not from the clock:
func (o *Order) ApplyFill(f Fill, when time.Time) error {
    // ...
    o.UpdatedAt = when
    return nil
}
```

## Unit Tests (Priority: High)

### Matching Engine

```go
func TestMatchingEngine_MarketBuy_FillsAgainstAsks(t *testing.T) {
    book := order.OrderBook{
        Asks: []order.PriceLevel{
            {Price: core.NewMoneyOrPanic("100.00"), Orders: []*order.Order{
                {ID: "ask1", Quantity: 100, FilledQty: 0},
            }},
        },
    }
    buyOrder := &order.Order{
        ID: "buy1", Side: core.OrderSideBuy, Type: core.OrderTypeMarket, Quantity: 50,
    }
    engine := &order.MatchingEngine{}
    fills, err := engine.Match(buyOrder, &book)
    assert.NoError(t, err)
    assert.Len(t, fills, 1)
    assert.Equal(t, int64(50), fills[0].Quantity)
    assert.Equal(t, "100.00", fills[0].Price.String())
}

func TestMatchingEngine_MarketBuy_NoLiquidity(t *testing.T) {
    book := order.OrderBook{}  // empty
    buyOrder := &order.Order{
        ID: "buy1", Side: core.OrderSideBuy, Type: core.OrderTypeMarket, Quantity: 50,
    }
    engine := &order.MatchingEngine{}
    _, err := engine.Match(buyOrder, &book)
    assert.ErrorIs(t, err, order.ErrNoLiquidity)
}

func TestMatchingEngine_LimitBuy_CrossesThenRests(t *testing.T) {
    book := order.OrderBook{
        Asks: []order.PriceLevel{
            {Price: core.NewMoneyOrPanic("99.00"), Orders: []*order.Order{
                {ID: "ask1", Quantity: 50, FilledQty: 0},
            }},
            {Price: core.NewMoneyOrPanic("101.00"), Orders: []*order.Order{
                {ID: "ask2", Quantity: 100, FilledQty: 0},
            }},
        },
    }
    // Limit buy @ 100: crosses ask @ 99 (fills 50), rest 50 sits @ 100
    buyOrder := &order.Order{
        ID: "buy1", Side: core.OrderSideBuy, Type: core.OrderTypeLimit,
        Price: core.NewMoneyOrPanic("100.00"), Quantity: 100,
    }
    engine := &order.MatchingEngine{}
    fills, err := engine.Match(buyOrder, &book)
    assert.NoError(t, err)
    assert.Len(t, fills, 1)
    assert.Equal(t, int64(50), fills[0].Quantity)
    assert.Equal(t, "99.00", fills[0].Price.String())
    assert.Equal(t, int64(50), buyOrder.RemainingQty())
    // Order should be added to book
    assert.Len(t, book.Bids, 1)
    assert.Equal(t, "100.00", book.Bids[0].Price.String())
}
```

### Order Lifecycle

```go
func TestOrder_ApplyFill_UpdatesStatus(t *testing.T) {
    o := &order.Order{
        ID: "o1", Status: core.OrderStatusSubmitted,
        Quantity: 100, FilledQty: 0,
    }
    fill := order.Fill{Quantity: 100, Price: core.NewMoneyOrPanic("50.00")}
    err := o.ApplyFill(fill)
    assert.NoError(t, err)
    assert.Equal(t, core.OrderStatusFilled, o.Status)
    assert.Equal(t, int64(100), o.FilledQty)
}

func TestOrder_ApplyFill_ExceedsQuantity_Error(t *testing.T) {
    o := &order.Order{
        ID: "o1", Status: core.OrderStatusSubmitted,
        Quantity: 100, FilledQty: 0,
    }
    fill := order.Fill{Quantity: 150, Price: core.NewMoneyOrPanic("50.00")}
    err := o.ApplyFill(fill)
    assert.ErrorIs(t, err, order.ErrFillExceedsOrderQty)
}

func TestOrder_StatusTransition_Invalid(t *testing.T) {
    o := &order.Order{Status: core.OrderStatusFilled}
    valid := o.Status.CanTransitionTo(core.OrderStatusCancelled)
    assert.False(t, valid)
}
```

### Portfolio

```go
func TestPortfolio_TotalEquity(t *testing.T) {
    p := portfolio.NewPortfolio("p1", core.NewMoneyOrPanic("10000.00"))
    pos := &portfolio.Position{Symbol: "AAPL", Quantity: 10, AvgEntryPrice: core.NewMoneyOrPanic("150.00")}
    p.Positions["AAPL"] = pos

    prices := map[core.Symbol]core.Money{"AAPL": core.NewMoneyOrPanic("160.00")}
    equity := p.TotalEquity(prices)
    // cash 10000 + 10 * 160 = 11600
    assert.Equal(t, "11600.00", equity.String())
}
```

### Risk Rules

```go
func TestMaxDrawdown_Exceeded_RejectsOrder(t *testing.T) {
    rule := risk.NewMaxDrawdownRule(0.10)  // 10% max drawdown
    portfolio := loadPortfolioWithDrawdown(0.15)
    err := rule.Check(portfolio, someOrder)
    assert.ErrorIs(t, err, risk.ErrDrawdownLimitExceeded)
}
```

### Stop Order Trigger

```go
func TestStopBook_StopLoss_TriggersOnPriceDrop(t *testing.T) {
    sb := &order.StopBook{}
    stopOrder := &order.Order{
        ID: "stop1", Side: core.OrderSideSell, Type: core.OrderTypeStop,
        StopPrice: core.NewMoneyOrPanic("90.00"), Quantity: 100,
    }
    sb.Stops = append(sb.Stops, &order.StopOrder{Order: stopOrder})

    activated := sb.CheckTriggers(core.NewMoneyOrPanic("89.50"))
    assert.Len(t, activated, 1)
    assert.Equal(t, core.OrderTypeMarket, activated[0].Type)
}

func TestStopBook_StopLoss_NotTriggeredAboveStop(t *testing.T) {
    sb := &order.StopBook{}
    stopOrder := &order.Order{
        ID: "stop1", Side: core.OrderSideSell, Type: core.OrderTypeStop,
        StopPrice: core.NewMoneyOrPanic("90.00"), Quantity: 100,
    }
    sb.Stops = append(sb.Stops, &order.StopOrder{Order: stopOrder})

    activated := sb.CheckTriggers(core.NewMoneyOrPanic("90.50"))
    assert.Len(t, activated, 0)
}
```

## Integration Tests (Priority: Medium)

### Repository

```go
func TestSQLiteOrderRepository_SaveAndFind(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    repo := sqlite.NewOrderRepository(db)
    o := &order.Order{ID: "o1", Symbol: "AAPL", Quantity: 100, /* ... */}
    err := repo.Save(context.Background(), o)
    assert.NoError(t, err)

    found, err := repo.FindByID(context.Background(), "o1")
    assert.NoError(t, err)
    assert.Equal(t, o.ID, found.ID)
}
```

### CSV Feed

```go
func TestCSVFeed_ReplaysTicks(t *testing.T) {
    // Create temp CSV, feed it, verify events are published in order
}
```

## E2E Tests (Priority: Low)

### Full Simulation

```go
func TestFullBacktest_SampleData(t *testing.T) {
    // 1. Load sample CSV
    // 2. Create portfolio with $100k
    // 3. Run simple moving average strategy
    // 4. Assert final equity is deterministic
    // 5. Assert trade log has expected number of trades
}
```

## Test Infrastructure

```go
// testutil/db.go
package testutil

import (
    "database/sql"
    "testing"
    "papertrading/internal/infra/storage/sqlite"
)

func SetupTestDB(t *testing.T) *sql.DB {
    t.Helper()
    db, err := sql.Open("sqlite", ":memory:")
    if err != nil {
        t.Fatal(err)
    }
    if err := sqlite.Migrate(db); err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { db.Close() })
    return db
}
```

```go
// testutil/must.go
package testutil

import "papertrading/internal/domain/core"

func MustMoney(t *testing.T, s string) core.Money {
    t.Helper()
    m, err := core.NewMoney(s)
    if err != nil {
        t.Fatalf("MustMoney(%q): %v", s, err)
    }
    return m
}
```

## Running Tests

```bash
# Unit tests only (fast, no deps)
go test ./internal/domain/...

# All tests including integration
go test ./...

# With race detector
go test -race ./...

# Coverage
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

## Golden Files for Complex Scenarios

For complex backtest expectations, use golden files:

```go
func TestStrategy_GoldenOutput(t *testing.T) {
    result := runBacktest("testdata/input.csv", myStrategy)
    golden := filepath.Join("testdata", "golden", t.Name()+".json")
    if *update {
        os.WriteFile(golden, json.MarshalIndent(result), 0644)
    }
    expected, _ := os.ReadFile(golden)
    assert.JSONEq(t, string(expected), json.MarshalIndent(result))
}
```

Run with `go test -update` to regenerate golden files when expected behavior changes intentionally.
