import { useState, useEffect, useCallback } from 'react'
import type { BacktestResponse } from '../types/backtest'
import { getBacktest } from '../api/client'

export function useBacktest(id: string | undefined) {
  const [data, setData] = useState<BacktestResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetch = useCallback(async () => {
    if (!id) return
    setLoading(true)
    try {
      const res = await getBacktest(id)
      setData(res)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'fetch failed')
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => {
    fetch()
  }, [fetch])

  return { data, loading, error, refetch: fetch }
}
