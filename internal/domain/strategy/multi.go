package strategy

import (
	"context"
	"github.com/felix/papertrading/internal/domain/market"
)

type MultiStrategy struct {
	strategies []Strategy
}

func NewMultiStrategy(strategies ...Strategy) *MultiStrategy {
	return &MultiStrategy{strategies: strategies}
}

func (m *MultiStrategy) Name() string {
	return "multi"
}

func (m *MultiStrategy) OnTick(ctx context.Context, tick market.Tick) (*Signal, error) {
	for _, s := range m.strategies {
		sig, err := s.OnTick(ctx, tick)
		if err != nil {
			return nil, err
		}
		if sig != nil {
			return sig, nil
		}
	}
	return nil, nil
}

func (m *MultiStrategy) Reset() {
	for _, s := range m.strategies {
		s.Reset()
	}
}
