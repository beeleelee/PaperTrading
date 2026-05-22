package api

import (
	"fmt"
	"strconv"

	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/strategy"
)

func createStrategy(name string, params map[string]any, symbol core.Symbol) (strategy.Strategy, error) {
	switch name {
	case "ma_cross":
		fast := getInt(params, "fast", 10)
		slow := getInt(params, "slow", 30)
		qty := getInt(params, "qty", 100)
		return strategy.NewMovingAverageCross(name, symbol, fast, slow, int64(qty)), nil

	case "price_cross":
		threshold := getFloat(params, "threshold", 0)
		qty := getInt(params, "qty", 100)
		orderTypeStr := getString(params, "order_type", "market")
		var ot core.OrderType
		switch orderTypeStr {
		case "limit":
			ot = core.OrderTypeLimit
		case "stop":
			ot = core.OrderTypeStop
		case "stop_limit":
			ot = core.OrderTypeStopLimit
		default:
			ot = core.OrderTypeMarket
		}
		return strategy.NewPriceCrosses(name, symbol, core.NewMoneyFromFloat(threshold), int64(qty), ot), nil

	default:
		return nil, fmt.Errorf("unknown strategy: %s", name)
	}
}

func getInt(m map[string]any, key string, defaultVal int) int {
	v, ok := m[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case string:
		n, err := strconv.Atoi(val)
		if err != nil {
			return defaultVal
		}
		return n
	default:
		return defaultVal
	}
}

func getFloat(m map[string]any, key string, defaultVal float64) float64 {
	v, ok := m[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case string:
		n, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return defaultVal
		}
		return n
	default:
		return defaultVal
	}
}

func getString(m map[string]any, key string, defaultVal string) string {
	v, ok := m[key]
	if !ok {
		return defaultVal
	}
	s, ok := v.(string)
	if !ok {
		return defaultVal
	}
	return s
}
