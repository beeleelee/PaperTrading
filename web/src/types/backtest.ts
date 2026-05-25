export interface NoiseConfig {
  enabled: boolean
  count: number
  max_spread_bp: number
  min_qty: number
  max_qty: number
  order_rate: number
  market_order_rate: number
}

export interface CreateBacktestRequest {
  symbols: string[]
  csv_paths: string[]
  initial_cash: string
  strategy: StrategyConfig
  risk: RiskConfig
  noise?: NoiseConfig
}

export interface StrategyConfig {
  name: string
  params: Record<string, unknown>
}

export interface RiskConfig {
  max_position_pct: number
  max_drawdown_pct: number
  max_positions: number
}

export interface BacktestResponse {
  id: string
  status: string
  created_at: string
  completed_at?: string
  result?: SimulationDTO
  error?: string
}

export interface ListResponse {
  backtests: string[]
}

export interface SimulationDTO {
  portfolio: PortfolioDTO
  orders: OrderDTO[]
  fills: FillDTO[]
  equity_curve: EquityPointDTO[]
  metrics: MetricsDTO
  start_time: string
  end_time: string
}

export interface PortfolioDTO {
  id: string
  cash: string
  positions: Record<string, PositionDTO>
  closed_trades: ClosedTradeDTO[]
  realized_pnl: string
  created_at: string
  updated_at: string
}

export interface PositionDTO {
  symbol: string
  quantity: number
  avg_entry_price: string
  opened_at: string
}

export interface ClosedTradeDTO {
  symbol: string
  entry_price: string
  exit_price: string
  quantity: number
  pnl: string
  opened_at: string
  closed_at: string
}

export interface OrderDTO {
  id: string
  portfolio_id: string
  symbol: string
  side: string
  order_type: string
  status: string
  price: string
  stop_price: string
  quantity: number
  filled_qty: number
  created_at: string
  updated_at: string
  fills: FillDTO[]
}

export interface FillDTO {
  id: string
  order_id: string
  price: string
  quantity: number
  commission: string
  timestamp: string
}

export interface EquityPointDTO {
  time: string
  equity: string
}

export interface MetricsDTO {
  total_pnl: string
  max_drawdown: number
  sharpe_ratio: number
  win_rate: number
  profit_factor: number
  total_trades: number
  winning_trades: number
  losing_trades: number
}

export interface DepthLevel {
  price: string
  quantity: number
  count: number
}

export interface WSMessage {
  type: 'tick' | 'completed' | 'error'
  symbol?: string
  price?: string
  volume?: number
  equity?: string
  mark_price?: string
  bids?: DepthLevel[]
  asks?: DepthLevel[]
  timestamp?: string
  error?: string
}

export interface TickData {
  time: number
  open: number
  high: number
  low: number
  close: number
  volume?: number
}
