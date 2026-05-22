package strategy

import (
	"context"
	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/market"
)

type MovingAverageCross struct {
	name       string
	symbol     core.Symbol
	fastPeriod int
	slowPeriod int
	fastSMA    *SMA
	slowSMA    *SMA
	prevFast   core.Money
	prevSlow   core.Money
	quantity   int64
}

func NewMovingAverageCross(
	name string,
	symbol core.Symbol,
	fastPeriod int,
	slowPeriod int,
	quantity int64,
) *MovingAverageCross {
	return &MovingAverageCross{
		name:       name,
		symbol:     symbol,
		fastPeriod: fastPeriod,
		slowPeriod: slowPeriod,
		fastSMA:    NewSMA(fastPeriod),
		slowSMA:    NewSMA(slowPeriod),
		quantity:   quantity,
	}
}

func (m *MovingAverageCross) Name() string {
	return m.name
}

func (m *MovingAverageCross) OnTick(ctx context.Context, tick market.Tick) (*Signal, error) {
	if tick.Symbol != m.symbol {
		return nil, nil
	}

	fastVal, fastReady := m.fastSMA.Update(tick.Price)
	slowVal, slowReady := m.slowSMA.Update(tick.Price)

	if !fastReady || !slowReady {
		m.prevFast = fastVal
		m.prevSlow = slowVal
		return nil, nil
	}

	var signal *Signal
	if m.prevFast.LessThanOrEqual(m.prevSlow) && fastVal.GreaterThan(slowVal) {
		signal = &Signal{
			Type:      SignalBuy,
			Symbol:    m.symbol,
			Quantity:  m.quantity,
			OrderType: core.OrderTypeMarket,
			Timestamp: tick.Timestamp,
		}
	} else if m.prevFast.GreaterThanOrEqual(m.prevSlow) && fastVal.LessThan(slowVal) {
		signal = &Signal{
			Type:      SignalSell,
			Symbol:    m.symbol,
			Quantity:  m.quantity,
			OrderType: core.OrderTypeMarket,
			Timestamp: tick.Timestamp,
		}
	}

	m.prevFast = fastVal
	m.prevSlow = slowVal
	return signal, nil
}

func (m *MovingAverageCross) Reset() {
	m.fastSMA.Reset()
	m.slowSMA.Reset()
	m.prevFast = core.NewMoneyFromInt(0)
	m.prevSlow = core.NewMoneyFromInt(0)
}

type PriceCrosses struct {
	name       string
	symbol     core.Symbol
	threshold  core.Money
	above      bool
	quantity   int64
	orderType  core.OrderType
}

func NewPriceCrosses(
	name string,
	symbol core.Symbol,
	threshold core.Money,
	quantity int64,
	orderType core.OrderType,
) *PriceCrosses {
	return &PriceCrosses{
		name:      name,
		symbol:    symbol,
		threshold: threshold,
		quantity:  quantity,
		orderType: orderType,
	}
}

func (p *PriceCrosses) Name() string {
	return p.name
}

func (p *PriceCrosses) OnTick(ctx context.Context, tick market.Tick) (*Signal, error) {
	if tick.Symbol != p.symbol {
		return nil, nil
	}

	nowAbove := tick.Price.GreaterThan(p.threshold)
	triggered := false

	if nowAbove && !p.above {
		triggered = true
	} else if !nowAbove && p.above {
		triggered = true
	}

	p.above = nowAbove

	if !triggered {
		return nil, nil
	}

	side := SignalBuy
	if nowAbove {
		side = SignalSell
	}

	return &Signal{
		Type:      side,
		Symbol:    p.symbol,
		Quantity:  p.quantity,
		OrderType: p.orderType,
		Timestamp: tick.Timestamp,
	}, nil
}

func (p *PriceCrosses) Reset() {
	p.above = false
}


