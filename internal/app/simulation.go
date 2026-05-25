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
	NoiseConfig     market.NoiseConfig
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
	orderBook *OrderBookManager
	stopBook  *order.StopBook
	portfolio *portfolio.Portfolio
	clk       clock.Clock
	bus       *event.Bus
	log       Logger
	config    SimulationConfig

	noiseGroup      *market.NoiseTraderGroup
	noisePortfolios map[string]*portfolio.Portfolio
	markPrices      map[core.Symbol]core.Money
	initialPriceSet bool

	orderRepo     order.Repository
	portfolioRepo portfolio.Repository

	orders     []*order.Order
	fills      []order.Fill
	equity     []EquityPoint
	peakEquity core.Money
}

func NewSimulation(
	feed market.Feed,
	strategy strategy.Strategy,
	clk clock.Clock,
	bus *event.Bus,
	config SimulationConfig,
) *Simulation {
	noiseGroup := market.NewNoiseTraderGroup(config.NoiseConfig)
	noisePortfolios := make(map[string]*portfolio.Portfolio)
	for _, id := range noiseGroup.IDs() {
		p := portfolio.NewPortfolio(
			portfolio.PortfolioID(id),
			config.InitialCash,
			clk.Now(),
		)
		noisePortfolios[id] = p
	}

	return &Simulation{
		feed:            feed,
		strategy:        strategy,
		orderBook:       NewOrderBookManager(),
		stopBook:        &order.StopBook{},
		portfolio:       portfolio.NewPortfolio(config.PortfolioID, config.InitialCash, clk.Now()),
		clk:             clk,
		bus:             bus,
		log:             stdLogger{},
		config:          config,
		noiseGroup:      noiseGroup,
		noisePortfolios: noisePortfolios,
		markPrices:      make(map[core.Symbol]core.Money),
		orderRepo:       config.OrderRepo,
		portfolioRepo:   config.PortfolioRepo,
		orders:          make([]*order.Order, 0),
		fills:           make([]order.Fill, 0),
		equity:          make([]EquityPoint, 0),
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
	sym := tick.Symbol

	mp := s.markPrices[sym]
	if mp.IsZero() {
		mp = tick.Price
		s.markPrices[sym] = mp
	}

	if s.config.LogTrades {
		s.log.Log("[%s] %s mark=%s ref=%s",
			tick.Timestamp.Format("15:04:05"), sym, mp, tick.Price)
	}

	// Step 1: Noise traders generate liquidity around markPrice
	noiseOrders := s.noiseGroup.GenerateOrders(mp, sym, now)

	// Split into limit orders (book liquidity) and market orders (aggressive fills)
	var noiseLimits, noiseMarkets []*order.Order
	for _, o := range noiseOrders {
		if o.Type == core.OrderTypeMarket {
			noiseMarkets = append(noiseMarkets, o)
		} else {
			noiseLimits = append(noiseLimits, o)
		}
	}

	// Cross-match noise limit orders among themselves (atomic buy/sell)
	crosses := market.DetectCross(noiseLimits)
	for _, x := range crosses {
		if err := s.crossFill(x.BuyOrder, x.SellOrder, x.Fill, now); err != nil {
			return err
		}
		mp = x.Fill.Price
		s.markPrices[sym] = mp
	}
	// Add remaining (unfilled) noise limit orders to the book
	for _, o := range noiseLimits {
		if o.IsActive() && o.RemainingQty() > 0 {
			s.orderBook.AddOrder(o)
		}
	}

	// Match noise market orders against the book (creates baseline price movement)
	// matchAgainstBook already handles both sides atomically via atomicFill.
	for _, mo := range noiseMarkets {
		matches := s.matchAgainstBook(mo, mp)
		if len(matches) > 0 {
			s.markPrices[sym] = matches[0].Price
			mp = matches[0].Price
		}
	}

	// Step 2: Check stop orders against markPrice
	activated := s.stopBook.CheckTriggers(mp)
	for _, o := range activated {
		switch o.Type {
		case core.OrderTypeMarket:
			matches := s.matchAgainstBook(o, mp)
			for _, fill := range matches {
				if err := s.applyFill(o, fill, now); err != nil {
					return err
				}
				s.markPrices[sym] = fill.Price
				mp = fill.Price
			}
		case core.OrderTypeLimit:
			s.orderBook.AddOrder(o)
			matches := s.orderBook.FindMatch(o, mp)
			for _, fill := range matches {
				if err := s.applyFill(o, fill, now); err != nil {
					return err
				}
				s.markPrices[sym] = fill.Price
				mp = fill.Price
			}
		}
	}

	// Step 3: Check all resting orders against markPrice
	resting := s.orderBook.RestingOrders(sym)
	for _, o := range resting {
		matches := s.orderBook.FindMatch(o, mp)
		for _, fill := range matches {
			if err := s.applyFill(o, fill, now); err != nil {
				return err
			}
			s.markPrices[sym] = fill.Price
			mp = fill.Price
		}
	}

	// Step 4: Strategy receives tick with current markPrice
	strategyTick := market.Tick{
		Symbol:    sym,
		Price:     mp,
		Volume:    tick.Volume,
		Timestamp: tick.Timestamp,
	}

	sig, err := s.strategy.OnTick(ctx, strategyTick)
	if err != nil {
		return fmt.Errorf("strategy error: %w", err)
	}

	if sig != nil {
		prices := map[core.Symbol]core.Money{sym: mp}
		equity := s.portfolio.TotalEquity(prices)

		skipOrder := false
		orderPrice := sig.Price
		if orderPrice.IsZero() {
			orderPrice = mp
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
			} else {
				matches := s.matchAgainstBook(o, mp)
				for _, fill := range matches {
					s.markPrices[sym] = fill.Price
					mp = fill.Price
				}
				if !o.IsFilled() && o.Type == core.OrderTypeLimit {
					s.orderBook.AddOrder(o)
				}
			}

			if s.config.LogTrades {
				s.log.Log("  -> %s %s %s qty=%d", o.Side, o.Type, o.Symbol, o.Quantity)
			}
		}
	}

	// Step 5: Update equity curve using markPrice
	equity := s.portfolio.TotalEquity(s.markPrices)
	if equity.GreaterThan(s.peakEquity) {
		s.peakEquity = equity
	}
	s.equity = append(s.equity, EquityPoint{Time: tick.Timestamp, Equity: equity})

	// Step 6: Publish tick processed event
	s.bus.Publish(ctx, event.TickProcessed{
		Symbol: sym,
		Price:  mp,
		Volume: tick.Volume,
		Equity: equity,
		At:     tick.Timestamp,
	})

	if s.portfolioRepo != nil {
		s.portfolioRepo.Save(context.Background(), s.portfolio)
	}

	return nil
}

