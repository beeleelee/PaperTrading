package feed

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"github.com/felix/papertrading/internal/domain/market"
)

func TestCSVFeed_ReadsTicks(t *testing.T) {
	csvContent := `timestamp,price,volume
2024-01-02 09:30:00,185.50,1500
2024-01-02 09:35:00,186.20,2300`

	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatal(err)
	}

	feed := NewCSVFeed(csvPath, WithSkipRows(1), WithSymbol("AAPL"))
	ctx := context.Background()
	tickCh := make(chan market.Tick, 10)

	go func() {
		if err := feed.Stream(ctx, tickCh); err != nil {
			t.Error(err)
		}
	}()

	var ticks []market.Tick
	for tick := range tickCh {
		ticks = append(ticks, tick)
	}

	if len(ticks) != 2 {
		t.Fatalf("expected 2 ticks, got %d", len(ticks))
	}
	if ticks[0].Price.String() != "185.50" {
		t.Fatalf("expected price 185.50, got %s", ticks[0].Price)
	}
	if ticks[1].Price.String() != "186.20" {
		t.Fatalf("expected price 186.20, got %s", ticks[1].Price)
	}
}

func TestCSVFeed_WithSymbolColumn(t *testing.T) {
	csvContent := `timestamp,symbol,price,volume
2024-01-02 09:30:00,AAPL,185.50,1500`

	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatal(err)
	}

	feed := NewCSVFeed(csvPath, WithSkipRows(1))
	ctx := context.Background()
	tickCh := make(chan market.Tick, 10)

	go feed.Stream(ctx, tickCh)
	tick := <-tickCh

	if tick.Symbol != "AAPL" {
		t.Fatalf("expected AAPL, got %s", tick.Symbol)
	}
}

func TestCSVFeed_CancelledContext(t *testing.T) {
	csvContent := `timestamp,price
2024-01-02 09:30:00,185.50
2024-01-02 09:35:00,186.20`

	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	feed := NewCSVFeed(csvPath, WithSkipRows(1), WithSymbol("AAPL"))
	tickCh := make(chan market.Tick, 10)

	cancel()

	err := feed.Stream(ctx, tickCh)
	t.Logf("err: %v", err)
	if err == nil {
		t.Fatal("expected context cancelled error")
	}
}
