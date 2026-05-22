package portfolio

import (
	"time"
	"github.com/felix/papertrading/internal/domain/core"
)

type PortfolioID string

type Portfolio struct {
	ID        PortfolioID
	Cash      core.Money
	Positions map[core.Symbol]*Position
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewPortfolio(id PortfolioID, initialCash core.Money, now time.Time) *Portfolio {
	return &Portfolio{
		ID:        id,
		Cash:      initialCash,
		Positions: make(map[core.Symbol]*Position),
		CreatedAt: now,
		UpdatedAt: now,
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

func (p *Portfolio) RealizedPnL() core.Money {
	return core.NewMoneyFromInt(0)
}

func (p *Portfolio) UnrealizedPnL(prices map[core.Symbol]core.Money) core.Money {
	total := core.NewMoneyFromInt(0)
	for symbol, pos := range p.Positions {
		if price, ok := prices[symbol]; ok {
			total = total.Add(pos.UnrealizedPnL(price))
		}
	}
	return total
}

func (p *Portfolio) Deposit(amount core.Money, now time.Time) {
	p.Cash = p.Cash.Add(amount)
	p.UpdatedAt = now
}

func (p *Portfolio) Withdraw(amount core.Money, now time.Time) error {
	if amount.GreaterThan(p.Cash) {
		return ErrInsufficientCash
	}
	p.Cash = p.Cash.Sub(amount)
	p.UpdatedAt = now
	return nil
}

func (p *Portfolio) OpenPosition(symbol core.Symbol, price core.Money, quantity int64, now time.Time) error {
	if _, exists := p.Positions[symbol]; exists {
		return ErrDuplicatePosition
	}
	cost := price.Mul(quantity)
	if cost.GreaterThan(p.Cash) {
		return ErrInsufficientCash
	}
	pos, err := NewPosition(symbol, price, quantity)
	if err != nil {
		return err
	}
	p.Positions[symbol] = pos
	p.Cash = p.Cash.Sub(cost)
	p.UpdatedAt = now
	return nil
}

func (p *Portfolio) IncreasePosition(symbol core.Symbol, price core.Money, quantity int64, now time.Time) error {
	pos, exists := p.Positions[symbol]
	if !exists {
		return ErrPositionNotFound
	}
	cost := price.Mul(quantity)
	if cost.GreaterThan(p.Cash) {
		return ErrInsufficientCash
	}
	totalQty := pos.Quantity + quantity
	totalCost := pos.AvgEntryPrice.Mul(pos.Quantity).Add(price.Mul(quantity))
	pos.AvgEntryPrice = totalCost.Div(totalQty)
	pos.Quantity = totalQty
	p.Cash = p.Cash.Sub(cost)
	p.UpdatedAt = now
	return nil
}

func (p *Portfolio) ReducePosition(symbol core.Symbol, price core.Money, quantity int64, now time.Time) error {
	pos, exists := p.Positions[symbol]
	if !exists {
		return ErrPositionNotFound
	}
	if quantity > pos.Quantity {
		return ErrInsufficientPosition
	}
	revenue := price.Mul(quantity)
	p.Cash = p.Cash.Add(revenue)

	if quantity == pos.Quantity {
		delete(p.Positions, symbol)
	} else {
		pos.Quantity -= quantity
	}
	p.UpdatedAt = now
	return nil
}

func (p *Portfolio) ClosePosition(symbol core.Symbol, price core.Money, now time.Time) error {
	pos, exists := p.Positions[symbol]
	if !exists {
		return ErrPositionNotFound
	}
	return p.ReducePosition(symbol, price, pos.Quantity, now)
}

func (p *Portfolio) HasPosition(symbol core.Symbol) bool {
	_, exists := p.Positions[symbol]
	return exists
}

func (p *Portfolio) PositionCount() int {
	return len(p.Positions)
}
