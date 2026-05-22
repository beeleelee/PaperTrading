package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/order"
)

type OrderRepository struct {
	db *DB
}

func NewOrderRepository(db *DB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) Save(ctx context.Context, o *order.Order) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO orders (id, portfolio_id, symbol, side, type, status, price, stop_price, quantity, filled_qty, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status=excluded.status, filled_qty=excluded.filled_qty, updated_at=excluded.updated_at
	`,
		string(o.ID), o.PortfolioID, string(o.Symbol),
		int(o.Side), int(o.Type), int(o.Status),
		o.Price.String(), o.StopPrice.String(),
		o.Quantity, o.FilledQty,
		o.CreatedAt.UTC().Format(time.RFC3339),
		o.UpdatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("save order: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, `DELETE FROM fills WHERE order_id = ?`, string(o.ID)); err != nil {
		return fmt.Errorf("clear fills: %w", err)
	}
	for _, f := range o.Fills {
		if _, err := r.db.ExecContext(ctx, `
			INSERT INTO fills (id, order_id, price, quantity, commission, timestamp)
			VALUES (?, ?, ?, ?, ?, ?)
		`, string(f.ID), string(o.ID), f.Price.String(), f.Quantity, f.Commission.String(),
			f.Timestamp.UTC().Format(time.RFC3339),
		); err != nil {
			return fmt.Errorf("save fill: %w", err)
		}
	}
	return nil
}

func (r *OrderRepository) FindByID(ctx context.Context, id order.OrderID) (*order.Order, error) {
	o, err := r.scanOrder(ctx, r.db.QueryRowContext(ctx, `
		SELECT id, portfolio_id, symbol, side, type, status, price, stop_price, quantity, filled_qty, created_at, updated_at
		FROM orders WHERE id = ?
	`, string(id)))
	if err != nil {
		return nil, err
	}
	fills, err := r.scanFills(ctx, id)
	if err != nil {
		return nil, err
	}
	o.Fills = fills
	return o, nil
}

func (r *OrderRepository) FindByStatus(ctx context.Context, status core.OrderStatus) ([]*order.Order, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, portfolio_id, symbol, side, type, status, price, stop_price, quantity, filled_qty, created_at, updated_at
		FROM orders WHERE status = ?
	`, int(status))
	if err != nil {
		return nil, fmt.Errorf("find by status: %w", err)
	}
	orders, err := r.scanOrders(ctx, rows)
	rows.Close()
	if err != nil {
		return nil, err
	}
	return r.loadFills(ctx, orders)
}

func (r *OrderRepository) FindByPortfolioID(ctx context.Context, portfolioID string) ([]*order.Order, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, portfolio_id, symbol, side, type, status, price, stop_price, quantity, filled_qty, created_at, updated_at
		FROM orders WHERE portfolio_id = ?
	`, portfolioID)
	if err != nil {
		return nil, fmt.Errorf("find by portfolio: %w", err)
	}
	orders, err := r.scanOrders(ctx, rows)
	rows.Close()
	if err != nil {
		return nil, err
	}
	return r.loadFills(ctx, orders)
}

func (r *OrderRepository) scanOrder(ctx context.Context, row *sql.Row) (*order.Order, error) {
	var (
		id, portfolioID, symbol, price, stopPrice, createdAt, updatedAt string
		side, otype, status                                             int
		quantity, filledQty                                             int64
	)
	if err := row.Scan(&id, &portfolioID, &symbol, &side, &otype, &status, &price, &stopPrice, &quantity, &filledQty, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, order.ErrOrderNotFound
		}
		return nil, fmt.Errorf("scan order: %w", err)
	}
	return r.buildOrder(id, portfolioID, symbol, side, otype, status, price, stopPrice, quantity, filledQty, createdAt, updatedAt)
}

func (r *OrderRepository) scanOrders(ctx context.Context, rows *sql.Rows) ([]*order.Order, error) {
	var orders []*order.Order
	for rows.Next() {
		var (
			id, portfolioID, symbol, price, stopPrice, createdAt, updatedAt string
			side, otype, status                                             int
			quantity, filledQty                                             int64
		)
		if err := rows.Scan(&id, &portfolioID, &symbol, &side, &otype, &status, &price, &stopPrice, &quantity, &filledQty, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan order row: %w", err)
		}
		o, err := r.buildOrder(id, portfolioID, symbol, side, otype, status, price, stopPrice, quantity, filledQty, createdAt, updatedAt)
		if err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

func (r *OrderRepository) loadFills(ctx context.Context, orders []*order.Order) ([]*order.Order, error) {
	for _, o := range orders {
		fills, err := r.scanFills(ctx, o.ID)
		if err != nil {
			return nil, err
		}
		o.Fills = fills
	}
	return orders, nil
}

func (r *OrderRepository) buildOrder(id, portfolioID, symbol string, side, otype, status int, price, stopPrice string, quantity, filledQty int64, createdAt, updatedAt string) (*order.Order, error) {
	mPrice, err := core.NewMoney(price)
	if err != nil {
		return nil, fmt.Errorf("parse price: %w", err)
	}
	mStop, err := core.NewMoney(stopPrice)
	if err != nil {
		return nil, fmt.Errorf("parse stop price: %w", err)
	}
	ct, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	ut, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return &order.Order{
		ID:          order.OrderID(id),
		PortfolioID: portfolioID,
		Symbol:      core.Symbol(symbol),
		Side:        core.OrderSide(side),
		Type:       core.OrderType(otype),
		Status:      core.OrderStatus(status),
		Price:       mPrice,
		StopPrice:   mStop,
		Quantity:    quantity,
		FilledQty:   filledQty,
		CreatedAt:   ct,
		UpdatedAt:   ut,
	}, nil
}

func (r *OrderRepository) scanFills(ctx context.Context, orderID order.OrderID) ([]order.Fill, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, order_id, price, quantity, commission, timestamp
		FROM fills WHERE order_id = ?
	`, string(orderID))
	if err != nil {
		return nil, fmt.Errorf("query fills: %w", err)
	}
	defer rows.Close()

	var fills []order.Fill
	for rows.Next() {
		var id, oid, price, commission, ts string
		var quantity int64
		if err := rows.Scan(&id, &oid, &price, &quantity, &commission, &ts); err != nil {
			return nil, fmt.Errorf("scan fill: %w", err)
		}
		mPrice, err := core.NewMoney(price)
		if err != nil {
			return nil, fmt.Errorf("parse fill price: %w", err)
		}
		mComm, err := core.NewMoney(commission)
		if err != nil {
			return nil, fmt.Errorf("parse fill commission: %w", err)
		}
		t, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			return nil, fmt.Errorf("parse fill timestamp: %w", err)
		}
		fills = append(fills, order.Fill{
			ID:         order.FillID(id),
			OrderID:    order.OrderID(oid),
			Price:      mPrice,
			Quantity:   quantity,
			Commission: mComm,
			Timestamp:  t,
		})
	}
	return fills, rows.Err()
}
