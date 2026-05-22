# Matching Engine Design

The matching engine is the **core domain** of a paper trading system. It determines simulation fidelity.

## Order Book Structure

```go
package order

import (
    "container/list"
    "sort"
    "papertrading/internal/domain/core"
)

type PriceLevel struct {
    Price    core.Money
    Orders   []*Order
}

type OrderBook struct {
    Symbol    core.Symbol
    Bids      []PriceLevel  // Buy orders, highest price first
    Asks      []PriceLevel  // Sell orders, lowest price first
}

// ByPriceDesc and ByPriceAsc for sorting
type ByPriceDesc []PriceLevel
func (a ByPriceDesc) Len() int           { return len(a) }
func (a ByPriceDesc) Less(i, j int) bool { return a[i].Price.GreaterThan(a[j].Price) }
func (a ByPriceDesc) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

type ByPriceAsc []PriceLevel
func (a ByPriceAsc) Len() int            { return len(a) }
func (a ByPriceAsc) Less(i, j int) bool  { return a[i].Price.LessThan(a[j].Price) }
func (a ByPriceAsc) Swap(i, j int)       { a[i], a[j] = a[j], a[i] }
```

## Matching Algorithm

### Market Order

```
Market Buy  → match against Asks (lowest price first) until filled
Market Sell → match against Bids (highest price first) until filled
```

```go
func (m *MatchingEngine) matchMarket(order *Order, book *OrderBook) ([]Fill, error) {
    var fills []Fill
    remaining := order.RemainingQty()

    levels := book.Asks  // for buy; would be book.Bids for sell
    if order.Side == core.OrderSideSell {
        levels = book.Bids
    }

    for i := 0; i < len(levels) && remaining > 0; i++ {
        for _, restingOrder := range levels[i].Orders {
            if remaining <= 0 {
                break
            }
            matchQty := min(remaining, restingOrder.RemainingQty())
            if matchQty <= 0 {
                continue
            }
            fill := Fill{
                ID:        generateFillID(),
                OrderID:   order.ID,
                Price:     levels[i].Price,
                Quantity:  matchQty,
                Timestamp: time.Now(),
            }
            fills = append(fills, fill)
            remaining -= matchQty
        }
    }

    if remaining > 0 {
        return fills, ErrNoLiquidity
    }
    return fills, nil
}
```

### Limit Order

```
Limit Buy @ $100:
  1. Match against Asks with price ≤ $100 (best price first)
  2. If partially filled, rest sits in Bids @ $100
```

```go
func (m *MatchingEngine) matchLimit(order *Order, book *OrderBook) ([]Fill, error) {
    var fills []Fill
    remaining := order.RemainingQty()

    // Determine which side to cross
    levels := book.Asks   // buy limit crosses the ask side
    priceCheck := func(levelPrice core.Money) bool {
        return levelPrice.LessThanOrEqual(order.Price) // buy: only cross if ask ≤ our limit
    }
    if order.Side == core.OrderSideSell {
        levels = book.Bids
        priceCheck = func(levelPrice core.Money) bool {
            return levelPrice.GreaterThanOrEqual(order.Price) // sell: only cross if bid ≥ our limit
        }
    }

    for i := 0; i < len(levels) && remaining > 0; i++ {
        if !priceCheck(levels[i].Price) {
            break  // no more crossing possible
        }
        for _, restingOrder := range levels[i].Orders {
            if remaining <= 0 {
                break
            }
            matchQty := min(remaining, restingOrder.RemainingQty())
            if matchQty <= 0 {
                continue
            }
            fill := Fill{
                ID:        generateFillID(),
                OrderID:   order.ID,
                Price:     levels[i].Price,
                Quantity:  matchQty,
                Timestamp: time.Now(),
            }
            fills = append(fills, fill)
            remaining -= matchQty
        }
    }

    // If not fully filled, add to order book
    if remaining > 0 {
        book.addOrder(order)
    }

    return fills, nil
}
```

### Stop Order

Stop orders are **not** visible in the order book. They sit in a separate list and are **triggered** when the market price crosses the stop price:

```
Stop Loss Sell @ $90:
  - Monitor last traded price
  - When price ≤ $90, the stop becomes a MARKET order to sell
```

