package portfolio

import "context"

type Repository interface {
	Save(ctx context.Context, p *Portfolio) error
	FindByID(ctx context.Context, id PortfolioID) (*Portfolio, error)
}
