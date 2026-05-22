import type { MetricsDTO } from '../types/backtest'

interface Props {
  metrics: MetricsDTO
}

const fmt = (v: number, d = 2) => v.toFixed(d)

export function MetricsCards({ metrics }: Props) {
  const cards = [
    { label: 'Total PnL', value: metrics.total_pnl },
    { label: 'Win Rate', value: `${fmt(metrics.win_rate)}%` },
    { label: 'Max Drawdown', value: `${fmt(metrics.max_drawdown)}%` },
    { label: 'Sharpe', value: fmt(metrics.sharpe_ratio) },
    {
      label: 'Profit Factor',
      value:
        metrics.profit_factor > 1e9
          ? '∞'
          : fmt(metrics.profit_factor),
    },
    { label: 'Trades', value: String(metrics.total_trades) },
  ]

  return (
    <div className="metrics-cards">
      {cards.map((c) => (
        <div key={c.label} className="card">
          <div className="card-label">{c.label}</div>
          <div className="card-value">{c.value}</div>
        </div>
      ))}
    </div>
  )
}
