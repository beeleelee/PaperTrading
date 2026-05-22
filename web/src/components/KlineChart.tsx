import { useEffect, useRef, useCallback } from 'react'
import {
  createChart,
  type IChartApi,
  type ISeriesApi,
  type CandlestickData,
  type LineData,
  type Time,
} from 'lightweight-charts'
import type { EquityPointDTO, WSMessage } from '../types/backtest'

interface Props {
  equityCurve: EquityPointDTO[]
  wsTicks: WSMessage[]
}

function parsePrice(s: string): number {
  return parseFloat(s) || 0
}

const BAR_SECONDS = 300 // 5-minute bars

function buildCandles(
  ticks: WSMessage[],
  startTs: number,
): CandlestickData[] {
  if (ticks.length === 0) return []

  const bars = new Map<number, CandlestickData>()

  const seed = (t: WSMessage) => {
    const ts = new Date(t.timestamp ?? startTs).getTime() / 1000
    const bucket = Math.floor(ts / BAR_SECONDS) * BAR_SECONDS
    const p = parsePrice(t.price ?? '0')
    const vol = t.volume ?? 0
    const existing = bars.get(bucket)
    if (!existing) {
      bars.set(bucket, {
        time: bucket as Time,
        open: p,
        high: p,
        low: p,
        close: p,
        customValues: { volume: vol },
      })
    } else {
      existing.high = Math.max(existing.high, p)
      existing.low = Math.min(existing.low, p)
      existing.close = p
      if (existing.customValues) {
        (existing.customValues as Record<string, number>).volume =
          ((existing.customValues as Record<string, number>).volume ?? 0) + vol
      }
    }
  }

  for (const t of ticks) seed(t)
  // ensure first candle from startTs
  seed({
    type: 'tick' as const,
    price: ticks[0].price,
    volume: 0,
    timestamp: new Date(startTs * 1000).toISOString(),
  })

  const result = Array.from(bars.values())
  result.sort((a, b) => Number(a.time) - Number(b.time))
  return result
}

export function KlineChart({ equityCurve, wsTicks }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)
  const candleSeriesRef = useRef<ISeriesApi<'Candlestick'> | null>(null)
  const equitySeriesRef = useRef<ISeriesApi<'Line'> | null>(null)

  const initChart = useCallback(() => {
    if (!containerRef.current || chartRef.current) return

    const chart = createChart(containerRef.current, {
      width: containerRef.current.clientWidth,
      height: 500,
      layout: {
        background: { color: '#1a1a2e' },
        textColor: '#d1d4dc',
      },
      grid: {
        vertLines: { color: '#2a2a3e' },
        horzLines: { color: '#2a2a3e' },
      },
      crosshair: {
        mode: 0,
      },
      timeScale: {
        borderColor: '#2a2a3e',
        timeVisible: true,
        secondsVisible: false,
      },
      rightPriceScale: {
        borderColor: '#2a2a3e',
      },
    })

    const candles = chart.addCandlestickSeries({
      upColor: '#26a69a',
      downColor: '#ef5350',
      borderDownColor: '#ef5350',
      borderUpColor: '#26a69a',
      wickDownColor: '#ef5350',
      wickUpColor: '#26a69a',
    })

    const equity = chart.addLineSeries({
      color: '#42a5f5',
      lineWidth: 2,
      priceFormat: { type: 'price' },
      priceLineVisible: false,
    })

    candleSeriesRef.current = candles
    equitySeriesRef.current = equity
    chartRef.current = chart

    const handleResize = () => {
      if (containerRef.current && chartRef.current) {
        chartRef.current.applyOptions({
          width: containerRef.current.clientWidth,
        })
      }
    }

    window.addEventListener('resize', handleResize)

    return () => {
      window.removeEventListener('resize', handleResize)
      chart.remove()
      chartRef.current = null
      candleSeriesRef.current = null
      equitySeriesRef.current = null
    }
  }, [])

  // Initialize chart once
  useEffect(() => {
    const cleanup = initChart()
    return () => cleanup?.()
  }, [initChart])

  // Update candles from wsTicks
  useEffect(() => {
    if (!candleSeriesRef.current || wsTicks.length === 0) return

    const startTs = wsTicks[0]?.timestamp
      ? new Date(wsTicks[0].timestamp).getTime() / 1000
      : Date.now() / 1000

    const candles = buildCandles(wsTicks, startTs)
    if (candles.length > 0) {
      candleSeriesRef.current.setData(candles)
    }
  }, [wsTicks])

  // Update equity line from stored equityCurve
  useEffect(() => {
    if (!equitySeriesRef.current || equityCurve.length === 0) return

    const lineData: LineData[] = equityCurve.map((ep) => ({
      time: (new Date(ep.time).getTime() / 1000) as Time,
      value: parsePrice(ep.equity),
    }))

    equitySeriesRef.current.setData(lineData)
  }, [equityCurve])

  return <div ref={containerRef} className="kline-chart" />
}
