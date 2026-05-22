package risk

import (
	"errors"
	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/portfolio"
)

var (
	ErrMaxPositionsExceeded  = errors.New("max positions exceeded")
	ErrPositionSizeExceeded  = errors.New("position size exceeds max allowed")
	ErrMaxDrawdownExceeded   = errors.New("max drawdown exceeded")
)

type Constraints struct {
	MaxPositionPct float64
	MaxDrawdownPct float64
	MaxPositions   int
}

func (c Constraints) ValidateOrder(pf *portfolio.Portfolio, symbol core.Symbol, side core.OrderSide, price core.Money, qty int64, prices map[core.Symbol]core.Money) error {
	if side == core.OrderSideSell {
		return nil
	}

	if c.MaxPositions > 0 && pf.PositionCount() >= c.MaxPositions {
		return ErrMaxPositionsExceeded
	}

	if c.MaxPositionPct > 0 {
		equity := pf.TotalEquity(prices)
		cost := price.Mul(qty)
		maxCost := equity.Mul(int64(c.MaxPositionPct * 100)).Div(10000)
		if cost.GreaterThan(maxCost) {
			return ErrPositionSizeExceeded
		}
	}

	return nil
}

func (c Constraints) ValidateDrawdown(currentEquity, peakEquity core.Money) error {
	if c.MaxDrawdownPct <= 0 {
		return nil
	}
	if peakEquity.IsZero() {
		return nil
	}
	drawdown := peakEquity.Sub(currentEquity)
	maxDrawdown := peakEquity.Mul(int64(c.MaxDrawdownPct * 100)).Div(10000)
	if drawdown.GreaterThan(maxDrawdown) {
		return ErrMaxDrawdownExceeded
	}
	return nil
}
