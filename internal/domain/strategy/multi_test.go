package strategy

import (
	"context"
	"testing"
	"time"

	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/market"
)

func TestMultiStrategy_DispatchesToCorrectStrategy(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	s1 := NewPriceCrosses("cross1", "AAPL", mustMoney(t, "100.00"), 100, core.OrderTypeMarket)
	s2 := NewPriceCrosses("cross2", "GOOG", mustMoney(t, "200.00"), 50, core.OrderTypeMarket)

	m := NewMultiStrategy(s1, s2)

	tick := market.Tick{Symbol: "GOOG", Price: mustMoney(t, "201.00"), Timestamp: now}
	sig, err := m.OnTick(ctx, tick)
	if err != nil {
		t.Fatal(err)
	}
	if sig == nil {
		t.Fatal("expected signal from GOOG strategy")
	}
	if sig.Symbol != "GOOG" {
		t.Fatalf("expected GOOG signal, got %s", sig.Symbol)
	}
}

func TestMultiStrategy_NoSignal(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	s1 := NewPriceCrosses("cross1", "AAPL", mustMoney(t, "100.00"), 100, core.OrderTypeMarket)
	m := NewMultiStrategy(s1)

	tick := market.Tick{Symbol: "AAPL", Price: mustMoney(t, "99.00"), Timestamp: now}
	sig, err := m.OnTick(ctx, tick)
	if err != nil {
		t.Fatal(err)
	}
	if sig != nil {
		t.Fatal("expected no signal")
	}
}

func TestMultiStrategy_Name(t *testing.T) {
	m := NewMultiStrategy()
	if m.Name() != "multi" {
		t.Fatalf("expected 'multi', got %s", m.Name())
	}
}

func TestMultiStrategy_Reset(t *testing.T) {
	s1 := NewPriceCrosses("cross1", "AAPL", mustMoney(t, "100.00"), 100, core.OrderTypeMarket)
	m := NewMultiStrategy(s1)
	m.Reset()
}
