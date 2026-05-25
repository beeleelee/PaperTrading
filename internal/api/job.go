package api

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/felix/papertrading/internal/app"
	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/market"
	"github.com/felix/papertrading/internal/domain/order"
	"github.com/felix/papertrading/internal/domain/portfolio"
	"github.com/felix/papertrading/internal/domain/risk"
	"github.com/felix/papertrading/internal/domain/strategy"
	"github.com/felix/papertrading/internal/event"
	"github.com/felix/papertrading/internal/infra/clock"
	"github.com/felix/papertrading/internal/infra/feed"
	"github.com/google/uuid"
)

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

type WSMessage struct {
	Type      string     `json:"type"`
	Symbol    string     `json:"symbol,omitempty"`
	Price     string     `json:"price,omitempty"`
	Volume    int64      `json:"volume,omitempty"`
	Equity    string     `json:"equity,omitempty"`
	MarkPrice string     `json:"mark_price,omitempty"`
	Bids      []DepthDTO `json:"bids,omitempty"`
	Asks      []DepthDTO `json:"asks,omitempty"`
	Timestamp string     `json:"timestamp,omitempty"`
	Error     string     `json:"error,omitempty"`
}

type BacktestJob struct {
	ID          string
	Config      CreateBacktestRequest
	Status      JobStatus
	Result      *app.SimulationResult
	Error       string
	CreatedAt   time.Time
	CompletedAt time.Time
	TickChan    chan WSMessage
	cancel      context.CancelFunc
}

type JobManager struct {
	mu   sync.RWMutex
	jobs map[string]*BacktestJob
}

func NewJobManager() *JobManager {
	return &JobManager{
		jobs: make(map[string]*BacktestJob),
	}
}

func (jm *JobManager) Create(req CreateBacktestRequest) *BacktestJob {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	job := &BacktestJob{
		ID:        uuid.NewString(),
		Config:    req,
		Status:    JobStatusPending,
		CreatedAt: time.Now(),
		TickChan:  make(chan WSMessage, 256),
	}
	jm.jobs[job.ID] = job
	return job
}

func (jm *JobManager) Get(id string) (*BacktestJob, bool) {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	job, ok := jm.jobs[id]
	return job, ok
}

func (jm *JobManager) List() []*BacktestJob {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	result := make([]*BacktestJob, 0, len(jm.jobs))
	for _, job := range jm.jobs {
		result = append(result, job)
	}
	return result
}

func (jm *JobManager) Run(job *BacktestJob) {
	jm.mu.Lock()
	job.Status = JobStatusRunning
	jm.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	jm.mu.Lock()
	job.cancel = cancel
	jm.mu.Unlock()

	go func() {
		defer cancel()
		defer close(job.TickChan)
		defer func() {
			if r := recover(); r != nil {
				log.Printf("backtest %s panicked: %v", job.ID, r)
			}
		}()

		result, err := jm.execute(ctx, job, job.TickChan)
		jm.mu.Lock()
		defer jm.mu.Unlock()

		job.CompletedAt = time.Now()
		if err != nil {
			job.Status = JobStatusFailed
			job.Error = err.Error()
			job.TickChan <- WSMessage{Type: "error", Error: err.Error()}
		} else {
			job.Status = JobStatusCompleted
			job.Result = result
			job.TickChan <- WSMessage{Type: "completed"}
		}
	}()
}

func (jm *JobManager) Cancel(id string) bool {
	jm.mu.Lock()
	job, ok := jm.jobs[id]
	jm.mu.Unlock()
	if !ok {
		return false
	}
	jm.mu.RLock()
	cancel := job.cancel
	jm.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
	return true
}

func (jm *JobManager) execute(ctx context.Context, job *BacktestJob, tickChan chan<- WSMessage) (*app.SimulationResult, error) {
	initialCash, err := core.NewMoney(job.Config.InitialCash)
	if err != nil {
		return nil, fmt.Errorf("invalid cash: %w", err)
	}

	if len(job.Config.Symbols) != len(job.Config.CSVPaths) {
		return nil, fmt.Errorf("symbols and csv_paths must have same length")
	}

	symbols := make([]core.Symbol, len(job.Config.Symbols))
	for i, sym := range job.Config.Symbols {
		symbols[i] = core.Symbol(sym)
	}

	var feeds []market.Feed
	var strats []strategy.Strategy
	for i, sym := range symbols {
		f := feed.NewCSVFeed(job.Config.CSVPaths[i], feed.WithSkipRows(1), feed.WithSymbol(sym))
		feeds = append(feeds, f)

		s, err := createStrategy(job.Config.Strategy.Name, job.Config.Strategy.Params, sym)
		if err != nil {
			return nil, fmt.Errorf("create strategy for %s: %w", sym, err)
		}
		strats = append(strats, s)
	}

	mergedFeed := feed.NewMergedFeed(feeds...)

	var strat strategy.Strategy
	if len(strats) == 1 {
		strat = strats[0]
	} else {
		strat = strategy.NewMultiStrategy(strats...)
	}

	nc := job.Config.Noise
	simConfig := app.SimulationConfig{
		InitialCash: initialCash,
		PortfolioID: portfolio.PortfolioID("api-" + uuid.NewString()[:8]),
		FillConfig:  order.DefaultFillConfig(),
		RiskConstraints: risk.Constraints{
			MaxPositionPct: job.Config.Risk.MaxPositionPct,
			MaxDrawdownPct: job.Config.Risk.MaxDrawdownPct,
			MaxPositions:   job.Config.Risk.MaxPositions,
		},
		NoiseConfig: market.NoiseConfig{
			Enabled:         nc.Enabled,
			Count:           nc.Count,
			MaxSpreadBP:     nc.MaxSpreadBP,
			MinQty:          nc.MinQty,
			MaxQty:          nc.MaxQty,
			OrderRate:       nc.OrderRate,
			MarketOrderRate: nc.MarketOrderRate,
		},
		LogTrades: false,
	}

	bus := event.NewBus()
	sim := app.NewSimulation(mergedFeed, strat, clock.RealClock{}, bus, simConfig)

	bus.Subscribe("market.tick_processed", func(ctx context.Context, e event.Event) error {
		tp := e.(event.TickProcessed)
		depth := sim.Depth(tp.Symbol)
		bids := make([]DepthDTO, len(depth.Bids))
		for i, b := range depth.Bids {
			bids[i] = DepthDTO{Price: b.Price.String(), Quantity: b.Quantity, Count: b.Count}
		}
		asks := make([]DepthDTO, len(depth.Asks))
		for i, a := range depth.Asks {
			asks[i] = DepthDTO{Price: a.Price.String(), Quantity: a.Quantity, Count: a.Count}
		}
		select {
		case tickChan <- WSMessage{
			Type:      "tick",
			Symbol:    string(tp.Symbol),
			Price:     tp.Price.String(),
			Volume:    tp.Volume,
			Equity:    tp.Equity.String(),
			MarkPrice: tp.Price.String(),
			Bids:      bids,
			Asks:      asks,
			Timestamp: tp.At.Format(time.RFC3339),
		}:
		default:
		}
		return nil
	})

	return sim.Run(ctx)
}
