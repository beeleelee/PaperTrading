package core

import "errors"

var (
	ErrInvalidOrderType   = errors.New("invalid order type")
	ErrInvalidOrderSide   = errors.New("invalid order side")
	ErrInvalidOrderStatus = errors.New("invalid order status")
	ErrInvalidSymbol      = errors.New("invalid symbol")
	ErrInvalidMoney       = errors.New("invalid money value")
	ErrNegativeQuantity   = errors.New("quantity must be positive")
)
