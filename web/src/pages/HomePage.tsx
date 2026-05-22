import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { createBacktest, listBacktests } from '../api/client'
import type { CreateBacktestRequest } from '../types/backtest'

export function HomePage() {
  const navigate = useNavigate()
  const [ids, setIds] = useState<string[]>([])
  const [symbol, setSymbol] = useState('AAPL')
  const [csv, setCsv] = useState('test/data/aapl_5min.csv')
  const [cash, setCash] = useState('100000.00')
  const [loading, setLoading] = useState(false)

  const refresh = () => {
    listBacktests().then((r) => setIds(r.backtests)).catch(() => {})
  }

  useEffect(() => {
    refresh()
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    try {
      const req: CreateBacktestRequest = {
        symbols: symbol.split(',').map((s) => s.trim()),
        csv_paths: csv.split(',').map((s) => s.trim()),
        initial_cash: cash,
        strategy: {
          name: 'ma_cross',
          params: { fast: 10, slow: 30, qty: 100 },
        },
        risk: {
          max_position_pct: 0,
          max_drawdown_pct: 0,
          max_positions: 0,
        },
      }
      const job = await createBacktest(req)
      refresh()
      navigate(`/backtests/${job.id}`)
    } catch (e) {
      alert(e instanceof Error ? e.message : 'failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="home">
      <h1>PaperTrading</h1>
      <form onSubmit={handleSubmit} className="form">
        <div className="form-row">
          <label>
            Symbol
            <input
              value={symbol}
              onChange={(e) => setSymbol(e.target.value)}
            />
          </label>
          <label>
            CSV path
            <input
              value={csv}
              onChange={(e) => setCsv(e.target.value)}
            />
          </label>
          <label>
            Cash
            <input
              value={cash}
              onChange={(e) => setCash(e.target.value)}
            />
          </label>
        </div>
        <button type="submit" disabled={loading}>
          {loading ? 'Running…' : 'Run Backtest'}
        </button>
      </form>

      <h2>Recent Backtests</h2>
      {ids.length === 0 ? (
        <p className="empty">No backtests yet</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            {ids.map((id) => (
              <tr key={id}>
                <td className="mono">{id.slice(0, 8)}…</td>
                <td>
                  <button onClick={() => navigate(`/backtests/${id}`)}>
                    View
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
