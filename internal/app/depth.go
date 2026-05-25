package app

import (
	"sort"

	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/order"
)

type DepthLevel struct {
	Price    core.Money `json:"price"`
	Quantity int64      `json:"quantity"`
	Count    int        `json:"count"`
}

type DepthSnapshot struct {
	Symbol string      `json:"symbol"`
	Bids   []DepthLevel `json:"bids"`
	Asks   []DepthLevel `json:"asks"`
}

func (m *OrderBookManager) Depth(symbol core.Symbol) DepthSnapshot {
	b := m.book(symbol)

	bids := aggregateLevels(b.Bids, true)
	asks := aggregateLevels(b.Asks, false)

	return DepthSnapshot{
		Symbol: string(symbol),
		Bids:   bids,
		Asks:   asks,
	}
}

func aggregateLevels(orders []*order.Order, isBid bool) []DepthLevel {
	var active []*order.Order
	for _, o := range orders {
		if o.IsActive() && o.RemainingQty() > 0 {
			active = append(active, o)
		}
	}

	levels := make(map[string]*DepthLevel)
	for _, o := range active {
		key := o.Price.String()
		if l, ok := levels[key]; ok {
			l.Quantity += o.RemainingQty()
			l.Count++
		} else {
			levels[key] = &DepthLevel{
				Price:    o.Price,
				Quantity: o.RemainingQty(),
				Count:    1,
			}
		}
	}

	result := make([]DepthLevel, 0, len(levels))
	for _, l := range levels {
		result = append(result, *l)
	}

	if isBid {
		sort.Slice(result, func(i, j int) bool {
			return result[i].Price.GreaterThan(result[j].Price)
		})
	} else {
		sort.Slice(result, func(i, j int) bool {
			return result[i].Price.LessThan(result[j].Price)
		})
	}

	return result
}
