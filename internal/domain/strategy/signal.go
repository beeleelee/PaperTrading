package strategy

import (
	"time"
	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/order"
)

type SignalType int

const (
	SignalBuy  SignalType = 1
	SignalSell SignalType = 2
)

type Signal struct {
	Type      SignalType
	Symbol    core.Symbol
	Quantity  int64
	Price     core.Money
	OrderType core.OrderType
	Timestamp time.Time
}

func (st SignalType) String() string {
	switch st {
	case SignalBuy:
		return "buy"
	case SignalSell:
		return "sell"
	default:
		return "unknown"
	}
}

func (st SignalType) ToOrderSide() core.OrderSide {
	if st == SignalSell {
		return core.OrderSideSell
	}
	return core.OrderSideBuy
}

func (s Signal) ToOrder(portfolioID string, now time.Time) (*order.Order, error) {
	side := core.OrderSideBuy
	if s.Type == SignalSell {
		side = core.OrderSideSell
	}

	var price, stopPrice core.Money
	if s.OrderType == core.OrderTypeLimit {
		price = s.Price
	}
	if s.OrderType == core.OrderTypeStop || s.OrderType == core.OrderTypeStopLimit {
		stopPrice = s.Price
	}

	return order.NewOrder(
		order.OrderID(newOrderID(s.Symbol, now)),
		portfolioID,
		s.Symbol,
		side,
		s.OrderType,
		price,
		stopPrice,
		s.Quantity,
		now,
	)
}

var signalIDCounter int64

func newOrderID(symbol core.Symbol, t time.Time) string {
	signalIDCounter++
	return string(symbol) + "_" + t.Format("20060102150405") + "_" + itoa(signalIDCounter)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
