import type {
  BacktestResponse,
  CreateBacktestRequest,
  ListResponse,
} from '../types/backtest'

const BASE = '/api/v1'

export async function createBacktest(
  req: CreateBacktestRequest,
): Promise<BacktestResponse> {
  const res = await fetch(`${BASE}/backtests`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const err = await res.json()
    throw new Error(err.error ?? 'create failed')
  }
  return res.json()
}

export async function listBacktests(): Promise<ListResponse> {
  const res = await fetch(`${BASE}/backtests`)
  if (!res.ok) throw new Error('list failed')
  return res.json()
}

export async function getBacktest(
  id: string,
): Promise<BacktestResponse> {
  const res = await fetch(`${BASE}/backtests/${id}`)
  if (!res.ok) throw new Error('not found')
  return res.json()
}

export async function cancelBacktest(
  id: string,
): Promise<void> {
  await fetch(`${BASE}/backtests/${id}`, { method: 'DELETE' })
}
