package order

import (
	"time"
	"github.com/felix/papertrading/internal/domain/core"
)

type OrderID string

type Order struct {
	ID         OrderID
	PortfolioID string
	Symbol     core.Symbol
	Side       core.OrderSide
	Type      core.OrderType
	Status     core.OrderStatus
	Price      core.Money
	StopPrice  core.Money
	Quantity   int64
	FilledQty  int64
	Fills      []Fill
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NewOrder(
	id OrderID,
	portfolioID string,
	symbol core.Symbol,
	side core.OrderSide,
	orderType core.OrderType,
	price core.Money,
	stopPrice core.Money,
	quantity int64,
	now time.Time,
) (*Order, error) {
	if !symbol.Valid() {
		return nil, core.ErrInvalidSymbol
	}
	if !side.Valid() {
		return nil, core.ErrInvalidOrderSide
	}
	if !orderType.Valid() {
		return nil, core.ErrInvalidOrderType
	}
	if quantity <= 0 {
		return nil, core.ErrNegativeQuantity
	}
	return &Order{
		ID:          id,
		PortfolioID: portfolioID,
		Symbol:      symbol,
		Side:        side,
		Type:       orderType,
		Status:      core.OrderStatusCreated,
		Price:       price,
		StopPrice:   stopPrice,
		Quantity:    quantity,
		FilledQty:   0,
		Fills:       nil,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (o *Order) RemainingQty() int64 {
	return o.Quantity - o.FilledQty
}

func (o *Order) IsFilled() bool {
	return o.Status == core.OrderStatusFilled
}

func (o *Order) IsActive() bool {
	return o.Status == core.OrderStatusCreated ||
		o.Status == core.OrderStatusSubmitted ||
		o.Status == core.OrderStatusPartiallyFilled
}

func (o *Order) Submit(now time.Time) error {
	if !o.Status.CanTransitionTo(core.OrderStatusSubmitted) {
		return ErrInvalidStatusTransition
	}
	o.Status = core.OrderStatusSubmitted
	o.UpdatedAt = now
	return nil
}

func (o *Order) ApplyFill(f Fill, now time.Time) error {
	if !o.IsActive() {
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
	o.UpdatedAt = now
	return nil
}

func (o *Order) Cancel(now time.Time) error {
	if !o.Status.CanTransitionTo(core.OrderStatusCancelled) {
		return ErrInvalidStatusTransition
	}
	o.Status = core.OrderStatusCancelled
	o.UpdatedAt = now
	return nil
}

func (o *Order) Reject(now time.Time) error {
	if !o.Status.CanTransitionTo(core.OrderStatusRejected) {
		return ErrInvalidStatusTransition
	}
	o.Status = core.OrderStatusRejected
	o.UpdatedAt = now
	return nil
}
