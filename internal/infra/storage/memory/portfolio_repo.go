package memory

import (
	"context"
	"sync"
	"github.com/felix/papertrading/internal/domain/portfolio"
)

type PortfolioRepository struct {
	mu         sync.RWMutex
	portfolios map[portfolio.PortfolioID]*portfolio.Portfolio
}

func NewPortfolioRepository() *PortfolioRepository {
	return &PortfolioRepository{
		portfolios: make(map[portfolio.PortfolioID]*portfolio.Portfolio),
	}
}

func (r *PortfolioRepository) Save(_ context.Context, p *portfolio.Portfolio) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	clone := *p
	r.portfolios[p.ID] = &clone
	return nil
}

func (r *PortfolioRepository) FindByID(_ context.Context, id portfolio.PortfolioID) (*portfolio.Portfolio, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.portfolios[id]
	if !ok {
		return nil, portfolio.ErrPositionNotFound
	}
	clone := *p
	return &clone, nil
}