// matchAgainstBook matches an active order against the best opposing order in the book.
// Handles both portfolios atomically via atomicFill.
func (s *Simulation) matchAgainstBook(o *order.Order, markPrice core.Money) []order.Fill {
	b := s.orderBook.book(o.Symbol)

	var opposing []*order.Order
	if o.Side == core.OrderSideBuy {
		opposing = b.Asks
	} else {
		opposing = b.Bids
	}

	best := findBestOpposing(opposing, o.Side)
	if best == nil {
		if markPrice.IsZero() {
			return nil
		}
		return s.orderBook.FindMatch(o, markPrice)
	}

	fillQty := o.RemainingQty()
	if fillQty > best.RemainingQty() {
		fillQty = best.RemainingQty()
	}

	fillPrice := best.Price

	fill := order.Fill{
		ID:        order.FillID(newFillID()),
		OrderID:   o.ID,
		Price:     fillPrice,
		Quantity:  fillQty,
		Timestamp: time.Now(),
	}

	if err := s.atomicFill(o, best, fill, time.Now()); err == nil {
		if best.IsFilled() {
			s.orderBook.RemoveFilled(best)
		}
	}

	return []order.Fill{fill}
}

// atomicFill executes a trade between two orders, updating BOTH portfolios atomically.
// The fill's OrderID matches the active order; the opposing order also gets a fill.
func (s *Simulation) atomicFill(active, opposing *order.Order, fill order.Fill, now time.Time) error {
	// Mark both orders filled
	if err := active.ApplyFill(fill, now); err != nil {
		return err
	}
	opposingFill := order.Fill{
		ID:        order.FillID(newFillID()),
		OrderID:   opposing.ID,
		Price:     fill.Price,
		Quantity:  fill.Quantity,
		Timestamp: now,
	}
	if err := opposing.ApplyFill(opposingFill, now); err != nil {
		return err
	}
	s.fills = append(s.fills, fill, opposingFill)

	// Determine which portfolio is buying and which is selling
	var buyOrder, sellOrder *order.Order
	if active.Side == core.OrderSideBuy {
		buyOrder, sellOrder = active, opposing
	} else {
		buyOrder, sellOrder = opposing, active
	}

	// Get portfolios
	var buyPort, sellPort *portfolio.Portfolio
	if buyOrder.PortfolioID == string(s.config.PortfolioID) {
		buyPort = s.portfolio
	} else if p, ok := s.noisePortfolios[buyOrder.PortfolioID]; ok {
		buyPort = p
	}
	if sellOrder.PortfolioID == string(s.config.PortfolioID) {
		sellPort = s.portfolio
	} else if p, ok := s.noisePortfolios[sellOrder.PortfolioID]; ok {
		sellPort = p
	}

	if buyPort == nil || sellPort == nil {
		return nil
	}

	cost := fill.Price.Mul(fill.Quantity)
	revenue := fill.Price.Mul(fill.Quantity)

	// Execute: buyer pays, gets shares
	if cost.GreaterThan(buyPort.Cash) {
		return nil
	}
	buyPort.Cash = buyPort.Cash.Sub(cost)
	if pos, exists := buyPort.Positions[buyOrder.Symbol]; exists {
		totalQty := pos.Quantity + fill.Quantity
		totalCost := pos.AvgEntryPrice.Mul(pos.Quantity).Add(cost)
		pos.AvgEntryPrice = totalCost.Div(totalQty)
		pos.Quantity = totalQty
	} else {
		np, err := portfolio.NewPosition(buyOrder.Symbol, fill.Price, fill.Quantity)
		if err != nil {
			return err
		}
		np.OpenedAt = now
		buyPort.Positions[buyOrder.Symbol] = np
	}

	// Seller: gets cash. If seller has shares, reduce/close the position.
	// If seller doesn't have shares (short), the order still fills — cash is credited.
	sellPort.Cash = sellPort.Cash.Add(revenue)
	spos, hasPos := sellPort.Positions[sellOrder.Symbol]
	if hasPos {
		pnl := revenue.Sub(spos.AvgEntryPrice.Mul(fill.Quantity))
		sellPort.SetRealizedPnL(sellPort.RealizedPnL().Add(pnl))
		if fill.Quantity >= spos.Quantity {
			delete(sellPort.Positions, sellOrder.Symbol)
		} else {
			spos.Quantity -= fill.Quantity
		}
	}

	return nil
}

