package order

import (
	"context"
	"github.com/felix/papertrading/internal/domain/core"
)

type Repository interface {
	Save(ctx context.Context, o *Order) error
	FindByID(ctx context.Context, id OrderID) (*Order, error)
	FindByStatus(ctx context.Context, status core.OrderStatus) ([]*Order, error)
	FindByPortfolioID(ctx context.Context, portfolioID string) ([]*Order, error)
}
