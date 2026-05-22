package order

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

func TestNewOrder_Valid(t *testing.T) {
	now := time.Now()
	o, err := NewOrder(
		"o1", "p1", "AAPL",
		core.OrderSideBuy, core.OrderTypeMarket,
		core.NewMoneyFromInt(0), core.NewMoneyFromInt(0),
		100, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if o.Status != core.OrderStatusCreated {
		t.Fatalf("expected created, got %s", o.Status)
	}
	if o.RemainingQty() != 100 {
		t.Fatalf("expected 100 remaining, got %d", o.RemainingQty())
	}
}

func TestNewOrder_InvalidSymbol(t *testing.T) {
	_, err := NewOrder(
		"o1", "p1", "",
		core.OrderSideBuy, core.OrderTypeMarket,
		core.NewMoneyFromInt(0), core.NewMoneyFromInt(0),
		100, time.Now(),
	)
	if err != core.ErrInvalidSymbol {
		t.Fatal("expected ErrInvalidSymbol")
	}
}

func TestNewOrder_NegativeQuantity(t *testing.T) {
	_, err := NewOrder(
		"o1", "p1", "AAPL",
		core.OrderSideBuy, core.OrderTypeMarket,
		core.NewMoneyFromInt(0), core.NewMoneyFromInt(0),
		-1, time.Now(),
	)
	if err != core.ErrNegativeQuantity {
		t.Fatal("expected ErrNegativeQuantity")
	}
}

func TestOrder_Submit(t *testing.T) {
	now := time.Now()
	o, _ := NewOrder("o1", "p1", "AAPL", core.OrderSideBuy, core.OrderTypeMarket, core.NewMoneyFromInt(0), core.NewMoneyFromInt(0), 100, now)
	if err := o.Submit(now); err != nil {
		t.Fatal(err)
	}
	if o.Status != core.OrderStatusSubmitted {
		t.Fatalf("expected submitted, got %s", o.Status)
	}
}

func TestOrder_Submit_InvalidTransition(t *testing.T) {
	now := time.Now()
	o, _ := NewOrder("o1", "p1", "AAPL", core.OrderSideBuy, core.OrderTypeMarket, core.NewMoneyFromInt(0), core.NewMoneyFromInt(0), 100, now)
	o.Submit(now)
	err := o.Submit(now)
	if err != ErrInvalidStatusTransition {
		t.Fatal("expected ErrInvalidStatusTransition")
	}
}

func TestOrder_ApplyFill_FullFill(t *testing.T) {
	now := time.Now()
	o, _ := NewOrder("o1", "p1", "AAPL", core.OrderSideBuy, core.OrderTypeMarket, core.NewMoneyFromInt(0), core.NewMoneyFromInt(0), 100, now)
	o.Submit(now)

	fill := Fill{ID: "f1", Price: mustMoney(t, "150.00"), Quantity: 100, Timestamp: now}
	if err := o.ApplyFill(fill, now); err != nil {
		t.Fatal(err)
	}
	if o.Status != core.OrderStatusFilled {
		t.Fatalf("expected filled, got %s", o.Status)
	}
	if o.FilledQty != 100 {
		t.Fatalf("expected filled qty 100, got %d", o.FilledQty)
	}
	if !o.IsFilled() {
		t.Fatal("IsFilled should be true")
	}
}

func TestOrder_ApplyFill_PartialFill(t *testing.T) {
	now := time.Now()
	o, _ := NewOrder("o1", "p1", "AAPL", core.OrderSideBuy, core.OrderTypeMarket, core.NewMoneyFromInt(0), core.NewMoneyFromInt(0), 100, now)
	o.Submit(now)

	fill := Fill{ID: "f1", Price: mustMoney(t, "150.00"), Quantity: 40, Timestamp: now}
	if err := o.ApplyFill(fill, now); err != nil {
		t.Fatal(err)
	}
	if o.Status != core.OrderStatusPartiallyFilled {
		t.Fatalf("expected partially_filled, got %s", o.Status)
	}
	if o.RemainingQty() != 60 {
		t.Fatalf("expected 60 remaining, got %d", o.RemainingQty())
	}
}

func TestOrder_ApplyFill_ExceedsQuantity(t *testing.T) {
	now := time.Now()
	o, _ := NewOrder("o1", "p1", "AAPL", core.OrderSideBuy, core.OrderTypeMarket, core.NewMoneyFromInt(0), core.NewMoneyFromInt(0), 100, now)
	o.Submit(now)

	fill := Fill{ID: "f1", Price: mustMoney(t, "150.00"), Quantity: 150, Timestamp: now}
	err := o.ApplyFill(fill, now)
	if err != ErrFillExceedsOrderQty {
		t.Fatal("expected ErrFillExceedsOrderQty")
	}
}

func TestOrder_ApplyFill_NotActive(t *testing.T) {
	now := time.Now()
	o, _ := NewOrder("o1", "p1", "AAPL", core.OrderSideBuy, core.OrderTypeMarket, core.NewMoneyFromInt(0), core.NewMoneyFromInt(0), 100, now)
	o.Status = core.OrderStatusFilled

	fill := Fill{ID: "f1", Price: mustMoney(t, "150.00"), Quantity: 10, Timestamp: now}
	err := o.ApplyFill(fill, now)
	if err != ErrOrderNotActive {
		t.Fatal("expected ErrOrderNotActive")
	}
}

func TestOrder_Cancel(t *testing.T) {
	now := time.Now()
	o, _ := NewOrder("o1", "p1", "AAPL", core.OrderSideBuy, core.OrderTypeMarket, core.NewMoneyFromInt(0), core.NewMoneyFromInt(0), 100, now)
	o.Submit(now)
	if err := o.Cancel(now); err != nil {
		t.Fatal(err)
	}
	if o.Status != core.OrderStatusCancelled {
		t.Fatalf("expected cancelled, got %s", o.Status)
	}
}

func TestOrder_Reject(t *testing.T) {
	now := time.Now()
	o, _ := NewOrder("o1", "p1", "AAPL", core.OrderSideBuy, core.OrderTypeMarket, core.NewMoneyFromInt(0), core.NewMoneyFromInt(0), 100, now)
	if err := o.Reject(now); err != nil {
		t.Fatal(err)
	}
	if o.Status != core.OrderStatusRejected {
		t.Fatalf("expected rejected, got %s", o.Status)
	}
}

func TestOrder_IsActive(t *testing.T) {
	now := time.Now()
	o, _ := NewOrder("o1", "p1", "AAPL", core.OrderSideBuy, core.OrderTypeMarket, core.NewMoneyFromInt(0), core.NewMoneyFromInt(0), 100, now)
	if !o.IsActive() {
		t.Fatal("created order should be active")
	}
	o.Submit(now)
	if !o.IsActive() {
		t.Fatal("submitted order should be active")
	}
	o.Cancel(now)
	if o.IsActive() {
		t.Fatal("cancelled order should not be active")
	}
}