func findBestOpposing(orders []*order.Order, side core.OrderSide) *order.Order {
	var best *order.Order
	for _, o := range orders {
		if !o.IsActive() || o.RemainingQty() <= 0 {
			continue
		}
		if side == core.OrderSideBuy {
			if best == nil || o.Price.LessThan(best.Price) {
				best = o
			}
		} else {
			if best == nil || o.Price.GreaterThan(best.Price) {
				best = o
			}
		}
	}
	return best
}

func (s *Simulation) applyFill(o *order.Order, fill order.Fill, now time.Time) error {
	if err := o.ApplyFill(fill, now); err != nil {
		return err
	}
	s.fills = append(s.fills, fill)

	// Route fill to the correct portfolio
	var targetPortfolio *portfolio.Portfolio
	if o.PortfolioID == string(s.config.PortfolioID) {
		targetPortfolio = s.portfolio
	} else if p, ok := s.noisePortfolios[o.PortfolioID]; ok {
		targetPortfolio = p
	} else {
		return fmt.Errorf("unknown portfolio: %s", o.PortfolioID)
	}

	markPrice, hasMP := s.markPrices[o.Symbol]
	if !hasMP {
		markPrice = fill.Price
	}
	prices := map[core.Symbol]core.Money{o.Symbol: markPrice}

	switch o.Side {
	case core.OrderSideBuy:
		if _, exists := targetPortfolio.Positions[o.Symbol]; exists {
			if err := targetPortfolio.IncreasePosition(o.Symbol, fill.Price, fill.Quantity, now); err != nil {
				return err
			}
		} else {
			if err := targetPortfolio.OpenPosition(o.Symbol, fill.Price, fill.Quantity, now); err != nil {
				return err
			}
		}

	case core.OrderSideSell:
		pos, posExists := targetPortfolio.Positions[o.Symbol]
		if !posExists {
			s.log.Log("  [SKIP] no position to sell %s for %s", o.Symbol, o.PortfolioID)
			return nil
		}
		if o.Quantity >= pos.Quantity {
			if err := targetPortfolio.ClosePosition(o.Symbol, fill.Price, now); err != nil {
				return err
			}
		} else {
			if err := targetPortfolio.ReducePosition(o.Symbol, fill.Price, fill.Quantity, now); err != nil {
				return err
			}
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
	if s.portfolioRepo != nil && targetPortfolio == s.portfolio {
		s.portfolioRepo.Save(context.Background(), s.portfolio)
	}

	if s.config.LogTrades {
		s.log.Log("  [FILL] %s %s @ %s qty=%d (cash=%s, equity=%s)",
			o.Side, o.Symbol, fill.Price, fill.Quantity,
			targetPortfolio.Cash, targetPortfolio.TotalEquity(prices))
	}

	return nil
}

func (s *Simulation) crossFill(buy, sell *order.Order, fill order.Fill, now time.Time) error {
	buyPort, buyOk := s.noisePortfolios[buy.PortfolioID]
	sellPort, sellOk := s.noisePortfolios[sell.PortfolioID]
	if !buyOk || !sellOk {
		return nil
	}

	if err := buy.ApplyFill(fill, now); err != nil {
		return err
	}
	if err := sell.ApplyFill(fill, now); err != nil {
		return err
	}
	s.fills = append(s.fills, fill, fill)

	cost := fill.Price.Mul(fill.Quantity)
	revenue := fill.Price.Mul(fill.Quantity)

	if cost.GreaterThan(buyPort.Cash) {
		return nil
	}
	buyPort.Cash = buyPort.Cash.Sub(cost)

	if pos, exists := buyPort.Positions[buy.Symbol]; exists {
		totalQty := pos.Quantity + fill.Quantity
		totalCost := pos.AvgEntryPrice.Mul(pos.Quantity).Add(cost)
		pos.AvgEntryPrice = totalCost.Div(totalQty)
		pos.Quantity = totalQty
	} else {
		np, err := portfolio.NewPosition(buy.Symbol, fill.Price, fill.Quantity)
		if err != nil {
			return err
		}
		np.OpenedAt = now
		buyPort.Positions[buy.Symbol] = np
	}

	sellPort.Cash = sellPort.Cash.Add(revenue)
	spos, hasPos := sellPort.Positions[sell.Symbol]
	if hasPos {
		pnl := revenue.Sub(spos.AvgEntryPrice.Mul(fill.Quantity))
		sellPort.SetRealizedPnL(sellPort.RealizedPnL().Add(pnl))
		if fill.Quantity >= spos.Quantity {
			delete(sellPort.Positions, sell.Symbol)
		} else {
			spos.Quantity -= fill.Quantity
		}
	}

	return nil
}

func (s *Simulation) Depth(symbol core.Symbol) DepthSnapshot {
	return s.orderBook.Depth(symbol)
}

func (s *Simulation) updateLastPrice(symbol core.Symbol, price core.Money) {
	s.markPrices[symbol] = price
}

func (s *Simulation) lastPrice(symbol core.Symbol) core.Money {
	return s.markPrices[symbol]
}

func (s *Simulation) LastPrice(symbol core.Symbol) core.Money {
	return s.markPrices[symbol]
}

func (s *Simulation) result() *SimulationResult {
	prices := make(map[core.Symbol]core.Money)
	for sym := range s.portfolio.Positions {
		prices[sym] = s.markPrices[sym]
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
