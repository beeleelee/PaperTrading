package sqlite

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/order"
	"github.com/felix/papertrading/internal/domain/portfolio"
)

func mustMoney(t *testing.T, s string) core.Money {
	t.Helper()
	m, err := core.NewMoney(s)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func newTestDB(t *testing.T) *DB {
	t.Helper()
	f, err := os.CreateTemp("", "papertrade-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	db, err := Open(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(); os.Remove(f.Name()) })
	return db
}

func TestOrderRepository_SaveAndFind(t *testing.T) {
	db := newTestDB(t)
	repo := NewOrderRepository(db)
	ctx := context.Background()

	now := time.Now().UTC()
	o, err := order.NewOrder("ord1", "p1", "AAPL", core.OrderSideBuy, core.OrderTypeMarket, mustMoney(t, "0"), mustMoney(t, "0"), 100, now)
	if err != nil {
		t.Fatal(err)
	}
	o.Submit(now)

	fill := order.Fill{
		ID:         "f1",
		OrderID:    o.ID,
		Price:      mustMoney(t, "150.00"),
		Quantity:   100,
		Commission: mustMoney(t, "0"),
		Timestamp:  now,
	}
	o.ApplyFill(fill, now)

	if err := repo.Save(ctx, o); err != nil {
		t.Fatal(err)
	}

	loaded, err := repo.FindByID(ctx, "ord1")
	if err != nil {
		t.Fatal(err)
	}

	if loaded.ID != "ord1" {
		t.Fatalf("expected ord1, got %s", loaded.ID)
	}
	if loaded.Symbol != "AAPL" {
		t.Fatalf("expected AAPL, got %s", loaded.Symbol)
	}
	if loaded.Status != core.OrderStatusFilled {
		t.Fatalf("expected filled, got %s", loaded.Status)
	}
	if len(loaded.Fills) != 1 {
		t.Fatalf("expected 1 fill, got %d", len(loaded.Fills))
	}
	if loaded.Fills[0].Price.String() != "150.00" {
		t.Fatalf("expected fill price 150.00, got %s", loaded.Fills[0].Price)
	}
}

func TestOrderRepository_FindByStatus(t *testing.T) {
	db := newTestDB(t)
	repo := NewOrderRepository(db)
	ctx := context.Background()
	now := time.Now().UTC()

	o1, _ := order.NewOrder("o1", "p1", "AAPL", core.OrderSideBuy, core.OrderTypeMarket, mustMoney(t, "0"), mustMoney(t, "0"), 100, now)
	o2, _ := order.NewOrder("o2", "p1", "AAPL", core.OrderSideBuy, core.OrderTypeMarket, mustMoney(t, "0"), mustMoney(t, "0"), 50, now)
	o1.Submit(now)
	o2.Submit(now)
	repo.Save(ctx, o1)
	repo.Save(ctx, o2)

	orders, err := repo.FindByStatus(ctx, core.OrderStatusSubmitted)
	if err != nil {
		t.Fatal(err)
	}
	if len(orders) != 2 {
		t.Fatalf("expected 2 submitted orders, got %d", len(orders))
	}
}

func TestOrderRepository_FindByPortfolioID(t *testing.T) {
	db := newTestDB(t)
	repo := NewOrderRepository(db)
	ctx := context.Background()
	now := time.Now().UTC()

	o1, _ := order.NewOrder("o1", "pf1", "AAPL", core.OrderSideBuy, core.OrderTypeMarket, mustMoney(t, "0"), mustMoney(t, "0"), 100, now)
	o2, _ := order.NewOrder("o2", "pf1", "GOOG", core.OrderSideBuy, core.OrderTypeMarket, mustMoney(t, "0"), mustMoney(t, "0"), 50, now)
	o3, _ := order.NewOrder("o3", "pf2", "MSFT", core.OrderSideBuy, core.OrderTypeMarket, mustMoney(t, "0"), mustMoney(t, "0"), 30, now)
	repo.Save(ctx, o1)
	repo.Save(ctx, o2)
	repo.Save(ctx, o3)

	orders, err := repo.FindByPortfolioID(ctx, "pf1")
	if err != nil {
		t.Fatal(err)
	}
	if len(orders) != 2 {
		t.Fatalf("expected 2 orders for pf1, got %d", len(orders))
	}
}

func TestOrderRepository_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := NewOrderRepository(db)
	_, err := repo.FindByID(context.Background(), "nonexistent")
	if err != order.ErrOrderNotFound {
		t.Fatalf("expected ErrOrderNotFound, got %v", err)
	}
}

func TestPortfolioRepository_SaveAndFind(t *testing.T) {
	db := newTestDB(t)
	repo := NewPortfolioRepository(db)
	ctx := context.Background()
	now := time.Now().UTC()

	p := portfolio.NewPortfolio("pf1", mustMoney(t, "100000.00"), now)
	p.OpenPosition("AAPL", mustMoney(t, "150.00"), 100, now)
	p.OpenPosition("GOOG", mustMoney(t, "200.00"), 50, now)

	if err := repo.Save(ctx, p); err != nil {
		t.Fatal(err)
	}

	loaded, err := repo.FindByID(ctx, "pf1")
	if err != nil {
		t.Fatal(err)
	}

	if loaded.ID != "pf1" {
		t.Fatalf("expected pf1, got %s", loaded.ID)
	}
	if loaded.Cash.String() != "75000.00" {
		t.Fatalf("expected cash 75000.00, got %s", loaded.Cash)
	}
	if loaded.PositionCount() != 2 {
		t.Fatalf("expected 2 positions, got %d", loaded.PositionCount())
	}
	if loaded.Positions["AAPL"].Quantity != 100 {
		t.Fatalf("expected 100 AAPL, got %d", loaded.Positions["AAPL"].Quantity)
	}
}

func TestPortfolioRepository_WithClosedTrades(t *testing.T) {
	db := newTestDB(t)
	repo := NewPortfolioRepository(db)
	ctx := context.Background()
	now := time.Now().UTC()

	p := portfolio.NewPortfolio("pf1", mustMoney(t, "100000.00"), now)
	p.OpenPosition("AAPL", mustMoney(t, "100.00"), 100, now)
	p.ClosePosition("AAPL", mustMoney(t, "120.00"), now)

	if err := repo.Save(ctx, p); err != nil {
		t.Fatal(err)
	}

	loaded, err := repo.FindByID(ctx, "pf1")
	if err != nil {
		t.Fatal(err)
	}

	if loaded.RealizedPnL().String() != "2000.00" {
		t.Fatalf("expected realized PnL 2000.00, got %s", loaded.RealizedPnL())
	}
	if len(loaded.ClosedTrades()) != 1 {
		t.Fatalf("expected 1 closed trade, got %d", len(loaded.ClosedTrades()))
	}
	if loaded.ClosedTrades()[0].PnL.String() != "2000.00" {
		t.Fatalf("expected trade PnL 2000.00, got %s", loaded.ClosedTrades()[0].PnL)
	}
}

func TestPortfolioRepository_Overwrite(t *testing.T) {
	db := newTestDB(t)
	repo := NewPortfolioRepository(db)
	ctx := context.Background()
	now := time.Now().UTC()

	p := portfolio.NewPortfolio("pf1", mustMoney(t, "100000.00"), now)
	p.OpenPosition("AAPL", mustMoney(t, "100.00"), 100, now)
	repo.Save(ctx, p)

	p.ClosePosition("AAPL", mustMoney(t, "120.00"), now)
	p.OpenPosition("GOOG", mustMoney(t, "50.00"), 200, now)
	repo.Save(ctx, p)

	loaded, err := repo.FindByID(ctx, "pf1")
	if err != nil {
		t.Fatal(err)
	}

	if loaded.PositionCount() != 1 {
		t.Fatalf("expected 1 position, got %d", loaded.PositionCount())
	}
	if !loaded.HasPosition("GOOG") {
		t.Fatal("expected GOOG position")
	}
	if loaded.RealizedPnL().String() != "2000.00" {
		t.Fatalf("expected realized PnL 2000.00, got %s", loaded.RealizedPnL())
	}
}

func TestPortfolioRepository_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := NewPortfolioRepository(db)
	_, err := repo.FindByID(context.Background(), "nonexistent")
	if err != portfolio.ErrPositionNotFound {
		t.Fatalf("expected ErrPositionNotFound, got %v", err)
	}
}

func TestMigration(t *testing.T) {
	db := newTestDB(t)

	var version int
	if err := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM _migrations").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("expected migration version 1, got %d", version)
	}
}
