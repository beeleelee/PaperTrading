package app

import (
	"context"
	"fmt"
	"time"
	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/market"
	"github.com/felix/papertrading/internal/domain/metrics"
	"github.com/felix/papertrading/internal/domain/order"
	"github.com/felix/papertrading/internal/domain/portfolio"
	"github.com/felix/papertrading/internal/domain/risk"
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
	InitialCash     core.Money
	PortfolioID     portfolio.PortfolioID
	FillConfig      order.FillConfig
	RiskConstraints risk.Constraints
	OrderRepo       order.Repository
	PortfolioRepo   portfolio.Repository
	LogTrades       bool
}

type SimulationResult struct {
	Portfolio   *portfolio.Portfolio
	Orders      []*order.Order
	Fills       []order.Fill
	EquityCurve []EquityPoint
	Metrics     metrics.Metrics
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
	orderBook *OrderBookManager
	stopBook  *order.StopBook
	portfolio *portfolio.Portfolio
	clk       clock.Clock
	bus       *event.Bus
	log       Logger
	config    SimulationConfig

	orderRepo     order.Repository
	portfolioRepo portfolio.Repository

	orders      []*order.Order
	fills       []order.Fill
	equity      []EquityPoint
	lastPrices  map[core.Symbol]core.Money
	peakEquity  core.Money
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
		orderBook:  NewOrderBookManager(),
		stopBook:   &order.StopBook{},
		portfolio:  portfolio.NewPortfolio(config.PortfolioID, config.InitialCash, clk.Now()),
		clk:        clk,
		bus:        bus,
		log:        stdLogger{},
		config:     config,
		orderRepo:     config.OrderRepo,
		portfolioRepo: config.PortfolioRepo,
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
		switch o.Type {
		case core.OrderTypeMarket:
			matches := s.orderBook.FindMatch(o, s.lastPrice(o.Symbol))
			for _, fill := range matches {
				if err := s.applyFill(o, fill, now); err != nil {
					return err
				}
			}
		case core.OrderTypeLimit:
			s.orderBook.AddOrder(o)
			matches := s.orderBook.FindMatch(o, s.lastPrice(o.Symbol))
			for _, fill := range matches {
				if err := s.applyFill(o, fill, now); err != nil {
					return err
				}
			}
		}
	}

	resting := s.orderBook.RestingOrders(tick.Symbol)
	for _, o := range resting {
		matches := s.orderBook.FindMatch(o, tick.Price)
		for _, fill := range matches {
			if err := s.applyFill(o, fill, now); err != nil {
				return err
			}
		}
	}

	sig, err := s.strategy.OnTick(ctx, tick)
	if err != nil {
		return fmt.Errorf("strategy error: %w", err)
	}

	if sig != nil {
		prices := map[core.Symbol]core.Money{tick.Symbol: tick.Price}
		equity := s.portfolio.TotalEquity(prices)

		skipOrder := false
		orderPrice := sig.Price
		if orderPrice.IsZero() {
			orderPrice = tick.Price
		}
		if err := s.config.RiskConstraints.ValidateOrder(s.portfolio, sig.Symbol, sig.Type.ToOrderSide(), orderPrice, sig.Quantity, prices); err != nil {
			s.log.Log("  [RISK] %s — skipping signal %s %s qty=%d @ %s", err, sig.Type, sig.Symbol, sig.Quantity, orderPrice)
			skipOrder = true
		}
		if !skipOrder {
			if err := s.config.RiskConstraints.ValidateDrawdown(equity, s.peakEquity); err != nil {
				s.log.Log("  [RISK] %s — stopping further trading", err)
				skipOrder = true
				s.config.RiskConstraints = risk.Constraints{MaxPositions: -1}
			}
		}

		if !skipOrder {
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
				OrderType:   o.Type,
				Price:       o.Price,
				Quantity:    o.Quantity,
				At:          now,
			})

			if o.Type == core.OrderTypeStop || o.Type == core.OrderTypeStopLimit {
				s.stopBook.AddStop(o)
			} else if o.Type == core.OrderTypeLimit {
				s.orderBook.AddOrder(o)
				matches := s.orderBook.FindMatch(o, s.lastPrice(o.Symbol))
				for _, fill := range matches {
					if err := s.applyFill(o, fill, now); err != nil {
						return err
					}
				}
			} else {
				matches := s.orderBook.FindMatch(o, s.lastPrice(o.Symbol))
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
	}

	equity := s.portfolio.TotalEquity(map[core.Symbol]core.Money{tick.Symbol: tick.Price})
	if equity.GreaterThan(s.peakEquity) {
		s.peakEquity = equity
	}
	s.equity = append(s.equity, EquityPoint{Time: tick.Timestamp, Equity: equity})

	if s.portfolioRepo != nil {
		s.portfolioRepo.Save(context.Background(), s.portfolio)
	}

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

	if s.orderRepo != nil {
		s.orderRepo.Save(context.Background(), o)
	}
	if s.portfolioRepo != nil {
		s.portfolioRepo.Save(context.Background(), s.portfolio)
	}

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

	equityValues := make([]core.Money, len(s.equity))
	for i, ep := range s.equity {
		equityValues[i] = ep.Equity
	}

	calc := metrics.NewCalculator()
	m := calc.Calculate(s.config.InitialCash, s.portfolio, equityValues, prices)

	return &SimulationResult{
		Portfolio:   s.portfolio,
		Orders:      s.orders,
		Fills:       s.fills,
		EquityCurve: s.equity,
		Metrics:     m,
		StartTime:   s.equity[0].Time,
		EndTime:     s.equity[len(s.equity)-1].Time,
	}
}

