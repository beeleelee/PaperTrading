package core

import (
	"testing"
)

func TestNewMoney_ValidString(t *testing.T) {
	m, err := NewMoney("123.45")
	if err != nil {
		t.Fatal(err)
	}
	if m.String() != "123.45" {
		t.Fatalf("expected 123.45, got %s", m.String())
	}
}

func TestNewMoney_InvalidString(t *testing.T) {
	_, err := NewMoney("not-a-number")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMoney_Add(t *testing.T) {
	a, _ := NewMoney("100.00")
	b, _ := NewMoney("50.50")
	sum := a.Add(b)
	if sum.String() != "150.50" {
		t.Fatalf("expected 150.50, got %s", sum.String())
	}
}

func TestMoney_Sub(t *testing.T) {
	a, _ := NewMoney("100.00")
	b, _ := NewMoney("30.00")
	diff := a.Sub(b)
	if diff.String() != "70.00" {
		t.Fatalf("expected 70.00, got %s", diff.String())
	}
}

func TestMoney_Mul(t *testing.T) {
	a, _ := NewMoney("10.50")
	product := a.Mul(3)
	if product.String() != "31.50" {
		t.Fatalf("expected 31.50, got %s", product.String())
	}
}

func TestMoney_Div(t *testing.T) {
	a, _ := NewMoney("100.00")
	q := a.Div(3)
	if q.String() != "33.33" {
		t.Fatalf("expected 33.33, got %s", q.String())
	}
}

func TestMoney_Comparisons(t *testing.T) {
	a, _ := NewMoney("100.00")
	b, _ := NewMoney("200.00")
	c, _ := NewMoney("100.00")

	if !a.LessThan(b) {
		t.Fatal("100 < 200 should be true")
	}
	if !b.GreaterThan(a) {
		t.Fatal("200 > 100 should be true")
	}
	if !a.LessThanOrEqual(c) {
		t.Fatal("100 <= 100 should be true")
	}
	if !a.GreaterThanOrEqual(c) {
		t.Fatal("100 >= 100 should be true")
	}
}

func TestMoney_IsZero(t *testing.T) {
	zero, _ := NewMoney("0.00")
	if !zero.IsZero() {
		t.Fatal("should be zero")
	}
	nonZero, _ := NewMoney("0.01")
	if nonZero.IsZero() {
		t.Fatal("should not be zero")
	}
}

func TestSymbol_Valid(t *testing.T) {
	if Symbol("AAPL").Valid() != true {
		t.Fatal("AAPL should be valid")
	}
	if UnknownSymbol.Valid() != false {
		t.Fatal("unknown symbol should be invalid")
	}
}

func TestOrderType_Valid(t *testing.T) {
	if OrderTypeMarket.Valid() != true {
		t.Fatal("market should be valid")
	}
	if OrderTypeUnknown.Valid() != false {
		t.Fatal("unknown should be invalid")
	}
}

func TestOrderSide_Inverse(t *testing.T) {
	if OrderSideBuy.Inverse() != OrderSideSell {
		t.Fatal("buy inverse should be sell")
	}
	if OrderSideSell.Inverse() != OrderSideBuy {
		t.Fatal("sell inverse should be buy")
	}
	if OrderSideUnknown.Inverse() != OrderSideUnknown {
		t.Fatal("unknown inverse should be unknown")
	}
}

func TestOrderStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		from OrderStatus
		to   OrderStatus
		ok   bool
	}{
		{OrderStatusCreated, OrderStatusSubmitted, true},
		{OrderStatusCreated, OrderStatusCancelled, true},
		{OrderStatusCreated, OrderStatusRejected, true},
		{OrderStatusCreated, OrderStatusFilled, false},
		{OrderStatusSubmitted, OrderStatusPartiallyFilled, true},
		{OrderStatusSubmitted, OrderStatusFilled, true},
		{OrderStatusSubmitted, OrderStatusCancelled, true},
		{OrderStatusPartiallyFilled, OrderStatusFilled, true},
		{OrderStatusPartiallyFilled, OrderStatusCancelled, true},
		{OrderStatusFilled, OrderStatusCancelled, false},
		{OrderStatusFilled, OrderStatusPartiallyFilled, false},
		{OrderStatusCancelled, OrderStatusSubmitted, false},
		{OrderStatusRejected, OrderStatusSubmitted, false},
	}
	for _, tt := range tests {
		got := tt.from.CanTransitionTo(tt.to)
		if got != tt.ok {
			t.Errorf("CanTransitionTo(%s -> %s): expected %v, got %v", tt.from, tt.to, tt.ok, got)
		}
	}
}
