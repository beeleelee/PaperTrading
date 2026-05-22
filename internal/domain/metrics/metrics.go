package metrics

import (
	"math"
	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/portfolio"
)

type Metrics struct {
	TotalPnL     core.Money
	MaxDrawdown  float64
	SharpeRatio  float64
	WinRate      float64
	ProfitFactor float64
	TotalTrades  int
	WinningTrades int
	LosingTrades  int
}

type Calculator struct {
	PeriodsPerYear float64
	RiskFreeRate   float64
}

func NewCalculator() *Calculator {
	return &Calculator{
		PeriodsPerYear: 252,
		RiskFreeRate:   0.05,
	}
}

func (c *Calculator) Calculate(
	initialCash core.Money,
	pf *portfolio.Portfolio,
	equityCurve []core.Money,
	prices map[core.Symbol]core.Money,
) Metrics {
	m := Metrics{}

	finalEquity := initialCash
	if len(equityCurve) > 0 {
		finalEquity = equityCurve[len(equityCurve)-1]
	} else {
		finalEquity = pf.TotalEquity(prices)
	}

	m.TotalPnL = finalEquity.Sub(initialCash)
	m.MaxDrawdown = calcMaxDrawdown(equityCurve)
	m.SharpeRatio = calcSharpe(equityCurve, c.PeriodsPerYear, c.RiskFreeRate)

	closedTrades := pf.ClosedTrades()
	if len(closedTrades) > 0 {
		m.TotalTrades = len(closedTrades)
		var grossProfit, grossLoss float64
		for _, t := range closedTrades {
			pnl, _ := t.PnL.Float64()
			if pnl >= 0 {
				m.WinningTrades++
				grossProfit += pnl
			} else {
				m.LosingTrades++
				grossLoss += math.Abs(pnl)
			}
		}

		if m.TotalTrades > 0 {
			m.WinRate = float64(m.WinningTrades) / float64(m.TotalTrades) * 100
		}

		if grossLoss > 0 {
			m.ProfitFactor = grossProfit / grossLoss
		} else if grossProfit > 0 {
			m.ProfitFactor = math.Inf(1)
		}
	}

	return m
}

func calcMaxDrawdown(equityCurve []core.Money) float64 {
	if len(equityCurve) < 2 {
		return 0
	}

	var peak float64
	var maxDD float64
	for _, e := range equityCurve {
		v, _ := e.Float64()
		if v > peak {
			peak = v
		}
		if peak > 0 {
			dd := (peak - v) / peak * 100
			if dd > maxDD {
				maxDD = dd
			}
		}
	}
	return maxDD
}

func calcSharpe(equityCurve []core.Money, periodsPerYear, riskFreeRate float64) float64 {
	if len(equityCurve) < 2 {
		return 0
	}

	returns := make([]float64, 0, len(equityCurve)-1)
	for i := 1; i < len(equityCurve); i++ {
		prev, _ := equityCurve[i-1].Float64()
		curr, _ := equityCurve[i].Float64()
		if prev == 0 {
			continue
		}
		returns = append(returns, (curr-prev)/prev)
	}

	if len(returns) < 2 {
		return 0
	}

	var sum float64
	for _, r := range returns {
		sum += r
	}
	mean := sum / float64(len(returns))

	var varSum float64
	for _, r := range returns {
		diff := r - mean
		varSum += diff * diff
	}
	stdDev := math.Sqrt(varSum / float64(len(returns)-1))

	if stdDev == 0 {
		return 0
	}

	excessReturn := mean - (riskFreeRate / periodsPerYear)
	return excessReturn / stdDev * math.Sqrt(periodsPerYear)
}
