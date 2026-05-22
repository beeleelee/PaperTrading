package event

import (
	"time"
	"github.com/felix/papertrading/internal/domain/core"
)

type Event interface {
	EventName() string
	Timestamp() time.Time
}

type TickReceived struct {
	Symbol    core.Symbol
	Price     core.Money
	Volume    int64
	At        time.Time
}

func (e TickReceived) EventName() string   { return "market.tick_received" }
func (e TickReceived) Timestamp() time.Time { return e.At }

type OrderSubmitted struct {
	OrderID     string
	PortfolioID string
	Symbol      core.Symbol
	Side        core.OrderSide
	OrderType  core.OrderType
	Price       core.Money
	StopPrice   core.Money
	Quantity    int64
	At          time.Time
}

func (e OrderSubmitted) EventName() string   { return "order.submitted" }
func (e OrderSubmitted) Timestamp() time.Time { return e.At }

type OrderFilled struct {
	OrderID     string
	FillID      string
	PortfolioID string
	Symbol      core.Symbol
	Side        core.OrderSide
	Price       core.Money
	Quantity    int64
	Commission  core.Money
	At          time.Time
}

func (e OrderFilled) EventName() string   { return "order.filled" }
func (e OrderFilled) Timestamp() time.Time { return e.At }

type OrderCancelled struct {
	OrderID     string
	PortfolioID string
	At          time.Time
}

func (e OrderCancelled) EventName() string   { return "order.cancelled" }
func (e OrderCancelled) Timestamp() time.Time { return e.At }

type OrderRejected struct {
	OrderID     string
	PortfolioID string
	Reason      string
	At          time.Time
}

func (e OrderRejected) EventName() string   { return "order.rejected" }
func (e OrderRejected) Timestamp() time.Time { return e.At }

type PositionOpened struct {
	PortfolioID string
	Symbol      core.Symbol
	Quantity    int64
	Price       core.Money
	At          time.Time
}

func (e PositionOpened) EventName() string   { return "portfolio.position_opened" }
func (e PositionOpened) Timestamp() time.Time { return e.At }

type PositionClosed struct {
	PortfolioID string
	Symbol      core.Symbol
	Quantity    int64
	Price       core.Money
	PnL         core.Money
	At          time.Time
}

func (e PositionClosed) EventName() string   { return "portfolio.position_closed" }
func (e PositionClosed) Timestamp() time.Time { return e.At }
