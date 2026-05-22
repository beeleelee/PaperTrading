package memory

import (
	"context"
	"sync"
	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/order"
)

type OrderRepository struct {
	mu     sync.RWMutex
	orders map[order.OrderID]*order.Order
}

func NewOrderRepository() *OrderRepository {
	return &OrderRepository{
		orders: make(map[order.OrderID]*order.Order),
	}
}

func (r *OrderRepository) Save(_ context.Context, o *order.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	clone := *o
	r.orders[o.ID] = &clone
	return nil
}

func (r *OrderRepository) FindByID(_ context.Context, id order.OrderID) (*order.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	o, ok := r.orders[id]
	if !ok {
		return nil, order.ErrOrderNotFound
	}
	clone := *o
	return &clone, nil
}

func (r *OrderRepository) FindByStatus(_ context.Context, status core.OrderStatus) ([]*order.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*order.Order
	for _, o := range r.orders {
		if o.Status == status {
			clone := *o
			result = append(result, &clone)
		}
	}
	return result, nil
}

func (r *OrderRepository) FindByPortfolioID(_ context.Context, portfolioID string) ([]*order.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*order.Order
	for _, o := range r.orders {
		if o.PortfolioID == portfolioID {
			clone := *o
			result = append(result, &clone)
		}
	}
	return result, nil
}
