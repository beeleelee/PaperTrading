import { useState, useCallback, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { cancelBacktest } from '../api/client'
import { useBacktest } from '../hooks/useBacktest'
import { useTickStream } from '../hooks/useTickStream'
import { MetricsCards } from '../components/MetricsCards'
import { KlineChart } from '../components/KlineChart'
import { OrdersTable, FillsTable, TradesTable } from '../components/Tables'
import type { WSMessage } from '../types/backtest'
import type { EquityPointDTO } from '../types/backtest'

export function ResultPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { data, loading, error, refetch } = useBacktest(id)

  const [wsTicks, setWsTicks] = useState<WSMessage[]>([])
  const [tab, setTab] = useState<'orders' | 'fills' | 'trades'>('orders')
  const [equityCurve, setEquityCurve] = useState<EquityPointDTO[]>([])

  const isRunning = data?.status === 'running' || data?.status === 'pending'
  const isDone = data?.status === 'completed'
  const isFailed = data?.status === 'failed'

  const handleTick = useCallback((msg: WSMessage) => {
    setWsTicks((prev) => [...prev, msg])
  }, [])

  const handleComplete = useCallback(() => {
    refetch()
  }, [refetch])

  useTickStream(isRunning ? id : undefined, handleTick, handleComplete)

  // When result becomes available, populate equity curve for chart
  useEffect(() => {
    if (data?.result?.equity_curve) {
      setEquityCurve(data.result.equity_curve)
    }
  }, [data])

  const handleCancel = async () => {
    if (!id) return
    try {
      await cancelBacktest(id)
      refetch()
    } catch {
      // ignore
    }
  }

  if (loading) return <div className="page"><p>Loading…</p></div>
  if (error) return <div className="page"><p className="error">{error}</p></div>
  if (!data) return <div className="page"><p>Not found</p></div>

  return (
    <div className="page">
      <div className="page-header">
        <button onClick={() => navigate('/')}>← Back</button>
        <h2>Backtest {data.id.slice(0, 8)}</h2>
        <span className={`badge badge-${data.status}`}>{data.status}</span>
        {isRunning && (
          <button onClick={handleCancel} className="danger">
            Cancel
          </button>
        )}
      </div>

      {isDone && data.result && (
        <>
          <MetricsCards metrics={data.result.metrics} />

          <KlineChart equityCurve={equityCurve} wsTicks={wsTicks} />

          <div className="tabs">
            <button
              className={tab === 'orders' ? 'active' : ''}
              onClick={() => setTab('orders')}
            >
              Orders ({data.result!.orders.length})
            </button>
            <button
              className={tab === 'fills' ? 'active' : ''}
              onClick={() => setTab('fills')}
            >
              Fills ({data.result!.fills.length})
            </button>
            <button
              className={tab === 'trades' ? 'active' : ''}
              onClick={() => setTab('trades')}
            >
              Trades ({data.result!.portfolio.closed_trades.length})
            </button>
          </div>

          <div className="tab-content">
            {tab === 'orders' && <OrdersTable orders={data.result.orders} />}
            {tab === 'fills' && <FillsTable fills={data.result.fills} />}
            {tab === 'trades' && (
              <TradesTable trades={data.result.portfolio.closed_trades} />
            )}
          </div>
        </>
      )}

      {isRunning && (
        <div className="running-msg">
          <p>Backtest is running — K-line chart will update live as ticks arrive.</p>
          {wsTicks.length > 0 && (
            <KlineChart equityCurve={equityCurve} wsTicks={wsTicks} />
          )}
        </div>
      )}

      {isFailed && <p className="error">{data.error}</p>}
    </div>
  )
}
