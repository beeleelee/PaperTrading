package order

import (
	"testing"
	"github.com/felix/papertrading/internal/domain/core"
)

func TestMatchingEngine_MarketBuy_FillsAgainstAsks(t *testing.T) {
	engine := NewMatchingEngineWithSeed(DefaultFillConfig(), 42)
	book := &OrderBook{
		Symbol: "AAPL",
		Asks: []PriceLevel{
			{Price: mustMoney(t, "100.00"), Orders: []*Order{
				{ID: "ask1", Quantity: 100, FilledQty: 0},
			}},
		},
	}
	buyOrder := &Order{
		ID: "buy1", Side: core.OrderSideBuy, Type: core.OrderTypeMarket,
		Quantity: 50, Status: core.OrderStatusSubmitted,
	}
	fills, err := engine.Match(buyOrder, book)
	if err != nil {
		t.Fatal(err)
	}
	if len(fills) != 1 {
		t.Fatalf("expected 1 fill, got %d", len(fills))
	}
	if fills[0].Quantity != 50 {
		t.Fatalf("expected fill qty 50, got %d", fills[0].Quantity)
	}
	if fills[0].Price.String() != "100.00" {
		t.Fatalf("expected price 100.00, got %s", fills[0].Price.String())
	}
}

func TestMatchingEngine_MarketSell_FillsAgainstBids(t *testing.T) {
	engine := NewMatchingEngineWithSeed(DefaultFillConfig(), 42)
	book := &OrderBook{
		Symbol: "AAPL",
		Bids: []PriceLevel{
			{Price: mustMoney(t, "99.00"), Orders: []*Order{
				{ID: "bid1", Quantity: 100, FilledQty: 0},
			}},
		},
	}
	sellOrder := &Order{
		ID: "sell1", Side: core.OrderSideSell, Type: core.OrderTypeMarket,
		Quantity: 30, Status: core.OrderStatusSubmitted,
	}
	fills, err := engine.Match(sellOrder, book)
	if err != nil {
		t.Fatal(err)
	}
	if len(fills) != 1 {
		t.Fatalf("expected 1 fill, got %d", len(fills))
	}
	if fills[0].Price.String() != "99.00" {
		t.Fatalf("expected price 99.00, got %s", fills[0].Price.String())
	}
}

func TestMatchingEngine_MarketBuy_NoLiquidity(t *testing.T) {
	engine := NewMatchingEngineWithSeed(DefaultFillConfig(), 42)
	book := &OrderBook{Symbol: "AAPL"}
	buyOrder := &Order{
		ID: "buy1", Side: core.OrderSideBuy, Type: core.OrderTypeMarket,
		Quantity: 50, Status: core.OrderStatusSubmitted,
	}
	_, err := engine.Match(buyOrder, book)
	if err != ErrNoLiquidity {
		t.Fatal("expected ErrNoLiquidity")
	}
}

func TestMatchingEngine_MarketBuy_MultipleLevels(t *testing.T) {
	engine := NewMatchingEngineWithSeed(DefaultFillConfig(), 42)
	book := &OrderBook{
		Symbol: "AAPL",
		Asks: []PriceLevel{
			{Price: mustMoney(t, "100.00"), Orders: []*Order{
				{ID: "ask1", Quantity: 50, FilledQty: 0},
			}},
			{Price: mustMoney(t, "101.00"), Orders: []*Order{
				{ID: "ask2", Quantity: 100, FilledQty: 0},
			}},
		},
	}
	buyOrder := &Order{
		ID: "buy1", Side: core.OrderSideBuy, Type: core.OrderTypeMarket,
		Quantity: 120, Status: core.OrderStatusSubmitted,
	}
	fills, err := engine.Match(buyOrder, book)
	if err != nil {
		t.Fatal(err)
	}
	if len(fills) != 2 {
		t.Fatalf("expected 2 fills, got %d", len(fills))
	}
	if fills[0].Quantity != 50 || fills[1].Quantity != 70 {
		t.Fatalf("expected fills 50+70, got %d+%d", fills[0].Quantity, fills[1].Quantity)
	}
	if fills[1].Price.String() != "101.00" {
		t.Fatalf("expected second fill price 101.00, got %s", fills[1].Price.String())
	}
}

