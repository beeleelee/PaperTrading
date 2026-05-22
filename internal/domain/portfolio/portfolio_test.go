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

func TestPortfolio_RealizedPnL_NoTrades(t *testing.T) {
	now := time.Now()
	p := NewPortfolio("p1", mustMoney(t, "100000.00"), now)
	if p.RealizedPnL().String() != "0.00" {
		t.Fatalf("expected 0 realized PnL, got %s", p.RealizedPnL())
	}
	if len(p.ClosedTrades()) != 0 {
		t.Fatalf("expected 0 closed trades, got %d", len(p.ClosedTrades()))
	}
}

func TestPortfolio_RealizedPnL_ClosePosition(t *testing.T) {
	now := time.Now()
	p := NewPortfolio("p1", mustMoney(t, "100000.00"), now)
	p.OpenPosition("AAPL", mustMoney(t, "100.00"), 100, now)
	p.ClosePosition("AAPL", mustMoney(t, "120.00"), now)

	// PnL = (120 - 100) * 100 = 2000
	if p.RealizedPnL().String() != "2000.00" {
		t.Fatalf("expected realized PnL 2000.00, got %s", p.RealizedPnL())
	}
	closed := p.ClosedTrades()
	if len(closed) != 1 {
		t.Fatalf("expected 1 closed trade, got %d", len(closed))
	}
	if closed[0].PnL.String() != "2000.00" {
		t.Fatalf("expected trade PnL 2000.00, got %s", closed[0].PnL)
	}
	if closed[0].Quantity != 100 {
		t.Fatalf("expected trade qty 100, got %d", closed[0].Quantity)
	}
	if closed[0].EntryPrice.String() != "100.00" {
		t.Fatalf("expected entry price 100.00, got %s", closed[0].EntryPrice)
	}
	if closed[0].ExitPrice.String() != "120.00" {
		t.Fatalf("expected exit price 120.00, got %s", closed[0].ExitPrice)
	}
}

func TestPortfolio_RealizedPnL_LosingTrade(t *testing.T) {
	now := time.Now()
	p := NewPortfolio("p1", mustMoney(t, "100000.00"), now)
	p.OpenPosition("AAPL", mustMoney(t, "100.00"), 100, now)
	p.ClosePosition("AAPL", mustMoney(t, "80.00"), now)

	// PnL = (80 - 100) * 100 = -2000
	if p.RealizedPnL().String() != "-2000.00" {
		t.Fatalf("expected realized PnL -2000.00, got %s", p.RealizedPnL())
	}
	if p.ClosedTrades()[0].PnL.String() != "-2000.00" {
		t.Fatalf("expected trade PnL -2000.00, got %s", p.ClosedTrades()[0].PnL)
	}
}

func TestPortfolio_RealizedPnL_PartialClose(t *testing.T) {
	now := time.Now()
	p := NewPortfolio("p1", mustMoney(t, "100000.00"), now)
	p.OpenPosition("AAPL", mustMoney(t, "100.00"), 100, now)
	p.ReducePosition("AAPL", mustMoney(t, "120.00"), 40, now)

	// PnL = (120 - 100) * 40 = 800
	if p.RealizedPnL().String() != "800.00" {
		t.Fatalf("expected realized PnL 800.00, got %s", p.RealizedPnL())
	}
	if len(p.ClosedTrades()) != 1 {
		t.Fatalf("expected 1 closed trade, got %d", len(p.ClosedTrades()))
	}
	if p.ClosedTrades()[0].Quantity != 40 {
		t.Fatalf("expected trade qty 40, got %d", p.ClosedTrades()[0].Quantity)
	}
	// Remaining position should still be there
	if !p.HasPosition("AAPL") {
		t.Fatal("AAPL position should still exist")
	}
	if p.Positions["AAPL"].Quantity != 60 {
		t.Fatalf("expected 60 shares remaining, got %d", p.Positions["AAPL"].Quantity)
	}
}

func TestPortfolio_RealizedPnL_MultipleTrades(t *testing.T) {
	now := time.Now()
	p := NewPortfolio("p1", mustMoney(t, "100000.00"), now)

	p.OpenPosition("AAPL", mustMoney(t, "100.00"), 100, now)
	p.ClosePosition("AAPL", mustMoney(t, "110.00"), now)

	p.OpenPosition("GOOG", mustMoney(t, "200.00"), 50, now)
	p.ClosePosition("GOOG", mustMoney(t, "180.00"), now)

	// Total PnL = (110-100)*100 + (180-200)*50 = 1000 + (-1000) = 0
	if p.RealizedPnL().String() != "0.00" {
		t.Fatalf("expected realized PnL 0.00, got %s", p.RealizedPnL())
	}
	if len(p.ClosedTrades()) != 2 {
		t.Fatalf("expected 2 closed trades, got %d", len(p.ClosedTrades()))
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
