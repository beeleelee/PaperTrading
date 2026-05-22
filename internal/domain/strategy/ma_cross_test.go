package strategy

import (
	"context"
	"testing"
	"time"
	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/market"
)

func mustMoney(t *testing.T, s string) core.Money {
	t.Helper()
	m, err := core.NewMoney(s)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestMovingAverageCross_BuySignal(t *testing.T) {
	s := NewMovingAverageCross("test", "AAPL", 3, 5, 100)
	ctx := context.Background()
	baseTime := time.Now()

	prices := []string{
		"100.00", "101.00", "102.00", // warm up fast (3)
		"100.00", "99.00", "98.00",   // warm up slow (5) + downtrend
		"99.00", "100.00", "101.00",  // fast crosses above slow
	}

	var lastSignal *Signal
	for i, p := range prices {
		price := mustMoney(t, p)
		tick := market.Tick{Symbol: "AAPL", Price: price, Timestamp: baseTime.Add(time.Duration(i) * time.Minute)}
		sig, err := s.OnTick(ctx, tick)
		if err != nil {
			t.Fatal(err)
		}
		if sig != nil {
			lastSignal = sig
		}
	}

	if lastSignal == nil {
		t.Fatal("expected a buy signal")
	}
	if lastSignal.Type != SignalBuy {
		t.Fatalf("expected buy signal, got %d", lastSignal.Type)
	}
	if lastSignal.Symbol != "AAPL" {
		t.Fatalf("expected AAPL, got %s", lastSignal.Symbol)
	}
	if lastSignal.Quantity != 100 {
		t.Fatalf("expected qty 100, got %d", lastSignal.Quantity)
	}
}

func TestMovingAverageCross_SellSignal(t *testing.T) {
	s := NewMovingAverageCross("test", "AAPL", 3, 5, 100)
	ctx := context.Background()
	baseTime := time.Now()

	prices := []string{
		"100.00", "101.00", "102.00", // warm up fast (3)
		"103.00", "104.00", "105.00", // warm up slow (5) + uptrend
		"104.00", "103.00", "102.00", // fast crosses below slow
	}

	var lastSignal *Signal
	for i, p := range prices {
		price := mustMoney(t, p)
		tick := market.Tick{Symbol: "AAPL", Price: price, Timestamp: baseTime.Add(time.Duration(i) * time.Minute)}
		sig, err := s.OnTick(ctx, tick)
		if err != nil {
			t.Fatal(err)
		}
		if sig != nil {
			lastSignal = sig
		}
	}

	if lastSignal == nil {
		t.Fatal("expected a sell signal")
	}
	if lastSignal.Type != SignalSell {
		t.Fatalf("expected sell signal, got %d", lastSignal.Type)
	}
}

func TestMovingAverageCross_NoSignalDuringWarmup(t *testing.T) {
	s := NewMovingAverageCross("test", "AAPL", 10, 20, 100)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		price := mustMoney(t, "100.00")
		tick := market.Tick{Symbol: "AAPL", Price: price, Timestamp: time.Now()}
		sig, err := s.OnTick(ctx, tick)
		if err != nil {
			t.Fatal(err)
		}
		if sig != nil {
			t.Fatal("expected no signal during warmup")
		}
	}
}

func TestMovingAverageCross_Reset(t *testing.T) {
	s := NewMovingAverageCross("test", "AAPL", 3, 5, 100)
	ctx := context.Background()

	// Send some ticks to warm up
	for i := 0; i < 6; i++ {
		price := mustMoney(t, "100.00")
		tick := market.Tick{Symbol: "AAPL", Price: price, Timestamp: time.Now()}
		s.OnTick(ctx, tick)
	}

	s.Reset()

	// After reset, should need warmup again
	for i := 0; i < 2; i++ {
		price := mustMoney(t, "100.00")
		tick := market.Tick{Symbol: "AAPL", Price: price, Timestamp: time.Now()}
		sig, err := s.OnTick(ctx, tick)
		if err != nil {
			t.Fatal(err)
		}
		if sig != nil {
			t.Fatal("expected no signal after reset during warmup")
		}
	}
}

func TestMovingAverageCross_WrongSymbol(t *testing.T) {
	s := NewMovingAverageCross("test", "AAPL", 3, 5, 100)
	ctx := context.Background()

	tick := market.Tick{Symbol: "GOOG", Price: mustMoney(t, "100.00"), Timestamp: time.Now()}
	sig, err := s.OnTick(ctx, tick)
	if err != nil {
		t.Fatal(err)
	}
	if sig != nil {
		t.Fatal("expected no signal for wrong symbol")
	}
}

func TestPriceCrosses_Threshold(t *testing.T) {
	threshold := mustMoney(t, "100.00")
	s := NewPriceCrosses("test", "AAPL", threshold, 100, core.OrderTypeMarket)
	ctx := context.Background()

	tick1 := market.Tick{Symbol: "AAPL", Price: mustMoney(t, "99.00"), Timestamp: time.Now()}
	sig, err := s.OnTick(ctx, tick1)
	if err != nil {
		t.Fatal(err)
	}
	if sig != nil {
		t.Fatal("expected no signal on first tick (initial state)")
	}

	tick2 := market.Tick{Symbol: "AAPL", Price: mustMoney(t, "101.00"), Timestamp: time.Now()}
	sig, err = s.OnTick(ctx, tick2)
	if err != nil {
		t.Fatal(err)
	}
	if sig == nil {
		t.Fatal("expected signal on cross above")
	}
	if sig.Type != SignalSell {
		t.Fatalf("expected sell signal when crossing above, got %d", sig.Type)
	}
}

func TestSMA(t *testing.T) {
	sma := NewSMA(3)

	_, ok := sma.Update(mustMoney(t, "10.00"))
	if ok {
		t.Fatal("should not be ready with 1 value")
	}
	_, ok = sma.Update(mustMoney(t, "20.00"))
	if ok {
		t.Fatal("should not be ready with 2 values")
	}
	avg, ok := sma.Update(mustMoney(t, "30.00"))
	if !ok {
		t.Fatal("should be ready with 3 values")
	}
	if avg.String() != "20.00" {
		t.Fatalf("expected SMA 20.00, got %s", avg.String())
	}

	avg, ok = sma.Update(mustMoney(t, "40.00"))
	if !ok {
		t.Fatal("should be ready")
	}
	if avg.String() != "30.00" {
		t.Fatalf("expected SMA 30.00, got %s", avg.String())
	}
}

func TestSignal_ToOrder(t *testing.T) {
	now := time.Now()
	s := Signal{
		Type:      SignalBuy,
		Symbol:    "AAPL",
		Quantity:  100,
		OrderType: core.OrderTypeMarket,
		Timestamp: now,
	}
	o, err := s.ToOrder("p1", now)
	if err != nil {
		t.Fatal(err)
	}
	if o.Symbol != "AAPL" {
		t.Fatalf("expected AAPL, got %s", o.Symbol)
	}
	if o.Side != core.OrderSideBuy {
		t.Fatalf("expected buy, got %s", o.Side)
	}
	if o.Type != core.OrderTypeMarket {
		t.Fatalf("expected market, got %s", o.Type)
	}
	if o.Quantity != 100 {
		t.Fatalf("expected qty 100, got %d", o.Quantity)
	}
}
