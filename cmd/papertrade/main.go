package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"github.com/felix/papertrading/internal/app"
	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/order"
	"github.com/felix/papertrading/internal/domain/portfolio"
	"github.com/felix/papertrading/internal/domain/strategy"
	"github.com/felix/papertrading/internal/event"
	"github.com/felix/papertrading/internal/infra/clock"
	"github.com/felix/papertrading/internal/infra/feed"
)

func main() {
	csvPath := flag.String("csv", "test/data/aapl_5min.csv", "path to CSV market data")
	symbol := flag.String("symbol", "AAPL", "default symbol for CSV without symbol column")
	cash := flag.String("cash", "100000.00", "initial cash")
	strategyName := flag.String("strategy", "ma_cross", "strategy: ma_cross or price_cross")
	fastPeriod := flag.Int("fast", 10, "fast MA period")
	slowPeriod := flag.Int("slow", 30, "slow MA period")
	qty := flag.Int("qty", 100, "trade quantity")
	threshold := flag.Float64("threshold", 0.0, "price threshold for price_cross strategy")
	flag.Parse()

	money, err := core.NewMoney(*cash)
	if err != nil {
		log.Fatalf("invalid cash: %v", err)
	}

	f := feed.NewCSVFeed(*csvPath, feed.WithSkipRows(1), feed.WithSymbol(core.Symbol(*symbol)))

	var strat strategy.Strategy
	switch *strategyName {
	case "ma_cross":
		strat = strategy.NewMovingAverageCross(
			*strategyName, core.Symbol(*symbol), *fastPeriod, *slowPeriod, int64(*qty),
		)
	case "price_cross":
		thresholdMoney := core.NewMoneyFromFloat(*threshold)
		strat = strategy.NewPriceCrosses(
			*strategyName, core.Symbol(*symbol), thresholdMoney, int64(*qty), core.OrderTypeMarket,
		)
	default:
		log.Fatalf("unknown strategy: %s", *strategyName)
	}

	bus := event.NewBus()
	cfg := app.SimulationConfig{
		InitialCash: money,
		PortfolioID: portfolio.PortfolioID("default"),
		FillConfig:  order.DefaultFillConfig(),
		LogTrades:   true,
	}

	sim := app.NewSimulation(f, strat, clock.RealClock{}, bus, cfg)
	sim.SetLogger(app.StdLogger{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Printf("Starting simulation: %s on %s with $%s\n", *strategyName, *symbol, *cash)
	fmt.Printf("Data: %s\n\n", *csvPath)

	result, err := sim.Run(ctx)
	if err != nil {
		log.Fatalf("simulation failed: %v", err)
	}

	fmt.Println()
	fmt.Println("=== Results ===")
	fmt.Printf("Initial cash:  $%s\n", *cash)
	fmt.Printf("Final cash:    $%s\n", result.Portfolio.Cash)
	fmt.Printf("Orders placed: %d\n", len(result.Orders))
	fmt.Printf("Fills:         %d\n", len(result.Fills))
	fmt.Printf("Positions:     %d\n", result.Portfolio.PositionCount())
	for sym, pos := range result.Portfolio.Positions {
		lastPrice := sim.LastPrice(sym)
		pnl := pos.UnrealizedPnL(lastPrice)
		fmt.Printf("  %s: %d shares @ avg $%s (last: $%s, PnL: $%s)\n",
			sym, pos.Quantity, pos.AvgEntryPrice, lastPrice, pnl)
	}
	fmt.Printf("Start:         %s\n", result.StartTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("End:           %s\n", result.EndTime.Format("2006-01-02 15:04:05"))

	if err := writeEquityCSV(result); err != nil {
		log.Printf("warning: could not write equity CSV: %v", err)
	}
}

func writeEquityCSV(result *app.SimulationResult) error {
	file, err := os.Create("equity_curve.csv")
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "timestamp,equity\n")
	for _, ep := range result.EquityCurve {
		fmt.Fprintf(file, "%s,%s\n", ep.Time.Format("2006-01-02 15:04:05"), ep.Equity)
	}
	return nil
}
