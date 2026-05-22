package portfolio

import (
	"testing"
	"time"
	"github.com/felix/papertrading/internal/domain/core"
)

func mustMoney(t *testing.T, s string) core.Money {
	t.Helper()
	m, err := core.NewMoney(s)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestNewPortfolio(t *testing.T) {
	now := time.Now()
	cash := mustMoney(t, "100000.00")
	p := NewPortfolio("p1", cash, now)
	if p.Cash.String() != "100000.00" {
		t.Fatalf("expected 100000.00, got %s", p.Cash.String())
	}
	if p.PositionCount() != 0 {
		t.Fatalf("expected 0 positions, got %d", p.PositionCount())
	}
}

func TestPortfolio_Deposit(t *testing.T) {
	now := time.Now()
	p := NewPortfolio("p1", mustMoney(t, "1000.00"), now)
	p.Deposit(mustMoney(t, "500.00"), now)
	if p.Cash.String() != "1500.00" {
		t.Fatalf("expected 1500.00, got %s", p.Cash.String())
	}
}

func TestPortfolio_Withdraw(t *testing.T) {
	now := time.Now()
	p := NewPortfolio("p1", mustMoney(t, "1000.00"), now)
	if err := p.Withdraw(mustMoney(t, "400.00"), now); err != nil {
		t.Fatal(err)
	}
	if p.Cash.String() != "600.00" {
		t.Fatalf("expected 600.00, got %s", p.Cash.String())
	}
}

func TestPortfolio_Withdraw_Insufficient(t *testing.T) {
	now := time.Now()
	p := NewPortfolio("p1", mustMoney(t, "100.00"), now)
	err := p.Withdraw(mustMoney(t, "200.00"), now)
	if err != ErrInsufficientCash {
		t.Fatal("expected ErrInsufficientCash")
	}
}

func TestPortfolio_OpenPosition(t *testing.T) {
	now := time.Now()
	p := NewPortfolio("p1", mustMoney(t, "10000.00"), now)
	err := p.OpenPosition("AAPL", mustMoney(t, "150.00"), 10, now)
	if err != nil {
		t.Fatal(err)
	}
	if !p.HasPosition("AAPL") {
		t.Fatal("should have AAPL position")
	}
	expectedCash := mustMoney(t, "8500.00")
	if p.Cash.String() != expectedCash.String() {
		t.Fatalf("expected cash %s, got %s", expectedCash, p.Cash.String())
	}
}

func TestPortfolio_OpenPosition_InsufficientCash(t *testing.T) {
	now := time.Now()
	p := NewPortfolio("p1", mustMoney(t, "100.00"), now)
	err := p.OpenPosition("AAPL", mustMoney(t, "150.00"), 10, now)
	if err != ErrInsufficientCash {
		t.Fatal("expected ErrInsufficientCash")
	}
}

func TestPortfolio_IncreasePosition(t *testing.T) {
	now := time.Now()
	p := NewPortfolio("p1", mustMoney(t, "100000.00"), now)
	p.OpenPosition("AAPL", mustMoney(t, "100.00"), 10, now)

	p.IncreasePosition("AAPL", mustMoney(t, "110.00"), 10, now)
	pos := p.Positions["AAPL"]
	if pos.Quantity != 20 {
		t.Fatalf("expected qty 20, got %d", pos.Quantity)
	}
	// avg = (10*100 + 10*110) / 20 = 105
	if pos.AvgEntryPrice.String() != "105.00" {
		t.Fatalf("expected avg 105.00, got %s", pos.AvgEntryPrice.String())
	}
}

func TestPortfolio_ReducePosition(t *testing.T) {
	now := time.Now()
	p := NewPortfolio("p1", mustMoney(t, "100000.00"), now)
	p.OpenPosition("AAPL", mustMoney(t, "100.00"), 10, now)
	p.ReducePosition("AAPL", mustMoney(t, "120.00"), 4, now)

	pos := p.Positions["AAPL"]
	if pos.Quantity != 6 {
		t.Fatalf("expected qty 6, got %d", pos.Quantity)
	}
	// cash: 100000 - 1000 (cost) + 480 (revenue) = 99480
	if p.Cash.String() != "99480.00" {
		t.Fatalf("expected cash 99480.00, got %s", p.Cash.String())
	}
}

func TestPortfolio_ReducePosition_FullClose(t *testing.T) {
	now := time.Now()
	p := NewPortfolio("p1", mustMoney(t, "100000.00"), now)
	p.OpenPosition("AAPL", mustMoney(t, "100.00"), 10, now)
	p.ReducePosition("AAPL", mustMoney(t, "120.00"), 10, now)

	if p.HasPosition("AAPL") {
		t.Fatal("position should be closed")
	}
	if p.PositionCount() != 0 {
		t.Fatalf("expected 0 positions, got %d", p.PositionCount())
	}
}

func TestPortfolio_ClosePosition(t *testing.T) {
	now := time.Now()
	p := NewPortfolio("p1", mustMoney(t, "100000.00"), now)
	p.OpenPosition("AAPL", mustMoney(t, "100.00"), 10, now)
	p.ClosePosition("AAPL", mustMoney(t, "120.00"), now)

	if p.HasPosition("AAPL") {
		t.Fatal("position should be closed")
	}
}

func TestPortfolio_TotalEquity(t *testing.T) {
	now := time.Now()
	p := NewPortfolio("p1", mustMoney(t, "10000.00"), now)
	p.OpenPosition("AAPL", mustMoney(t, "150.00"), 10, now)

	prices := map[core.Symbol]core.Money{"AAPL": mustMoney(t, "160.00")}
	equity := p.TotalEquity(prices)
	// cash: 10000 - 1500 = 8500, position: 10 * 160 = 1600, total: 10100
	if equity.String() != "10100.00" {
		t.Fatalf("expected equity 10100.00, got %s", equity.String())
	}
}

func TestPortfolio_UnrealizedPnL(t *testing.T) {
	now := time.Now()
	p := NewPortfolio("p1", mustMoney(t, "100000.00"), now)
	p.OpenPosition("AAPL", mustMoney(t, "100.00"), 10, now)

	prices := map[core.Symbol]core.Money{"AAPL": mustMoney(t, "120.00")}
	pnl := p.UnrealizedPnL(prices)
	// (120 - 100) * 10 = 200
	if pnl.String() != "200.00" {
		t.Fatalf("expected PnL 200.00, got %s", pnl.String())
	}
}

func TestPortfolio_PositionCount(t *testing.T) {
	now := time.Now()
	p := NewPortfolio("p1", mustMoney(t, "100000.00"), now)
	p.OpenPosition("AAPL", mustMoney(t, "100.00"), 10, now)
	p.OpenPosition("GOOG", mustMoney(t, "200.00"), 5, now)
	if p.PositionCount() != 2 {
		t.Fatalf("expected 2 positions, got %d", p.PositionCount())
	}
}
