package market

import (
	"context"
	"time"
	"github.com/felix/papertrading/internal/domain/core"
)

type Tick struct {
	Symbol    core.Symbol
	Price     core.Money
	Volume    int64
	Timestamp time.Time
}

type Feed interface {
	Stream(ctx context.Context, ticks chan<- Tick) error
}
