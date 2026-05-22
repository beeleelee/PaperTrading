package app

import (
	"context"
	"fmt"
	"time"
	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/market"
	"github.com/felix/papertrading/internal/domain/order"
	"github.com/felix/papertrading/internal/domain/portfolio"
	"github.com/felix/papertrading/internal/domain/strategy"
	"github.com/felix/papertrading/internal/event"
	"github.com/felix/papertrading/internal/infra/clock"
)

type Logger interface {
	Log(format string, args ...interface{})
}

type StdLogger struct{}

func (StdLogger) Log(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

type stdLogger struct{}

func (stdLogger) Log(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

type SimulationConfig struct {
	InitialCash   core.Money
	PortfolioID   portfolio.PortfolioID
	FillConfig    order.FillConfig
	LogTrades     bool
}

type SimulationResult struct {
	Portfolio   *portfolio.Portfolio
	Orders      []*order.Order
	Fills       []order.Fill
	EquityCurve []EquityPoint
	StartTime   time.Time
	EndTime     time.Time
}

type EquityPoint struct {
	Time   time.Time
	Equity core.Money
}

type Simulation struct {
	feed      market.Feed
	strategy  strategy.Strategy
	engine    *order.MatchingEngine
	orderBook *BacktestOrderBook
	stopBook  *order.StopBook
	portfolio *portfolio.Portfolio
	clk       clock.Clock
	bus       *event.Bus
	log       Logger
	config    SimulationConfig

	orders     []*order.Order
	fills      []order.Fill
	equity     []EquityPoint
	lastPrices map[core.Symbol]core.Money
	fillSeq    int64
}

func NewSimulation(
	feed market.Feed,
	strategy strategy.Strategy,
	clk clock.Clock,
	bus *event.Bus,
	config SimulationConfig,
) *Simulation {
	engine := order.NewMatchingEngine(config.FillConfig)
	return &Simulation{
		feed:       feed,
		strategy:   strategy,
		engine:     engine,
		orderBook:  NewBacktestOrderBook(),
		stopBook:   &order.StopBook{},
		portfolio:  portfolio.NewPortfolio(config.PortfolioID, config.InitialCash, clk.Now()),
		clk:        clk,
		bus:        bus,
		log:        stdLogger{},
		config:     config,
		orders:     make([]*order.Order, 0),
		fills:      make([]order.Fill, 0),
		equity:     make([]EquityPoint, 0),
		lastPrices: make(map[core.Symbol]core.Money),
	}
}

func (s *Simulation) SetLogger(l Logger) {
	s.log = l
}

func (s *Simulation) Run(ctx context.Context) (*SimulationResult, error) {
	tickCh := make(chan market.Tick, 100)
	errCh := make(chan error, 1)

	go func() {
		errCh <- s.feed.Stream(ctx, tickCh)
	}()

	for tick := range tickCh {
		if err := s.processTick(ctx, tick); err != nil {
			return nil, fmt.Errorf("process tick: %w", err)
		}
	}

	if err := <-errCh; err != nil {
		return nil, err
	}

	return s.result(), nil
}

func (s *Simulation) processTick(ctx context.Context, tick market.Tick) error {
	now := s.clk.Now()

	if s.config.LogTrades {
		s.log.Log("[%s] %s price=%s vol=%d",
			tick.Timestamp.Format("15:04:05"), tick.Symbol, tick.Price, tick.Volume)
	}

	s.updateLastPrice(tick.Symbol, tick.Price)

	activated := s.stopBook.CheckTriggers(tick.Price)
	for _, o := range activated {
			matches := s.orderBook.FindMatch(o, s.lastPrice(o.Symbol))
			for _, fill := range matches {
				if err := s.applyFill(o, fill, now); err != nil {
					return err
				}
			}
			if o.IsActive() {
				s.orderBook.AddOrder(o)
			}
		}

		sig, err := s.strategy.OnTick(ctx, tick)
		if err != nil {
			return fmt.Errorf("strategy error: %w", err)
		}

		if sig != nil {
			o, err := sig.ToOrder(string(s.config.PortfolioID), now)
			if err != nil {
				return fmt.Errorf("signal to order: %w", err)
			}
			o.Submit(now)
			s.orders = append(s.orders, o)

			s.bus.Publish(ctx, event.OrderSubmitted{
				OrderID:     string(o.ID),
				PortfolioID: o.PortfolioID,
				Symbol:      o.Symbol,
				Side:        o.Side,
				OrderType:  o.Type,
				Price:       o.Price,
				Quantity:    o.Quantity,
				At:          now,
			})

			if o.Type == core.OrderTypeStop || o.Type == core.OrderTypeStopLimit {
				s.stopBook.AddStop(o)
			} else {
				matches := s.orderBook.FindMatch(o, s.lastPrice(o.Symbol))
				if len(matches) == 0 && o.Type == core.OrderTypeLimit {
					s.orderBook.AddOrder(o)
				}
				for _, fill := range matches {
					if err := s.applyFill(o, fill, now); err != nil {
						return err
					}
				}
			}

		if s.config.LogTrades {
			s.log.Log("  -> %s %s %s qty=%d", o.Side, o.Type, o.Symbol, o.Quantity)
		}
	}

	lastPrice := s.lastPrice(tick.Symbol)
	prices := map[core.Symbol]core.Money{tick.Symbol: lastPrice}
	equity := s.portfolio.TotalEquity(prices)
	s.equity = append(s.equity, EquityPoint{Time: tick.Timestamp, Equity: equity})

	return nil
}

func (s *Simulation) applyFill(o *order.Order, fill order.Fill, now time.Time) error {
	if err := o.ApplyFill(fill, now); err != nil {
		return err
	}
	s.fills = append(s.fills, fill)

	lastPrice := s.lastPrice(o.Symbol)
	prices := map[core.Symbol]core.Money{o.Symbol: lastPrice}

	switch o.Side {
	case core.OrderSideBuy:
		if _, exists := s.portfolio.Positions[o.Symbol]; exists {
			s.portfolio.IncreasePosition(o.Symbol, fill.Price, fill.Quantity, now)
		} else {
			s.portfolio.OpenPosition(o.Symbol, fill.Price, fill.Quantity, now)
		}
	case core.OrderSideSell:
		pos, posExists := s.portfolio.Positions[o.Symbol]
		if !posExists {
			s.log.Log("  [SKIP] no position to sell %s", o.Symbol)
			return nil
		}
		if o.Quantity >= pos.Quantity {
			s.portfolio.ClosePosition(o.Symbol, fill.Price, now)
		} else {
			s.portfolio.ReducePosition(o.Symbol, fill.Price, fill.Quantity, now)
		}
	}

	s.bus.Publish(context.Background(), event.OrderFilled{
		OrderID:     string(o.ID),
		FillID:      string(fill.ID),
		PortfolioID: o.PortfolioID,
		Symbol:      o.Symbol,
		Side:        o.Side,
		Price:       fill.Price,
		Quantity:    fill.Quantity,
		Commission:  fill.Commission,
		At:          now,
	})

	if s.config.LogTrades {
		s.log.Log("  [FILL] %s %s @ %s qty=%d (cash=%s, equity=%s)",
			o.Side, o.Symbol, fill.Price, fill.Quantity,
			s.portfolio.Cash, s.portfolio.TotalEquity(prices))
	}

	return nil
}

func (s *Simulation) updateLastPrice(symbol core.Symbol, price core.Money) {
	s.lastPrices[symbol] = price
}

func (s *Simulation) lastPrice(symbol core.Symbol) core.Money {
	return s.LastPrice(symbol)
}

func (s *Simulation) LastPrice(symbol core.Symbol) core.Money {
	if p, ok := s.lastPrices[symbol]; ok {
		return p
	}
	return core.NewMoneyFromInt(0)
}

func (s *Simulation) result() *SimulationResult {
	prices := make(map[core.Symbol]core.Money)
	for sym := range s.portfolio.Positions {
		prices[sym] = s.lastPrice(sym)
	}
	return &SimulationResult{
		Portfolio:   s.portfolio,
		Orders:      s.orders,
		Fills:       s.fills,
		EquityCurve: s.equity,
		StartTime:   s.equity[0].Time,
		EndTime:     s.equity[len(s.equity)-1].Time,
	}
}

type BacktestOrderBook struct {
	bids []*order.Order
	asks []*order.Order
}

func NewBacktestOrderBook() *BacktestOrderBook {
	return &BacktestOrderBook{}
}

func (b *BacktestOrderBook) AddOrder(o *order.Order) {
	if o.Side == core.OrderSideBuy {
		b.bids = append(b.bids, o)
	} else {
		b.asks = append(b.asks, o)
	}
}

func (b *BacktestOrderBook) FindMatch(o *order.Order, lastPrice core.Money) []order.Fill {
	var fills []order.Fill

	switch o.Type {
	case core.OrderTypeMarket:
		if lastPrice.IsZero() {
			return nil
		}
		fill := order.Fill{
			ID:        order.FillID(newFillID()),
			OrderID:   o.ID,
			Price:     lastPrice,
			Quantity:  o.RemainingQty(),
			Timestamp: time.Now(),
		}
		fills = append(fills, fill)

	case core.OrderTypeLimit:
		if lastPrice.IsZero() {
			return nil
		}
		canFill := false
		if o.Side == core.OrderSideBuy && lastPrice.LessThanOrEqual(o.Price) {
			canFill = true
		} else if o.Side == core.OrderSideSell && lastPrice.GreaterThanOrEqual(o.Price) {
			canFill = true
		}
		if canFill {
			fill := order.Fill{
				ID:        order.FillID(newFillID()),
				OrderID:   o.ID,
				Price:     o.Price,
				Quantity:  o.RemainingQty(),
				Timestamp: time.Now(),
			}
			fills = append(fills, fill)
		}
	}

	return fills
}

var fillCounter int64

func newFillID() string {
	fillCounter++
	return fmt.Sprintf("fill_%d", fillCounter)
}
