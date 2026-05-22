package order

import "errors"

var (
	ErrOrderNotActive        = errors.New("order is not active")
	ErrFillExceedsOrderQty   = errors.New("fill quantity exceeds remaining order quantity")
	ErrInvalidPrice          = errors.New("invalid price for order type")
	ErrNoLiquidity           = errors.New("no liquidity available to fill order")
	ErrOrderNotFound         = errors.New("order not found")
	ErrInvalidStatusTransition = errors.New("invalid order status transition")
)
