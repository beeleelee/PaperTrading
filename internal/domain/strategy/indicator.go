package strategy

import (
	"github.com/felix/papertrading/internal/domain/core"
)

type SMA struct {
	values []core.Money
	period int
	sum    core.Money
}

func NewSMA(period int) *SMA {
	return &SMA{
		values: make([]core.Money, 0, period),
		period: period,
	}
}

func (s *SMA) Update(price core.Money) (core.Money, bool) {
	s.values = append(s.values, price)
	s.sum = s.sum.Add(price)

	if len(s.values) > s.period {
		removed := s.values[0]
		s.values = s.values[1:]
		s.sum = s.sum.Sub(removed)
	}

	if len(s.values) < s.period {
		return core.NewMoneyFromInt(0), false
	}

	return s.sum.Div(int64(len(s.values))), true
}

func (s *SMA) Reset() {
	s.values = s.values[:0]
	s.sum = core.NewMoneyFromInt(0)
}

type EMA struct {
	period    int
	multiplier float64
	current   core.Money
	initialized bool
}

func NewEMA(period int) *EMA {
	return &EMA{
		period:      period,
		multiplier:  2.0 / float64(period+1),
		initialized: false,
	}
}

func (e *EMA) Update(price core.Money) (core.Money, bool) {
	if !e.initialized {
		e.current = price
		e.initialized = true
		return price, false
	}

	priceFloat, _ := price.Decimal.Float64()
	currentFloat, _ := e.current.Decimal.Float64()
	emaFloat := (priceFloat - currentFloat) * e.multiplier + currentFloat

	e.current = core.NewMoneyFromFloat(emaFloat)
	return e.current, true
}

func (e *EMA) Reset() {
	e.initialized = false
	e.current = core.NewMoneyFromInt(0)
}
