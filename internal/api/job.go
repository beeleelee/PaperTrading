package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/felix/papertrading/internal/domain/core"
	"github.com/felix/papertrading/internal/domain/market"
	"github.com/felix/papertrading/internal/domain/order"
	"github.com/felix/papertrading/internal/domain/portfolio"
	"github.com/felix/papertrading/internal/domain/risk"
	"github.com/felix/papertrading/internal/domain/strategy"
	"github.com/felix/papertrading/internal/event"
	"github.com/felix/papertrading/internal/infra/feed"
	"github.com/felix/papertrading/internal/infra/clock"
	"github.com/felix/papertrading/internal/app"
	"github.com/google/uuid"
)

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

type BacktestJob struct {
	ID          string
	Config      CreateBacktestRequest
	Status      JobStatus
	Result      *app.SimulationResult
	Error       string
	CreatedAt   time.Time
	CompletedAt time.Time
	sim         *app.Simulation
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

		result, err := jm.execute(ctx, job.Config)
		jm.mu.Lock()
		defer jm.mu.Unlock()

		job.CompletedAt = time.Now()
		if err != nil {
			job.Status = JobStatusFailed
			job.Error = err.Error()
		} else {
			job.Status = JobStatusCompleted
			job.Result = result
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

func (jm *JobManager) execute(ctx context.Context, cfg CreateBacktestRequest) (*app.SimulationResult, error) {
	initialCash, err := core.NewMoney(cfg.InitialCash)
	if err != nil {
		return nil, fmt.Errorf("invalid cash: %w", err)
	}

	if len(cfg.Symbols) != len(cfg.CSVPaths) {
		return nil, fmt.Errorf("symbols and csv_paths must have same length")
	}

	symbols := make([]core.Symbol, len(cfg.Symbols))
	for i, sym := range cfg.Symbols {
		symbols[i] = core.Symbol(sym)
	}

	var feeds []market.Feed
	var strats []strategy.Strategy
	for i, sym := range symbols {
		f := feed.NewCSVFeed(cfg.CSVPaths[i], feed.WithSkipRows(1), feed.WithSymbol(sym))
		feeds = append(feeds, f)

		s, err := createStrategy(cfg.Strategy.Name, cfg.Strategy.Params, sym)
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

	simConfig := app.SimulationConfig{
		InitialCash: initialCash,
		PortfolioID: portfolio.PortfolioID("api-" + uuid.NewString()[:8]),
		FillConfig:  order.DefaultFillConfig(),
		RiskConstraints: risk.Constraints{
			MaxPositionPct: cfg.Risk.MaxPositionPct,
			MaxDrawdownPct: cfg.Risk.MaxDrawdownPct,
			MaxPositions:   cfg.Risk.MaxPositions,
		},
		LogTrades: false,
	}

	bus := event.NewBus()
	sim := app.NewSimulation(mergedFeed, strat, clock.RealClock{}, bus, simConfig)
	return sim.Run(ctx)
}
