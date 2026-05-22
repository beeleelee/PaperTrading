package core

type OrderType int

const (
	OrderTypeUnknown   OrderType = 0
	OrderTypeMarket    OrderType = 1
	OrderTypeLimit     OrderType = 2
	OrderTypeStop      OrderType = 3
	OrderTypeStopLimit OrderType = 4
)

func (t OrderType) Valid() bool {
	return t >= OrderTypeMarket && t <= OrderTypeStopLimit
}

func (t OrderType) String() string {
	switch t {
	case OrderTypeMarket:
		return "market"
	case OrderTypeLimit:
		return "limit"
	case OrderTypeStop:
		return "stop"
	case OrderTypeStopLimit:
		return "stop_limit"
	default:
		return "unknown"
	}
}

type OrderSide int

const (
	OrderSideUnknown OrderSide = 0
	OrderSideBuy     OrderSide = 1
	OrderSideSell    OrderSide = 2
)

func (s OrderSide) Valid() bool {
	return s >= OrderSideBuy && s <= OrderSideSell
}

func (s OrderSide) String() string {
	switch s {
	case OrderSideBuy:
		return "buy"
	case OrderSideSell:
		return "sell"
	default:
		return "unknown"
	}
}

func (s OrderSide) Inverse() OrderSide {
	switch s {
	case OrderSideBuy:
		return OrderSideSell
	case OrderSideSell:
		return OrderSideBuy
	default:
		return OrderSideUnknown
	}
}

type OrderStatus int

const (
	OrderStatusUnknown          OrderStatus = 0
	OrderStatusCreated          OrderStatus = 1
	OrderStatusSubmitted        OrderStatus = 2
	OrderStatusPartiallyFilled  OrderStatus = 3
	OrderStatusFilled           OrderStatus = 4
	OrderStatusCancelled        OrderStatus = 5
	OrderStatusRejected         OrderStatus = 6
)

func (s OrderStatus) Valid() bool {
	return s >= OrderStatusCreated && s <= OrderStatusRejected
}

func (s OrderStatus) CanTransitionTo(target OrderStatus) bool {
	transitions := map[OrderStatus][]OrderStatus{
		OrderStatusCreated:         {OrderStatusSubmitted, OrderStatusCancelled, OrderStatusRejected},
		OrderStatusSubmitted:       {OrderStatusPartiallyFilled, OrderStatusFilled, OrderStatusCancelled, OrderStatusRejected},
		OrderStatusPartiallyFilled: {OrderStatusFilled, OrderStatusCancelled, OrderStatusPartiallyFilled},
		OrderStatusFilled:          {},
		OrderStatusCancelled:       {},
		OrderStatusRejected:        {},
	}
	allowed, ok := transitions[s]
	if !ok {
		return false
	}
	for _, t := range allowed {
		if t == target {
			return true
		}
	}
	return false
}

func (s OrderStatus) String() string {
	switch s {
	case OrderStatusCreated:
		return "created"
	case OrderStatusSubmitted:
		return "submitted"
	case OrderStatusPartiallyFilled:
		return "partially_filled"
	case OrderStatusFilled:
		return "filled"
	case OrderStatusCancelled:
		return "cancelled"
	case OrderStatusRejected:
		return "rejected"
	default:
		return "unknown"
	}
}
