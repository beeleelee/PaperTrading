package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/felix/papertrading/internal/app"
	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/market"
	"github.com/felix/papertrading/internal/domain/order"
	"github.com/felix/papertrading/internal/domain/portfolio"
	"github.com/felix/papertrading/internal/domain/risk"
	"github.com/felix/papertrading/internal/domain/strategy"
	"github.com/felix/papertrading/internal/event"
	"github.com/felix/papertrading/internal/infra/clock"
	"github.com/felix/papertrading/internal/infra/feed"
	"github.com/felix/papertrading/internal/infra/storage/sqlite"
)

func main() {
	csvPath := flag.String("csv", "test/data/aapl_5min.csv", "path to CSV market data")
	symbol := flag.String("symbol", "AAPL", "symbol or comma-separated symbols")
	cash := flag.String("cash", "100000.00", "initial cash")
	strategyName := flag.String("strategy", "ma_cross", "strategy: ma_cross or price_cross")
	fastPeriod := flag.Int("fast", 10, "fast MA period")
	slowPeriod := flag.Int("slow", 30, "slow MA period")
	qty := flag.Int("qty", 100, "trade quantity per position")
	threshold := flag.Float64("threshold", 0.0, "price threshold for price_cross strategy")
	maxPosPct := flag.Float64("max-pos-pct", 0, "max position size as %% of portfolio (0 = unlimited)")
	maxDrawdown := flag.Float64("max-drawdown", 0, "max drawdown %% before stopping (0 = unlimited)")
	maxPositions := flag.Int("max-positions", 0, "max open positions (0 = unlimited)")
	db := flag.String("db", "memory", "storage backend: 'memory' or sqlite file path")
	flag.Parse()

	money, err := core.NewMoney(*cash)
	if err != nil {
		log.Fatalf("invalid cash: %v", err)
	}

	symbols := strings.Split(*symbol, ",")
	csvPaths := strings.Split(*csvPath, ",")
	if len(symbols) != len(csvPaths) {
		log.Fatalf("number of symbols (%d) must match number of CSV paths (%d)", len(symbols), len(csvPaths))
	}

	var feeds []market.Feed
	var strats []strategy.Strategy
	for i, sym := range symbols {
		sym = strings.TrimSpace(sym)
		f := feed.NewCSVFeed(strings.TrimSpace(csvPaths[i]), feed.WithSkipRows(1), feed.WithSymbol(core.Symbol(sym)))
		feeds = append(feeds, f)

		var s strategy.Strategy
		switch *strategyName {
		case "ma_cross":
			s = strategy.NewMovingAverageCross(
				*strategyName, core.Symbol(sym), *fastPeriod, *slowPeriod, int64(*qty),
			)
		case "price_cross":
			thresholdMoney := core.NewMoneyFromFloat(*threshold)
			s = strategy.NewPriceCrosses(
				*strategyName, core.Symbol(sym), thresholdMoney, int64(*qty), core.OrderTypeMarket,
			)
		default:
			log.Fatalf("unknown strategy: %s", *strategyName)
		}
		strats = append(strats, s)
	}

	f := feed.NewMergedFeed(feeds...)
	var strat strategy.Strategy
	if len(strats) == 1 {
		strat = strats[0]
	} else {
		strat = strategy.NewMultiStrategy(strats...)
	}

	bus := event.NewBus()
	cfg := app.SimulationConfig{
		InitialCash: money,
		PortfolioID: portfolio.PortfolioID("default"),
		FillConfig:  order.DefaultFillConfig(),
		RiskConstraints: risk.Constraints{
			MaxPositionPct: *maxPosPct,
			MaxDrawdownPct: *maxDrawdown,
			MaxPositions:   *maxPositions,
		},
		LogTrades: true,
	}

	if *db != "memory" {
		d, err := sqlite.Open(*db)
		if err != nil {
			log.Fatalf("open sqlite: %v", err)
		}
		defer d.Close()
		cfg.OrderRepo = sqlite.NewOrderRepository(d)
		cfg.PortfolioRepo = sqlite.NewPortfolioRepository(d)
		fmt.Printf("Storage:       SQLite (%s)\n", *db)
	} else {
		fmt.Printf("Storage:       memory\n")
	}

	sim := app.NewSimulation(f, strat, clock.RealClock{}, bus, cfg)
	sim.SetLogger(app.StdLogger{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Printf("Starting simulation: %s on %s with $%s\n", *strategyName, *symbol, *cash)
	fmt.Printf("Data: %s\n", *csvPath)

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

	m := result.Metrics
	pnlStr := m.TotalPnL.String()
	if m.TotalPnL.IsNegative() {
		pnlStr = "-$" + pnlStr[1:]
	} else {
		pnlStr = "$" + pnlStr
	}
	fmt.Printf("Total PnL:     %s\n", pnlStr)
	if m.TotalTrades > 0 {
		fmt.Printf("Trades:        %d (%d win / %d loss)\n", m.TotalTrades, m.WinningTrades, m.LosingTrades)
		fmt.Printf("Win rate:      %.1f%%\n", m.WinRate)
		if m.ProfitFactor > 1e9 {
			fmt.Printf("Profit factor: Inf\n")
		} else {
			fmt.Printf("Profit factor: %.2f\n", m.ProfitFactor)
		}
	}
	fmt.Printf("Max drawdown:  %.2f%%\n", m.MaxDrawdown)
	fmt.Printf("Sharpe ratio:  %.2f\n", m.SharpeRatio)
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