func TestMatchingEngine_LimitBuy_CrossesThenRests(t *testing.T) {
	engine := NewMatchingEngineWithSeed(DefaultFillConfig(), 42)
	book := &OrderBook{
		Symbol: "AAPL",
		Asks: []PriceLevel{
			{Price: mustMoney(t, "99.00"), Orders: []*Order{
				{ID: "ask1", Quantity: 50, FilledQty: 0},
			}},
		},
	}
	// Limit buy @ 100: crosses ask @ 99
	buyOrder := &Order{
		ID: "buy1", Side: core.OrderSideBuy, Type: core.OrderTypeLimit,
		Price: mustMoney(t, "100.00"), Quantity: 100, Status: core.OrderStatusSubmitted,
	}
	fills, err := engine.Match(buyOrder, book)
	if err != nil {
		t.Fatal(err)
	}
	if len(fills) != 1 {
		t.Fatalf("expected 1 fill, got %d", len(fills))
	}
	if fills[0].Price.String() != "99.00" {
		t.Fatalf("expected price 99.00, got %s", fills[0].Price.String())
	}
}

func TestStopBook_StopLoss_TriggersOnPriceDrop(t *testing.T) {
	sb := &StopBook{}
	stopOrder := &Order{
		ID: "stop1", Side: core.OrderSideSell, Type: core.OrderTypeStop,
		StopPrice: mustMoney(t, "90.00"), Quantity: 100, Status: core.OrderStatusSubmitted,
	}
	sb.AddStop(stopOrder)

	activated := sb.CheckTriggers(mustMoney(t, "89.50"))
	if len(activated) != 1 {
		t.Fatalf("expected 1 activated order, got %d", len(activated))
	}
	if activated[0].Type != core.OrderTypeMarket {
		t.Fatal("stop should convert to market order")
	}
}

func TestStopBook_StopLoss_NotTriggeredAboveStop(t *testing.T) {
	sb := &StopBook{}
	stopOrder := &Order{
		ID: "stop1", Side: core.OrderSideSell, Type: core.OrderTypeStop,
		StopPrice: mustMoney(t, "90.00"), Quantity: 100, Status: core.OrderStatusSubmitted,
	}
	sb.AddStop(stopOrder)

	activated := sb.CheckTriggers(mustMoney(t, "90.50"))
	if len(activated) != 0 {
		t.Fatalf("expected 0 activated orders, got %d", len(activated))
	}
}

func TestStopBook_StopBuy_TriggersOnPriceRise(t *testing.T) {
	sb := &StopBook{}
	stopOrder := &Order{
		ID: "stop1", Side: core.OrderSideBuy, Type: core.OrderTypeStop,
		StopPrice: mustMoney(t, "110.00"), Quantity: 100, Status: core.OrderStatusSubmitted,
	}
	sb.AddStop(stopOrder)

	activated := sb.CheckTriggers(mustMoney(t, "110.50"))
	if len(activated) != 1 {
		t.Fatalf("expected 1 activated order, got %d", len(activated))
	}
}

func TestStopBook_AlreadyTriggered_NoDuplicate(t *testing.T) {
	sb := &StopBook{}
	stopOrder := &Order{
		ID: "stop1", Side: core.OrderSideSell, Type: core.OrderTypeStop,
		StopPrice: mustMoney(t, "90.00"), Quantity: 100, Status: core.OrderStatusSubmitted,
	}
	sb.AddStop(stopOrder)
	sb.CheckTriggers(mustMoney(t, "89.00")) // triggers
	activated := sb.CheckTriggers(mustMoney(t, "85.00")) // should not re-trigger
	if len(activated) != 0 {
		t.Fatal("should not re-trigger already triggered stop")
	}
}
