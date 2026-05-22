package risk

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

func TestConstraints_NoConstraints(t *testing.T) {
	c := Constraints{}
	now := time.Now()
	pf := portfolio.NewPortfolio("p1", mustMoney(t, "100000.00"), now)
	pf.OpenPosition("AAPL", mustMoney(t, "150.00"), 100, now)

	prices := map[core.Symbol]core.Money{"AAPL": mustMoney(t, "160.00")}

	if err := c.ValidateOrder(pf, "GOOG", core.OrderSideBuy, mustMoney(t, "200.00"), 50, prices); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if err := c.ValidateDrawdown(mustMoney(t, "90000.00"), mustMoney(t, "100000.00")); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestConstraints_MaxPositions(t *testing.T) {
	c := Constraints{MaxPositions: 2}
	now := time.Now()
	pf := portfolio.NewPortfolio("p1", mustMoney(t, "100000.00"), now)
	pf.OpenPosition("AAPL", mustMoney(t, "150.00"), 100, now)
	pf.OpenPosition("GOOG", mustMoney(t, "200.00"), 50, now)

	prices := map[core.Symbol]core.Money{
		"AAPL": mustMoney(t, "160.00"),
		"GOOG": mustMoney(t, "210.00"),
	}

	if err := c.ValidateOrder(pf, "MSFT", core.OrderSideBuy, mustMoney(t, "300.00"), 30, prices); err != ErrMaxPositionsExceeded {
		t.Fatalf("expected ErrMaxPositionsExceeded, got: %v", err)
	}
}

func TestConstraints_MaxPositionSize(t *testing.T) {
	c := Constraints{MaxPositionPct: 10}
	now := time.Now()
	pf := portfolio.NewPortfolio("p1", mustMoney(t, "100000.00"), now)

	prices := map[core.Symbol]core.Money{}

	// 100 shares @ 200 = 20000, which is 20% of 100000 — exceeds 10%
	if err := c.ValidateOrder(pf, "AAPL", core.OrderSideBuy, mustMoney(t, "200.00"), 100, prices); err != ErrPositionSizeExceeded {
		t.Fatalf("expected ErrPositionSizeExceeded, got: %v", err)
	}

	// 50 shares @ 150 = 7500, which is 7.5% of 100000 — within 10%
	if err := c.ValidateOrder(pf, "AAPL", core.OrderSideBuy, mustMoney(t, "150.00"), 50, prices); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestConstraints_SellOrderBypassesPositionCheck(t *testing.T) {
	c := Constraints{MaxPositionPct: 10}
	now := time.Now()
	pf := portfolio.NewPortfolio("p1", mustMoney(t, "100000.00"), now)

	prices := map[core.Symbol]core.Money{}

	// Sell orders should bypass position sizing checks
	if err := c.ValidateOrder(pf, "AAPL", core.OrderSideSell, mustMoney(t, "200.00"), 1000, prices); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestConstraints_Drawdown(t *testing.T) {
	tests := []struct {
		name          string
		maxDrawdownPct float64
		current       string
		peak          string
		expectError   bool
	}{
		{"no constraint", 0, "80000.00", "100000.00", false},
		{"within limit", 25, "80000.00", "100000.00", false},
		{"exceeds limit", 15, "80000.00", "100000.00", true},
		{"exactly at limit", 20, "80000.00", "100000.00", false},
		{"at peak", 10, "100000.00", "100000.00", false},
		{"zero peak", 10, "80000.00", "0.00", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Constraints{MaxDrawdownPct: tt.maxDrawdownPct}
			current := mustMoney(t, tt.current)
			peak := mustMoney(t, tt.peak)
			err := c.ValidateDrawdown(current, peak)
			if tt.expectError && err != ErrMaxDrawdownExceeded {
				t.Fatalf("expected ErrMaxDrawdownExceeded, got: %v", err)
			}
			if !tt.expectError && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		})
	}
}
