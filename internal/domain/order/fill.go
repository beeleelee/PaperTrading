package order

import (
	"time"
	"github.com/felix/papertrading/internal/domain/core"
)

type FillID string

type Fill struct {
	ID        FillID
	OrderID   OrderID
	Price     core.Money
	Quantity  int64
	Commission core.Money
	Timestamp time.Time
}
