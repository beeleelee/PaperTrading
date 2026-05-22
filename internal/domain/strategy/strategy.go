package strategy

import (
	"context"
	"github.com/felix/papertrading/internal/domain/market"
)

type Strategy interface {
	Name() string
	OnTick(ctx context.Context, tick market.Tick) (*Signal, error)
	Reset()
}
