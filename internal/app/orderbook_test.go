package app

import (
	"testing"
	"time"

	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/order"
)

func mustMoney(t *testing.T, s string) core.Money {
	t.Helper()
	m, err := core.NewMoney(s)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func mustOrder(t *testing.T, id string, sym core.Symbol, side core.OrderSide, price string, qty int64) *order.Order {
	t.Helper()
	mPrice, _ := core.NewMoney(price)
	o, err := order.NewOrder(order.OrderID(id), "p1", sym, side, core.OrderTypeLimit, mPrice, core.NewMoneyFromInt(0), qty, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestOrderBookManager_AddAndFind(t *testing.T) {
	ob := NewOrderBookManager()
	o := mustOrder(t, "o1", "AAPL", core.OrderSideBuy, "150.00", 100)
	ob.AddOrder(o)

	matches := ob.FindMatch(o, mustMoney(t, "150.00"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
}

func TestOrderBookManager_PerSymbolBooks(t *testing.T) {
	ob := NewOrderBookManager()
	aapl := mustOrder(t, "o1", "AAPL", core.OrderSideBuy, "150.00", 100)
	goog := mustOrder(t, "o2", "GOOG", core.OrderSideBuy, "200.00", 50)
	ob.AddOrder(aapl)
	ob.AddOrder(goog)

	aaplResting := ob.RestingOrders("AAPL")
	if len(aaplResting) != 1 {
		t.Fatalf("expected 1 resting AAPL order, got %d", len(aaplResting))
	}

	googResting := ob.RestingOrders("GOOG")
	if len(googResting) != 1 {
		t.Fatalf("expected 1 resting GOOG order, got %d", len(googResting))
	}

	msftResting := ob.RestingOrders("MSFT")
	if len(msftResting) != 0 {
		t.Fatalf("expected 0 resting MSFT orders, got %d", len(msftResting))
	}
}

func TestOrderBookManager_RestingOrdersSkipsFilled(t *testing.T) {
	ob := NewOrderBookManager()
	o := mustOrder(t, "o1", "AAPL", core.OrderSideBuy, "150.00", 100)

	fill := order.Fill{ID: "f1", OrderID: o.ID, Price: mustMoney(t, "150.00"), Quantity: 100, Timestamp: time.Now()}
	o.ApplyFill(fill, time.Now())

	ob.AddOrder(o)
	resting := ob.RestingOrders("AAPL")
	if len(resting) != 0 {
		t.Fatalf("expected 0 resting orders for filled order, got %d", len(resting))
	}
}

func TestOrderBookManager_MarketMatch(t *testing.T) {
	ob := NewOrderBookManager()
	o := mustOrder(t, "o1", "AAPL", core.OrderSideBuy, "150.00", 100)
	o.Type = core.OrderTypeMarket

	matches := ob.FindMatch(o, mustMoney(t, "155.00"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].Price.String() != "155.00" {
		t.Fatalf("expected price 155.00, got %s", matches[0].Price)
	}
}

func TestOrderBookManager_LimitMatchBuy(t *testing.T) {
	ob := NewOrderBookManager()
	buy := mustOrder(t, "o1", "AAPL", core.OrderSideBuy, "150.00", 100)

	// Price at or below limit -> fill
	matches := ob.FindMatch(buy, mustMoney(t, "150.00"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 match at limit, got %d", len(matches))
	}

	matches = ob.FindMatch(buy, mustMoney(t, "149.00"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 match below limit, got %d", len(matches))
	}

	// Price above limit -> no fill
	matches = ob.FindMatch(buy, mustMoney(t, "151.00"))
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches above limit, got %d", len(matches))
	}
}

func TestOrderBookManager_LimitMatchSell(t *testing.T) {
	ob := NewOrderBookManager()
	sell := mustOrder(t, "o1", "AAPL", core.OrderSideSell, "150.00", 100)

	matches := ob.FindMatch(sell, mustMoney(t, "150.00"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 match at limit, got %d", len(matches))
	}

	matches = ob.FindMatch(sell, mustMoney(t, "151.00"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 match above limit, got %d", len(matches))
	}

	matches = ob.FindMatch(sell, mustMoney(t, "149.00"))
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches below limit, got %d", len(matches))
	}
}

func TestOrderBookManager_RemoveFilled(t *testing.T) {
	ob := NewOrderBookManager()
	o := mustOrder(t, "o1", "AAPL", core.OrderSideBuy, "150.00", 100)
	ob.AddOrder(o)

	if len(ob.RestingOrders("AAPL")) != 1 {
		t.Fatal("expected 1 resting order")
	}

	ob.RemoveFilled(o)
	if len(ob.RestingOrders("AAPL")) != 0 {
		t.Fatal("expected 0 resting orders after remove")
	}
}

func TestOrderBookManager_MultipleRestingOrdersSameSymbol(t *testing.T) {
	ob := NewOrderBookManager()
	o1 := mustOrder(t, "o1", "AAPL", core.OrderSideBuy, "150.00", 100)
	o2 := mustOrder(t, "o2", "AAPL", core.OrderSideBuy, "149.00", 50)
	ob.AddOrder(o1)
	ob.AddOrder(o2)

	resting := ob.RestingOrders("AAPL")
	if len(resting) != 2 {
		t.Fatalf("expected 2 resting orders, got %d", len(resting))
	}
}
