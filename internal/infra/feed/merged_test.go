package feed

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/felix/papertrading/internal/domain/market"
)

func writeCSV(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMergedFeed_SingleFeed(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "a.csv", "ts,price\n2024-01-02 09:30:00,100.00\n2024-01-02 09:35:00,101.00")

	f := NewMergedFeed(NewCSVFeed(csv, WithSkipRows(1), WithSymbol("AAPL")))
	ctx := context.Background()
	ch := make(chan market.Tick, 10)
	go f.Stream(ctx, ch)

	var ticks []market.Tick
	for t := range ch {
		ticks = append(ticks, t)
	}
	if len(ticks) != 2 {
		t.Fatalf("expected 2 ticks, got %d", len(ticks))
	}
}

func TestMergedFeed_MultipleFeeds(t *testing.T) {
	dir := t.TempDir()
	csv1 := writeCSV(t, dir, "a.csv", "ts,price\n2024-01-02 09:35:00,101.00\n2024-01-02 09:40:00,102.00")
	csv2 := writeCSV(t, dir, "b.csv", "ts,price\n2024-01-02 09:30:00,100.00\n2024-01-02 09:45:00,103.00")

	f := NewMergedFeed(
		NewCSVFeed(csv1, WithSkipRows(1), WithSymbol("AAPL")),
		NewCSVFeed(csv2, WithSkipRows(1), WithSymbol("GOOG")),
	)
	ctx := context.Background()
	ch := make(chan market.Tick, 10)
	go f.Stream(ctx, ch)

	var ticks []market.Tick
	for t := range ch {
		ticks = append(ticks, t)
	}
	if len(ticks) != 4 {
		t.Fatalf("expected 4 ticks, got %d", len(ticks))
	}

	// Should be sorted by timestamp: 09:30 (GOOG), 09:35 (AAPL), 09:40 (AAPL), 09:45 (GOOG)
	if ticks[0].Symbol != "GOOG" || ticks[0].Price.String() != "100.00" {
		t.Fatalf("expected GOOG 100.00 first, got %s %s", ticks[0].Symbol, ticks[0].Price)
	}
	if ticks[1].Symbol != "AAPL" || ticks[1].Price.String() != "101.00" {
		t.Fatalf("expected AAPL 101.00 second, got %s %s", ticks[1].Symbol, ticks[1].Price)
	}
}

func TestMergedFeed_Empty(t *testing.T) {
	f := NewMergedFeed()
	ctx := context.Background()
	ch := make(chan market.Tick, 10)
	go f.Stream(ctx, ch)

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected closed channel")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for close")
	}
}
