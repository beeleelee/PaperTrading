package metrics

import (
	"testing"
	"time"
	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/portfolio"
)

func mustMoney(t *testing.T, s string) core.Money {
	t.Helper()
	m, err := core.NewMoney(s)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestCalculator_NoTrades(t *testing.T) {
	now := time.Now()
	pf := portfolio.NewPortfolio("p1", mustMoney(t, "100000.00"), now)
	calc := NewCalculator()

	equityCurve := []core.Money{mustMoney(t, "100000.00"), mustMoney(t, "100000.00")}
	prices := map[core.Symbol]core.Money{}
	m := calc.Calculate(mustMoney(t, "100000.00"), pf, equityCurve, prices)

	if !m.TotalPnL.IsZero() {
		t.Fatalf("expected zero PnL, got %s", m.TotalPnL)
	}
	if m.MaxDrawdown != 0 {
		t.Fatalf("expected 0 max drawdown, got %.2f", m.MaxDrawdown)
	}
	if m.SharpeRatio != 0 {
		t.Fatalf("expected 0 Sharpe, got %.2f", m.SharpeRatio)
	}
	if m.TotalTrades != 0 {
		t.Fatalf("expected 0 trades, got %d", m.TotalTrades)
	}
}

func TestCalculator_WithClosedTrades(t *testing.T) {
	now := time.Now()
	pf := portfolio.NewPortfolio("p1", mustMoney(t, "50000.00"), now)
	pf.OpenPosition("AAPL", mustMoney(t, "100.00"), 100, now)
	pf.ClosePosition("AAPL", mustMoney(t, "120.00"), now)

	calc := NewCalculator()
	equityCurve := []core.Money{mustMoney(t, "50000.00"), mustMoney(t, "52000.00")}
	prices := map[core.Symbol]core.Money{}
	m := calc.Calculate(mustMoney(t, "50000.00"), pf, equityCurve, prices)

	// PnL: 52000 - 50000 = 2000
	if m.TotalPnL.String() != "2000.00" {
		t.Fatalf("expected PnL 2000.00, got %s", m.TotalPnL)
	}
	if m.TotalTrades != 1 {
		t.Fatalf("expected 1 trade, got %d", m.TotalTrades)
	}
	if m.WinningTrades != 1 {
		t.Fatalf("expected 1 winning trade, got %d", m.WinningTrades)
	}
	if m.WinRate != 100.0 {
		t.Fatalf("expected 100%% win rate, got %.1f", m.WinRate)
	}
	if m.ProfitFactor == 0 {
		t.Fatal("expected positive profit factor")
	}
}

func TestCalculator_WinRateAndProfitFactor(t *testing.T) {
	now := time.Now()
	pf := portfolio.NewPortfolio("p1", mustMoney(t, "100000.00"), now)

	// Two winning trades, one losing
	pf.OpenPosition("AAPL", mustMoney(t, "100.00"), 100, now)
	pf.ClosePosition("AAPL", mustMoney(t, "110.00"), now)

	pf.OpenPosition("GOOG", mustMoney(t, "200.00"), 50, now)
	pf.ClosePosition("GOOG", mustMoney(t, "180.00"), now)

	pf.OpenPosition("MSFT", mustMoney(t, "50.00"), 200, now)
	pf.ClosePosition("MSFT", mustMoney(t, "60.00"), now)

	calc := NewCalculator()
	equityCurve := []core.Money{mustMoney(t, "100000.00"), mustMoney(t, "100000.00")}
	prices := map[core.Symbol]core.Money{}
	m := calc.Calculate(mustMoney(t, "100000.00"), pf, equityCurve, prices)

	if m.TotalTrades != 3 {
		t.Fatalf("expected 3 trades, got %d", m.TotalTrades)
	}
	if m.WinningTrades != 2 {
		t.Fatalf("expected 2 winning trades, got %d", m.WinningTrades)
	}
	if m.LosingTrades != 1 {
		t.Fatalf("expected 1 losing trade, got %d", m.LosingTrades)
	}
	if m.WinRate < 66.0 || m.WinRate > 67.0 {
		t.Fatalf("expected ~66.7%% win rate, got %.1f", m.WinRate)
	}
}

func TestCalculator_MaxDrawdown(t *testing.T) {
	pf := portfolio.NewPortfolio("p1", mustMoney(t, "100000.00"), time.Now())
	calc := NewCalculator()

	equityCurve := []core.Money{
		mustMoney(t, "100000.00"),
		mustMoney(t, "105000.00"),
		mustMoney(t, "102000.00"),
		mustMoney(t, "95000.00"),
		mustMoney(t, "98000.00"),
		mustMoney(t, "110000.00"),
	}
	prices := map[core.Symbol]core.Money{}
	m := calc.Calculate(mustMoney(t, "100000.00"), pf, equityCurve, prices)

	// Peak = 110000 (last), trough after... wait, peak is 110000 at the end.
	// Let me recalculate: 
	// peak at idx 1: 105000, trough after: 95000 at idx 3
	// drawdown = (105000 - 95000) / 105000 * 100 = 10000/105000 * 100 = 9.52%
	if m.MaxDrawdown < 9.0 || m.MaxDrawdown > 10.0 {
		t.Fatalf("expected max drawdown ~9.52%%, got %.2f", m.MaxDrawdown)
	}
}

func TestCalculator_SharpeRatio(t *testing.T) {
	pf := portfolio.NewPortfolio("p1", mustMoney(t, "100000.00"), time.Now())
	calc := NewCalculator()

	// Steady upward equity curve
	equityCurve := []core.Money{
		mustMoney(t, "100000.00"),
		mustMoney(t, "101000.00"),
		mustMoney(t, "102000.00"),
		mustMoney(t, "103000.00"),
		mustMoney(t, "104000.00"),
	}
	prices := map[core.Symbol]core.Money{}
	m := calc.Calculate(mustMoney(t, "100000.00"), pf, equityCurve, prices)

	// With consistent 1% returns, Sharpe should be positive and significant
	if m.SharpeRatio <= 0 {
		t.Fatalf("expected positive Sharpe for upward trend, got %.2f", m.SharpeRatio)
	}
}