type SymbolOrders struct {
	Bids []*order.Order
	Asks []*order.Order
}

type OrderBookManager struct {
	books map[core.Symbol]*SymbolOrders
}

func NewOrderBookManager() *OrderBookManager {
	return &OrderBookManager{books: make(map[core.Symbol]*SymbolOrders)}
}

func (m *OrderBookManager) book(sym core.Symbol) *SymbolOrders {
	b, ok := m.books[sym]
	if !ok {
		b = &SymbolOrders{}
		m.books[sym] = b
	}
	return b
}

func (m *OrderBookManager) AddOrder(o *order.Order) {
	b := m.book(o.Symbol)
	if o.Side == core.OrderSideBuy {
		b.Bids = append(b.Bids, o)
	} else {
		b.Asks = append(b.Asks, o)
	}
}

func (m *OrderBookManager) FindMatch(o *order.Order, lastPrice core.Money) []order.Fill {
	var fills []order.Fill

	switch o.Type {
	case core.OrderTypeMarket:
		if lastPrice.IsZero() {
			return nil
		}
		fills = append(fills, order.Fill{
			ID:        order.FillID(newFillID()),
			OrderID:   o.ID,
			Price:     lastPrice,
			Quantity:  o.RemainingQty(),
			Timestamp: time.Now(),
		})

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
			fills = append(fills, order.Fill{
				ID:        order.FillID(newFillID()),
				OrderID:   o.ID,
				Price:     o.Price,
				Quantity:  o.RemainingQty(),
				Timestamp: time.Now(),
			})
		}
	}

	return fills
}

func (m *OrderBookManager) RestingOrders(symbol core.Symbol) []*order.Order {
	b, ok := m.books[symbol]
	if !ok {
		return nil
	}
	var result []*order.Order
	for _, o := range b.Bids {
		if o.IsActive() && o.RemainingQty() > 0 {
			result = append(result, o)
		}
	}
	for _, o := range b.Asks {
		if o.IsActive() && o.RemainingQty() > 0 {
			result = append(result, o)
		}
	}
	return result
}

func (m *OrderBookManager) RemoveFilled(o *order.Order) {
	b, ok := m.books[o.Symbol]
	if !ok {
		return
	}
	if o.Side == core.OrderSideBuy {
		b.Bids = removeOrder(b.Bids, o)
	} else {
		b.Asks = removeOrder(b.Asks, o)
	}
}

func removeOrder(orders []*order.Order, target *order.Order) []*order.Order {
	for i, o := range orders {
		if o == target {
			return append(orders[:i], orders[i+1:]...)
		}
	}
	return orders
}

var fillCounter int64

func newFillID() string {
	fillCounter++
	return fmt.Sprintf("fill_%d", fillCounter)
}
