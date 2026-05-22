package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/portfolio"
)

type PortfolioRepository struct {
	db *DB
}

func NewPortfolioRepository(db *DB) *PortfolioRepository {
	return &PortfolioRepository{db: db}
}

func (r *PortfolioRepository) Save(ctx context.Context, p *portfolio.Portfolio) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO portfolios (id, cash, realized_pnl, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			cash=excluded.cash, realized_pnl=excluded.realized_pnl, updated_at=excluded.updated_at
	`,
		string(p.ID), p.Cash.String(), p.RealizedPnL().String(),
		p.CreatedAt.UTC().Format(time.RFC3339),
		p.UpdatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("save portfolio: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, `DELETE FROM positions WHERE portfolio_id = ?`, string(p.ID)); err != nil {
		return fmt.Errorf("clear positions: %w", err)
	}
	for sym, pos := range p.Positions {
		if _, err := r.db.ExecContext(ctx, `
			INSERT INTO positions (portfolio_id, symbol, quantity, avg_entry_price, opened_at)
			VALUES (?, ?, ?, ?, ?)
		`, string(p.ID), string(sym), pos.Quantity, pos.AvgEntryPrice.String(),
			pos.OpenedAt.UTC().Format(time.RFC3339),
		); err != nil {
			return fmt.Errorf("save position: %w", err)
		}
	}

	if _, err := r.db.ExecContext(ctx, `DELETE FROM closed_trades WHERE portfolio_id = ?`, string(p.ID)); err != nil {
		return fmt.Errorf("clear closed trades: %w", err)
	}
	for _, t := range p.ClosedTrades() {
		if _, err := r.db.ExecContext(ctx, `
			INSERT INTO closed_trades (portfolio_id, symbol, entry_price, exit_price, quantity, pnl, opened_at, closed_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, string(p.ID), string(t.Symbol), t.EntryPrice.String(), t.ExitPrice.String(),
			t.Quantity, t.PnL.String(),
			t.OpenedAt.UTC().Format(time.RFC3339),
			t.ClosedAt.UTC().Format(time.RFC3339),
		); err != nil {
			return fmt.Errorf("save closed trade: %w", err)
		}
	}
	return nil
}

func (r *PortfolioRepository) FindByID(ctx context.Context, id portfolio.PortfolioID) (*portfolio.Portfolio, error) {
	var pid, cash, realizedPnL, createdAt, updatedAt string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, cash, realized_pnl, created_at, updated_at FROM portfolios WHERE id = ?
	`, string(id)).Scan(&pid, &cash, &realizedPnL, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, portfolio.ErrPositionNotFound
		}
		return nil, fmt.Errorf("find portfolio: %w", err)
	}

	mCash, err := core.NewMoney(cash)
	if err != nil {
		return nil, fmt.Errorf("parse cash: %w", err)
	}
	mPnl, err := core.NewMoney(realizedPnL)
	if err != nil {
		return nil, fmt.Errorf("parse realized pnl: %w", err)
	}
	ct, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	ut, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	p := portfolio.NewPortfolio(portfolio.PortfolioID(pid), mCash, ct)
	p.SetRealizedPnL(mPnl)
	p.UpdatedAt = ut

	posRows, err := r.db.QueryContext(ctx, `
		SELECT symbol, quantity, avg_entry_price, opened_at FROM positions WHERE portfolio_id = ?
	`, string(id))
	if err != nil {
		return nil, fmt.Errorf("query positions: %w", err)
	}
	defer posRows.Close()
	for posRows.Next() {
		var sym, avgPrice, openedAt string
		var qty int64
		if err := posRows.Scan(&sym, &qty, &avgPrice, &openedAt); err != nil {
			return nil, fmt.Errorf("scan position: %w", err)
		}
		mAvg, err := core.NewMoney(avgPrice)
		if err != nil {
			return nil, fmt.Errorf("parse avg price: %w", err)
		}
		ot, err := time.Parse(time.RFC3339, openedAt)
		if err != nil {
			return nil, fmt.Errorf("parse opened_at: %w", err)
		}
		pos, err := portfolio.NewPosition(core.Symbol(sym), mAvg, qty)
		if err != nil {
			return nil, fmt.Errorf("recreate position: %w", err)
		}
		pos.OpenedAt = ot
		p.Positions[core.Symbol(sym)] = pos
	}
	if err := posRows.Err(); err != nil {
		return nil, err
	}

	tradeRows, err := r.db.QueryContext(ctx, `
		SELECT symbol, entry_price, exit_price, quantity, pnl, opened_at, closed_at
		FROM closed_trades WHERE portfolio_id = ?
	`, string(id))
	if err != nil {
		return nil, fmt.Errorf("query closed trades: %w", err)
	}
	defer tradeRows.Close()
	var trades []portfolio.ClosedTrade
	for tradeRows.Next() {
		var sym, entryP, exitP, pnlStr, openedAt, closedAt string
		var qty int64
		if err := tradeRows.Scan(&sym, &entryP, &exitP, &qty, &pnlStr, &openedAt, &closedAt); err != nil {
			return nil, fmt.Errorf("scan closed trade: %w", err)
		}
		mEntry, err := core.NewMoney(entryP)
		if err != nil {
			return nil, fmt.Errorf("parse entry price: %w", err)
		}
		mExit, err := core.NewMoney(exitP)
		if err != nil {
			return nil, fmt.Errorf("parse exit price: %w", err)
		}
		mPnl, err := core.NewMoney(pnlStr)
		if err != nil {
			return nil, fmt.Errorf("parse trade pnl: %w", err)
		}
		ot, err := time.Parse(time.RFC3339, openedAt)
		if err != nil {
			return nil, fmt.Errorf("parse trade opened_at: %w", err)
		}
		ct, err := time.Parse(time.RFC3339, closedAt)
		if err != nil {
			return nil, fmt.Errorf("parse trade closed_at: %w", err)
		}
		trades = append(trades, portfolio.ClosedTrade{
			Symbol:     core.Symbol(sym),
			EntryPrice: mEntry,
			ExitPrice:  mExit,
			Quantity:   qty,
			PnL:        mPnl,
			OpenedAt:   ot,
			ClosedAt:   ct,
		})
	}
	if err := tradeRows.Err(); err != nil {
		return nil, err
	}
	p.SetClosedTrades(trades)

	return p, nil
}
