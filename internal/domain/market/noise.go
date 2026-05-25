package market

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/order"
	"github.com/shopspring/decimal"
)

type NoiseConfig struct {
	Enabled         bool    `json:"enabled"`
	Count           int     `json:"count"`
	MaxSpreadBP     float64 `json:"max_spread_bp"`
	MinQty          int64   `json:"min_qty"`
	MaxQty          int64   `json:"max_qty"`
	OrderRate       float64 `json:"order_rate"`
	MarketOrderRate float64 `json:"market_order_rate"`
	InitialPosition int64   `json:"initial_position"`
}

type NoiseTraderGroup struct {
	config   NoiseConfig
	traderID []string
}

func NewNoiseTraderGroup(config NoiseConfig) *NoiseTraderGroup {
	ids := make([]string, config.Count)
	for i := 0; i < config.Count; i++ {
		ids[i] = fmt.Sprintf("noise_%d", i+1)
	}
	return &NoiseTraderGroup{
		config:   config,
		traderID: ids,
	}
}

func (ng *NoiseTraderGroup) IDs() []string {
	return ng.traderID
}

func (ng *NoiseTraderGroup) GenerateOrders(markPrice core.Money, symbol core.Symbol, now time.Time) []*order.Order {
	if !ng.config.Enabled || ng.config.Count == 0 || markPrice.IsZero() {
		return nil
	}

	var orders []*order.Order
	maxSpreadFrac := ng.config.MaxSpreadBP / 10000.0
	spreadDec := decimal.NewFromFloat(maxSpreadFrac)

	for _, noiseID := range ng.traderID {
		if rand.Float64() >= ng.config.OrderRate {
			continue
		}

		qty := ng.config.MinQty
		if ng.config.MaxQty > ng.config.MinQty {
			qty += rand.Int63n(ng.config.MaxQty - ng.config.MinQty)
		}
		if qty <= 0 {
			qty = 1
		}

		side := core.OrderSideBuy
		if rand.Float64() < 0.5 {
			side = core.OrderSideSell
		}

		randomFrac := spreadDec.Mul(decimal.NewFromFloat(rand.Float64()))
		offset := markPrice.Decimal.Mul(randomFrac)

		var price core.Money
		if side == core.OrderSideBuy {
			price = core.Money{Decimal: markPrice.Decimal.Sub(offset)}
		} else {
			price = core.Money{Decimal: markPrice.Decimal.Add(offset)}
		}

		if price.LessThanOrEqual(core.NewMoneyFromInt(0)) {
			continue
		}

		o, err := order.NewOrder(
			order.OrderID(fmt.Sprintf("%s_%s_%d", noiseID, symbol, now.UnixNano())),
			noiseID,
			symbol,
			side,
			core.OrderTypeLimit,
			price,
			core.NewMoneyFromInt(0),
			qty,
			now,
		)
		if err != nil {
			continue
		}
		o.Submit(now)
		orders = append(orders, o)
	}

	// Also generate market orders (fill at best opposing price)
	for _, noiseID := range ng.traderID {
		if ng.config.MarketOrderRate <= 0 || rand.Float64() >= ng.config.MarketOrderRate {
			continue
		}

		qty := ng.config.MinQty
		if ng.config.MaxQty > ng.config.MinQty {
			qty += rand.Int63n(ng.config.MaxQty - ng.config.MinQty)
		}
		if qty <= 0 {
			qty = 1
		}

		side := core.OrderSideBuy
		if rand.Float64() < 0.5 {
			side = core.OrderSideSell
		}

		o, err := order.NewOrder(
			order.OrderID(fmt.Sprintf("%s_mkt_%s_%d", noiseID, symbol, now.UnixNano())),
			noiseID,
			symbol,
			side,
			core.OrderTypeMarket,
			core.NewMoneyFromInt(0),
			core.NewMoneyFromInt(0),
			qty,
			now,
		)
		if err != nil {
			continue
		}
		o.Submit(now)
		orders = append(orders, o)
	}

	return orders
}

type CrossFill struct {
	BuyOrder  *order.Order
	SellOrder *order.Order
	Fill      order.Fill
}

// DetectCross returns crossing pairs of noise orders without applying fills.
func DetectCross(noiseOrders []*order.Order) []CrossFill {
	var result []CrossFill
	var buys, sells []*order.Order
	for _, o := range noiseOrders {
		if !o.IsActive() || o.RemainingQty() <= 0 {
			continue
		}
		if o.Side == core.OrderSideBuy {
			buys = append(buys, o)
		} else {
			sells = append(sells, o)
		}
	}

	used := make(map[*order.Order]bool)
	for _, buy := range buys {
		if used[buy] {
			continue
		}
		for _, sell := range sells {
			if used[sell] {
				continue
			}
			if buy.Price.GreaterThanOrEqual(sell.Price) {
				fillQty := buy.RemainingQty()
				if fillQty > sell.RemainingQty() {
					fillQty = sell.RemainingQty()
				}

				mid := core.Money{Decimal: buy.Price.Decimal.Add(sell.Price.Decimal).Div(decimal.NewFromInt(2))}

				f := order.Fill{
					ID:        order.FillID(fmt.Sprintf("xfill_%d", len(result))),
					OrderID:   buy.ID,
					Price:     mid,
					Quantity:  fillQty,
					Timestamp: time.Now(),
				}
				result = append(result, CrossFill{
					BuyOrder:  buy,
					SellOrder: sell,
					Fill:      f,
				})
				used[buy] = true
				used[sell] = true
				break
			}
		}
	}
	return result
}
