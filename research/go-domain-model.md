# Go Domain Model

## Money Value Object

Use `shopspring/decimal` for all monetary values. Never use `float64`.

```go
package core

import "github.com/shopspring/decimal"

type Money struct {
    decimal.Decimal
}

func NewMoney(value string) (Money, error) {
    d, err := decimal.NewFromString(value)
    return Money{d}, err
}

func (m Money) Add(other Money) Money     { return Money{m.Decimal.Add(other.Decimal)} }
func (m Money) Sub(other Money) Money     { return Money{m.Decimal.Sub(other.Decimal)} }
func (m Money) Mul(n int64) Money         { return Money{m.Decimal.Mul(decimal.NewFromInt(n))} }
func (m Money) IsZero() bool              { return m.Decimal.IsZero() }
func (m Money) IsNegative() bool          { return m.Decimal.IsNegative() }
```

## Symbol Value Object

```go
package core

type Symbol string

const (
    UnknownSymbol Symbol = ""
)

func (s Symbol) Valid() bool { return s != UnknownSymbol }
```

## Order Type / Status Enums

```go
package core

type OrderType int

const (
    OrderTypeUnknown   OrderType = 0
    OrderTypeMarket    OrderType = 1
    OrderTypeLimit     OrderType = 2
    OrderTypeStop      OrderType = 3
    OrderTypeStopLimit OrderType = 4
)

type OrderSide int

const (
    OrderSideUnknown OrderSide = 0
    OrderSideBuy     OrderSide = 1
    OrderSideSell    OrderSide = 2
)

type OrderStatus int

const (
    OrderStatusUnknown     OrderStatus = 0
    OrderStatusCreated     OrderStatus = 1
    OrderStatusSubmitted   OrderStatus = 2
    OrderStatusPartiallyFilled OrderStatus = 3
    OrderStatusFilled      OrderStatus = 4
    OrderStatusCancelled   OrderStatus = 5
    OrderStatusRejected    OrderStatus = 6
)

func (s OrderStatus) CanTransitionTo(target OrderStatus) bool {
    transitions := map[OrderStatus][]OrderStatus{
        OrderStatusCreated:          {OrderStatusSubmitted, OrderStatusCancelled, OrderStatusRejected},
        OrderStatusSubmitted:        {OrderStatusPartiallyFilled, OrderStatusFilled, OrderStatusCancelled, OrderStatusRejected},
        OrderStatusPartiallyFilled:  {OrderStatusFilled, OrderStatusCancelled, OrderStatusPartiallyFilled},
        OrderStatusFilled:           {},
        OrderStatusCancelled:        {},
        OrderStatusRejected:         {},
    }
    allowed, ok := transitions[s]
    if !ok {
        return false
    }
    for _, t := range allowed {
        if t == target {
            return true
        }
    }
    return false
}
```

## Order Aggregate

```go
package order

import (
    "time"
    "papertrading/internal/domain/core"
)

type OrderID string

type Order struct {
    ID        OrderID
    Symbol    core.Symbol
    Side      core.OrderSide
    Type     core.OrderType
    Status    core.OrderStatus
    Price     core.Money      // zero for MARKET orders
    StopPrice core.Money      // zero for non-STOP orders
    Quantity  int64
    FilledQty int64
    Fills     []Fill
    CreatedAt time.Time
    UpdatedAt time.Time
}

func (o *Order) RemainingQty() int64 { return o.Quantity - o.FilledQty }

func (o *Order) IsFilled() bool { return o.Status == core.OrderStatusFilled }

// ApplyFill updates the order with a new fill, enforcing invariants.
func (o *Order) ApplyFill(f Fill) error {
    if o.Status != core.OrderStatusSubmitted && o.Status != core.OrderStatusPartiallyFilled {
        return ErrOrderNotActive
    }
    if o.FilledQty+f.Quantity > o.Quantity {
        return ErrFillExceedsOrderQty
    }
    o.Fills = append(o.Fills, f)
    o.FilledQty += f.Quantity
    if o.FilledQty == o.Quantity {
        o.Status = core.OrderStatusFilled
    } else {
        o.Status = core.OrderStatusPartiallyFilled
    }
    o.UpdatedAt = time.Now()
    return nil
}
```

## Fill Value Object

```go
package order

import (
    "time"
    "papertrading/internal/domain/core"
)

type FillID string

type Fill struct {
    ID        FillID
    OrderID   OrderID
    Price     core.Money
    Quantity  int64
    Timestamp time.Time
}
```

## Portfolio Aggregate Root

```go
package portfolio

import (
    "time"
    "papertrading/internal/domain/core"
)

type PortfolioID string

type Portfolio struct {
    ID         PortfolioID
    Cash       core.Money
    Positions  map[core.Symbol]*Position
    CreatedAt  time.Time
    UpdatedAt  time.Time
}

func NewPortfolio(id PortfolioID, initialCash core.Money) *Portfolio {
    return &Portfolio{
        ID:        id,
        Cash:      initialCash,
        Positions: make(map[core.Symbol]*Position),
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }
}

func (p *Portfolio) TotalEquity(prices map[core.Symbol]core.Money) core.Money {
    total := p.Cash
    for symbol, pos := range p.Positions {
        if price, ok := prices[symbol]; ok {
            total = total.Add(pos.MarketValue(price))
        }
    }
    return total
}

func (p *Portfolio) RealizedPnL() core.Money { ... }
func (p *Portfolio) UnrealizedPnL(prices map[core.Symbol]core.Money) core.Money { ... }
```

## Position Entity

```go
package portfolio

import "papertrading/internal/domain/core"

type Position struct {
    Symbol         core.Symbol
    Quantity       int64
    AvgEntryPrice  core.Money
}

func (pos *Position) MarketValue(currentPrice core.Money) core.Money {
    return currentPrice.Mul(pos.Quantity)
}

func (pos *Position) UnrealizedPnL(currentPrice core.Money) core.Money {
    return currentPrice.Sub(pos.AvgEntryPrice).Mul(pos.Quantity)
}
```

## Domain Service: MatchingEngine

Defined in `internal/domain/order/matching.go`:

```go
package order

import "papertrading/internal/domain/core"

type MatchingEngine struct {}

func (m *MatchingEngine) Match(order *Order, book *OrderBook) ([]Fill, error) {
    switch order.Type {
    case core.OrderTypeMarket:
        return m.matchMarket(order, book)
    case core.OrderTypeLimit:
        return m.matchLimit(order, book)
    // ...
    }
}
```

## Domain Repository Interfaces

Defined alongside the aggregate, not in infra:

```go
package order

import "context"

type Repository interface {
    Save(ctx context.Context, order *Order) error
    FindByID(ctx context.Context, id OrderID) (*Order, error)
    FindByStatus(ctx context.Context, status core.OrderStatus) ([]*Order, error)
}
```

```go
package portfolio

import "context"

type Repository interface {
    Save(ctx context.Context, p *Portfolio) error
    FindByID(ctx context.Context, id PortfolioID) (*Portfolio, error)
}
```
