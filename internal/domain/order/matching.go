package order

import (
	"math/rand"
	"sort"
	"time"
	"github.com/felix/papertrading/internal/domain/core"
)

type PriceLevel struct {
	Price  core.Money
	Orders []*Order
}

type OrderBook struct {
	Symbol core.Symbol
	Bids   []PriceLevel
	Asks   []PriceLevel
}

type FillConfig struct {
	LimitFillProbability float64
	SlippageBps          float64
	CommissionFixed      core.Money
	CommissionRateBps    float64
}

func DefaultFillConfig() FillConfig {
	return FillConfig{
		LimitFillProbability: 0.5,
		SlippageBps:          0.0,
		CommissionFixed:      core.NewMoneyFromInt(0),
		CommissionRateBps:    0.0,
	}
}

type MatchingEngine struct {
	rng    *rand.Rand
	config FillConfig
}

func NewMatchingEngine(config FillConfig) *MatchingEngine {
	return &MatchingEngine{
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
		config: config,
	}
}

func NewMatchingEngineWithSeed(config FillConfig, seed int64) *MatchingEngine {
	return &MatchingEngine{
		rng:    rand.New(rand.NewSource(seed)),
		config: config,
	}
}

func (m *MatchingEngine) Match(order *Order, book *OrderBook) ([]Fill, error) {
	switch order.Type {
	case core.OrderTypeMarket:
		return m.matchMarket(order, book)
	case core.OrderTypeLimit:
		return m.matchLimit(order, book)
	default:
		return nil, core.ErrInvalidOrderType
	}
}

func (m *MatchingEngine) matchMarket(order *Order, book *OrderBook) ([]Fill, error) {
	var fills []Fill
	remaining := order.RemainingQty()

	levels := book.Asks
	if order.Side == core.OrderSideSell {
		levels = book.Bids
	}

	for i := 0; i < len(levels) && remaining > 0; i++ {
		for _, restingOrder := range levels[i].Orders {
			if remaining <= 0 {
				break
			}
			restRemaining := restingOrder.RemainingQty()
			if restRemaining <= 0 {
				continue
			}
			matchQty := min64(remaining, restRemaining)
			price := m.config.ApplySlippage(levels[i].Price, order.Side)
			commission := m.config.CalcCommission(price.Mul(matchQty))
			fill := Fill{
				ID:         FillID(newFillID()),
				OrderID:    order.ID,
				Price:      price,
				Quantity:   matchQty,
				Commission: commission,
				Timestamp:  time.Now(),
			}
			fills = append(fills, fill)
			remaining -= matchQty
		}
	}

	if remaining > 0 {
		return fills, ErrNoLiquidity
	}
	return fills, nil
}

func (m *MatchingEngine) matchLimit(order *Order, book *OrderBook) ([]Fill, error) {
	var fills []Fill
	remaining := order.RemainingQty()

	levels := book.Asks
	priceCheck := func(levelPrice core.Money) bool {
		return levelPrice.LessThanOrEqual(order.Price)
	}
	if order.Side == core.OrderSideSell {
		levels = book.Bids
		priceCheck = func(levelPrice core.Money) bool {
			return levelPrice.GreaterThanOrEqual(order.Price)
		}
	}

	for i := 0; i < len(levels) && remaining > 0; i++ {
		if !priceCheck(levels[i].Price) {
			break
		}
		for _, restingOrder := range levels[i].Orders {
			if remaining <= 0 {
				break
			}
			restRemaining := restingOrder.RemainingQty()
			if restRemaining <= 0 {
				continue
			}
			if !m.ShouldFill() {
				continue
			}
			matchQty := min64(remaining, restRemaining)
			price := levels[i].Price
			commission := m.config.CalcCommission(price.Mul(matchQty))
			fill := Fill{
				ID:         FillID(newFillID()),
				OrderID:    order.ID,
				Price:      price,
				Quantity:   matchQty,
				Commission: commission,
				Timestamp:  time.Now(),
			}
			fills = append(fills, fill)
			remaining -= matchQty
		}
	}

	return fills, nil
}

func (m *MatchingEngine) ShouldFill() bool {
	return m.rng.Float64() < m.config.LimitFillProbability
}

func (cfg FillConfig) ApplySlippage(price core.Money, side core.OrderSide) core.Money {
	if cfg.SlippageBps == 0 {
		return price
	}
	multiplier := int64(cfg.SlippageBps * 10000)
	slippage := price.Mul(multiplier).Div(10000)
	if side == core.OrderSideBuy {
		return price.Add(slippage)
	}
	return price.Sub(slippage)
}

func (cfg FillConfig) CalcCommission(tradeValue core.Money) core.Money {
	commission := cfg.CommissionFixed
	if cfg.CommissionRateBps > 0 {
		rate := int64(cfg.CommissionRateBps * 10000)
		commission = commission.Add(tradeValue.Mul(rate).Div(10000))
	}
	return commission
}

type ByPriceDesc []PriceLevel

func (a ByPriceDesc) Len() int           { return len(a) }
func (a ByPriceDesc) Less(i, j int) bool { return a[i].Price.GreaterThan(a[j].Price) }
func (a ByPriceDesc) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

type ByPriceAsc []PriceLevel

func (a ByPriceAsc) Len() int           { return len(a) }
func (a ByPriceAsc) Less(i, j int) bool { return a[i].Price.LessThan(a[j].Price) }
func (a ByPriceAsc) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func (b *OrderBook) AddOrder(o *Order) {
	level := PriceLevel{Price: o.Price, Orders: []*Order{o}}
	if o.Side == core.OrderSideBuy {
		b.Bids = append(b.Bids, level)
		sort.Sort(ByPriceDesc(b.Bids))
	} else {
		b.Asks = append(b.Asks, level)
		sort.Sort(ByPriceAsc(b.Asks))
	}
}

type StopOrder struct {
	Order     *Order
	Triggered bool
}

type StopBook struct {
	Stops []*StopOrder
}

func (sb *StopBook) AddStop(o *Order) {
	sb.Stops = append(sb.Stops, &StopOrder{Order: o, Triggered: false})
}

func (sb *StopBook) CheckTriggers(lastPrice core.Money) []*Order {
	var activated []*Order
	for _, so := range sb.Stops {
		if so.Triggered {
			continue
		}
		var triggered bool
		switch so.Order.Side {
		case core.OrderSideSell:
			triggered = lastPrice.LessThanOrEqual(so.Order.StopPrice)
		case core.OrderSideBuy:
			triggered = lastPrice.GreaterThanOrEqual(so.Order.StopPrice)
		}
		if triggered {
			so.Triggered = true
			marketOrder := *so.Order
			marketOrder.Type = core.OrderTypeMarket
			activated = append(activated, &marketOrder)
		}
	}
	return activated
}

var fillIDCounter int64

func newFillID() string {
	fillIDCounter++
	return "fill_" + time.Now().Format("20060102150405") + "_" + itoa(fillIDCounter)
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
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
