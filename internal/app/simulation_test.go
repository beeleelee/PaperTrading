package app

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/market"
	"github.com/felix/papertrading/internal/domain/order"
	"github.com/felix/papertrading/internal/domain/portfolio"
	"github.com/felix/papertrading/internal/domain/strategy"
	"github.com/felix/papertrading/internal/event"
	"github.com/felix/papertrading/internal/infra/clock"
	"github.com/felix/papertrading/internal/infra/feed"
)

func TestSimulation_Run(t *testing.T) {
	rand.Seed(42)

	csvContent := `timestamp,price
2024-01-02 09:30:00,100.00
2024-01-02 09:35:00,101.00
2024-01-02 09:40:00,102.00
2024-01-02 09:45:00,103.00
2024-01-02 09:50:00,104.00
2024-01-02 09:55:00,105.00`

	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatal(err)
	}

	money, _ := core.NewMoney("100000.00")
	f := feed.NewCSVFeed(csvPath, feed.WithSkipRows(1), feed.WithSymbol("AAPL"))
	clk := clock.NewSimulatedClock(mustParseTime("2024-01-02 09:30:00"))
	bus := event.NewBus()
	cfg := SimulationConfig{
		InitialCash: money,
		PortfolioID: portfolio.PortfolioID("test-portfolio"),
		FillConfig:  order.DefaultFillConfig(),
		NoiseConfig: market.NoiseConfig{
			Enabled:         true,
			Count:           5,
			MaxSpreadBP:     100,
			MinQty:          10,
			MaxQty:          100,
			OrderRate:       1.0,
			MarketOrderRate: 0.4,
		},
	}

	sim := NewSimulation(f, strategy.NewPriceCrosses("cross", "AAPL", core.NewMoneyFromInt(101), 100, core.OrderTypeMarket), clk, bus, cfg)
	result, err := sim.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(result.EquityCurve) == 0 {
		t.Fatal("expected equity curve points")
	}

	t.Logf("orders: %d, fills: %d, cash: %s", len(result.Orders), len(result.Fills), result.Portfolio.Cash)
}

func TestSimulation_MAStrategy(t *testing.T) {
	rand.Seed(42)
	csvContent := `timestamp,price
2024-01-02 09:30:00,100.00
2024-01-02 09:35:00,101.00
2024-01-02 09:40:00,102.00
2024-01-02 09:45:00,103.00
2024-01-02 09:50:00,104.00
2024-01-02 09:55:00,105.00
2024-01-02 10:00:00,106.00
2024-01-02 10:05:00,107.00
2024-01-02 10:10:00,108.00
2024-01-02 10:15:00,107.00
2024-01-02 10:20:00,106.00
2024-01-02 10:25:00,105.00`

	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatal(err)
	}

	money, _ := core.NewMoney("100000.00")
	f := feed.NewCSVFeed(csvPath, feed.WithSkipRows(1), feed.WithSymbol("AAPL"))
	s := strategy.NewMovingAverageCross("ma-cross", "AAPL", 3, 5, 100)
	clk := clock.NewSimulatedClock(mustParseTime("2024-01-02 09:30:00"))
	bus := event.NewBus()
	cfg := SimulationConfig{
		InitialCash: money,
		PortfolioID: portfolio.PortfolioID("test-portfolio"),
		FillConfig:  order.DefaultFillConfig(),
		NoiseConfig: market.NoiseConfig{
			Enabled:         true,
			Count:           5,
			MaxSpreadBP:     30,
			MinQty:          10,
			MaxQty:          50,
			OrderRate:       1.0,
			MarketOrderRate: 0.3,
		},
	}

	sim := NewSimulation(f, s, clk, bus, cfg)
	result, err := sim.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Orders) == 0 {
		t.Log("no orders generated — MA cross may not have triggered")
	}

	if result.Portfolio.Cash.String() != "100000.00" {
		t.Logf("final cash: %s, orders: %d, positions: %d",
			result.Portfolio.Cash, len(result.Orders), result.Portfolio.PositionCount())
	}
}

func TestSimulation_NoStrategySignals(t *testing.T) {
	csvContent := `timestamp,price
2024-01-02 09:30:00,100.00
2024-01-02 09:35:00,100.50`

	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatal(err)
	}

	money, _ := core.NewMoney("50000.00")
	f := feed.NewCSVFeed(csvPath, feed.WithSkipRows(1), feed.WithSymbol("AAPL"))
	s := strategy.NewMovingAverageCross("ma-cross", "AAPL", 10, 20, 100)
	clk := clock.NewSimulatedClock(mustParseTime("2024-01-02 09:30:00"))
	bus := event.NewBus()
	cfg := SimulationConfig{
		InitialCash: money,
		PortfolioID: portfolio.PortfolioID("test-portfolio"),
		FillConfig:  order.DefaultFillConfig(),
	}

	sim := NewSimulation(f, s, clk, bus, cfg)
	result, err := sim.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Orders) != 0 {
		t.Fatal("expected 0 orders (insufficient warmup data)")
	}
	if len(result.EquityCurve) != 2 {
		t.Fatalf("expected 2 equity points, got %d", len(result.EquityCurve))
	}
}

func mustParseTime(s string) time.Time {
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		panic(err)
	}
	return t
}
