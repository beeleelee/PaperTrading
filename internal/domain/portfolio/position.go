package portfolio

import (
	"github.com/felix/papertrading/internal/domain/core"
)

type Position struct {
	Symbol        core.Symbol
	Quantity      int64
	AvgEntryPrice core.Money
}

func NewPosition(symbol core.Symbol, price core.Money, quantity int64) (*Position, error) {
	if !symbol.Valid() {
		return nil, core.ErrInvalidSymbol
	}
	if quantity <= 0 {
		return nil, core.ErrNegativeQuantity
	}
	return &Position{
		Symbol:        symbol,
		Quantity:      quantity,
		AvgEntryPrice: price,
	}, nil
}

func (p *Position) MarketValue(currentPrice core.Money) core.Money {
	return currentPrice.Mul(p.Quantity)
}

func (p *Position) UnrealizedPnL(currentPrice core.Money) core.Money {
	return currentPrice.Sub(p.AvgEntryPrice).Mul(p.Quantity)
}

func (p *Position) CostBasis() core.Money {
	return p.AvgEntryPrice.Mul(p.Quantity)
}