```go
type StopOrder struct {
    Order     *Order
    Triggered bool
}

type StopBook struct {
    Stops []*StopOrder
}

// CheckTriggers scans stops and returns orders that should be activated.
func (sb *StopBook) CheckTriggers(lastPrice core.Money) []*Order {
    var activated []*Order
    for _, so := range sb.Stops {
        if so.Triggered {
            continue
        }
        triggered := false
        switch so.Order.Side {
        case core.OrderSideSell:
            triggered = lastPrice.LessThanOrEqual(so.Order.StopPrice)
        case core.OrderSideBuy:
            triggered = lastPrice.GreaterThanOrEqual(so.Order.StopPrice)
        }
        if triggered {
            so.Triggered = true
            // Convert stop to market order
            marketOrder := *so.Order
            marketOrder.Type = core.OrderTypeMarket
            activated = append(activated, &marketOrder)
        }
    }
    return activated
}
```

## Fill Simulation

Real markets don't fill every order instantly. Add configurable fill probability:

```go
type FillConfig struct {
    // Probability that a limit order gets filled in a given tick
    // when the price condition is met. 1.0 = always fill (optimistic).
    LimitFillProbability float64

    // Slippage model: fills happen at order book price ± slippage
    // e.g., 0.001 = 0.1% slippage on market orders
    SlippageBps float64

    // Commission per fill (fixed + percentage)
    CommissionFixed    core.Money
    CommissionRateBps  float64
}

func (cfg *FillConfig) ShouldFill(rng *rand.Rand) bool {
    return rng.Float64() < cfg.LimitFillProbability
}

func (cfg *FillConfig) ApplySlippage(price core.Money, side core.OrderSide) core.Money {
    slippage := price.Mul(int64(cfg.SlippageBps * 10000)).Div(10000)
    if side == core.OrderSideBuy {
        return price.Add(slippage)  // buys fill at slightly higher price
    }
    return price.Sub(slippage)     // sells fill at slightly lower price
}
```

## Lifecycle Integration

```
                        ┌──────────┐
  TickReceived ────────▶│  StopBook │─── activated orders ──┐
                        └──────────┘                        │
                                                            ▼
                        ┌──────────────────┐         ┌──────────┐
                        │   MatchingEngine  │◀────────│  Order   │
                        │  .Match(order,   │         │ Incoming │
                        │   orderBook)     │         └──────────┘
                        └────────┬─────────┘
                                 │ []Fill
                                 ▼
                        ┌──────────────────┐
                        │  ApplyFill to    │
                        │  Order + update  │
                        │  Position/Cash   │
                        └────────┬─────────┘
                                 │ OrderFilled event
                                 ▼
                        ┌──────────────────┐
                        │  Update OrderBook │
                        │  (remove filled   │
                        │   resting orders) │
                        └──────────────────┘
```

## Order Book Depth

For realistic simulation, the order book should have configurable depth:

```go
func NewOrderBookFromL2(symbol core.Symbol, l2 L2Snapshot) *OrderBook {
    book := &OrderBook{Symbol: symbol}
    for _, bid := range l2.Bids {
        book.Bids = append(book.Bids, PriceLevel{
            Price:  bid.Price,
            Orders: []*Order{createDummyOrder(core.OrderSideBuy, bid.Price, bid.Size)},
        })
    }
    for _, ask := range l2.Asks {
        book.Asks = append(book.Asks, PriceLevel{
            Price:  ask.Price,
            Orders: []*Order{createDummyOrder(core.OrderSideSell, ask.Price, ask.Size)},
        })
    }
    // Sort bids high→low, asks low→high
    sort.Sort(ByPriceDesc(book.Bids))
    sort.Sort(ByPriceAsc(book.Asks))
    return book
}
```

## Edge Cases

| Scenario | Handling |
|---|---|
| **Partial fill at multiple price levels** | Each level produces a separate Fill record |
| **Order book empty** | Market order returns `ErrNoLiquidity` |
| **Self-trade** | Not relevant for paper trading (no real orders) |
| **Fill exceeds order quantity** | Prevented by `Order.ApplyFill` invariant |
| **Stop triggered but no liquidity** | Stop becomes market order → gets `ErrNoLiquidity` → order rejected |
| **Concurrent access** | Matching engine processes one tick at a time (no parallelism needed in simulation) |
