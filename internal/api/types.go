package api

type CreateBacktestRequest struct {
	Symbols     []string       `json:"symbols"`
	CSVPaths    []string       `json:"csv_paths"`
	InitialCash string         `json:"initial_cash"`
	Strategy    StrategyConfig `json:"strategy"`
	Risk        RiskConfig     `json:"risk"`
	Noise       NoiseConfig    `json:"noise"`
}

type StrategyConfig struct {
	Name   string         `json:"name"`
	Params map[string]any `json:"params"`
}

type NoiseConfig struct {
	Enabled         bool    `json:"enabled"`
	Count           int     `json:"count"`
	MaxSpreadBP     float64 `json:"max_spread_bp"`
	MinQty          int64   `json:"min_qty"`
	MaxQty          int64   `json:"max_qty"`
	OrderRate       float64 `json:"order_rate"`
	MarketOrderRate float64 `json:"market_order_rate"`
}

type RiskConfig struct {
	MaxPositionPct float64 `json:"max_position_pct"`
	MaxDrawdownPct float64 `json:"max_drawdown_pct"`
	MaxPositions   int     `json:"max_positions"`
}

type BacktestResponse struct {
	ID          string           `json:"id"`
	Status      string           `json:"status"`
	CreatedAt   string           `json:"created_at,omitempty"`
	CompletedAt string           `json:"completed_at,omitempty"`
	Result      *SimulationDTO   `json:"result,omitempty"`
	Error       string           `json:"error,omitempty"`
}

type SimulationDTO struct {
	Portfolio   *PortfolioDTO   `json:"portfolio"`
	Orders      []OrderDTO      `json:"orders"`
	Fills       []FillDTO       `json:"fills"`
	EquityCurve []EquityPointDTO `json:"equity_curve"`
	Metrics     *MetricsDTO     `json:"metrics"`
	StartTime   string          `json:"start_time"`
	EndTime     string          `json:"end_time"`
}

type PortfolioDTO struct {
	ID           string            `json:"id"`
	Cash         string            `json:"cash"`
	Positions    map[string]*PositionDTO `json:"positions"`
	ClosedTrades []ClosedTradeDTO  `json:"closed_trades"`
	RealizedPnL  string            `json:"realized_pnl"`
	CreatedAt    string            `json:"created_at"`
	UpdatedAt    string            `json:"updated_at"`
}

type PositionDTO struct {
	Symbol        string `json:"symbol"`
	Quantity      int64  `json:"quantity"`
	AvgEntryPrice string `json:"avg_entry_price"`
	OpenedAt      string `json:"opened_at"`
}

type ClosedTradeDTO struct {
	Symbol     string `json:"symbol"`
	EntryPrice string `json:"entry_price"`
	ExitPrice  string `json:"exit_price"`
	Quantity   int64  `json:"quantity"`
	PnL        string `json:"pnl"`
	OpenedAt   string `json:"opened_at"`
	ClosedAt   string `json:"closed_at"`
}

type OrderDTO struct {
	ID          string  `json:"id"`
	PortfolioID string  `json:"portfolio_id"`
	Symbol      string  `json:"symbol"`
	Side        string  `json:"side"`
	OrderType   string  `json:"order_type"`
	Status      string  `json:"status"`
	Price       string  `json:"price"`
	StopPrice   string  `json:"stop_price"`
	Quantity    int64   `json:"quantity"`
	FilledQty   int64   `json:"filled_qty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	Fills       []FillDTO `json:"fills,omitempty"`
}

type FillDTO struct {
	ID         string `json:"id"`
	OrderID    string `json:"order_id"`
	Price      string `json:"price"`
	Quantity   int64  `json:"quantity"`
	Commission string `json:"commission"`
	Timestamp  string `json:"timestamp"`
}

type EquityPointDTO struct {
	Time   string `json:"time"`
	Equity string `json:"equity"`
}

type MetricsDTO struct {
	TotalPnL      string  `json:"total_pnl"`
	MaxDrawdown   float64 `json:"max_drawdown"`
	SharpeRatio   float64 `json:"sharpe_ratio"`
	WinRate       float64 `json:"win_rate"`
	ProfitFactor  float64 `json:"profit_factor"`
	TotalTrades   int     `json:"total_trades"`
	WinningTrades int     `json:"winning_trades"`
	LosingTrades  int     `json:"losing_trades"`
}

type DepthDTO struct {
	Price    string `json:"price"`
	Quantity int64  `json:"quantity"`
	Count    int    `json:"count"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type ListResponse struct {
	Backtests []string `json:"backtests"`
}
