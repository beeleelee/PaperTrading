import { useEffect, useRef, useCallback } from 'react'
import { connectBacktestWS } from '../api/websocket'
import type { WSMessage } from '../types/backtest'

export function useTickStream(
  id: string | undefined,
  onTick: (msg: WSMessage) => void,
  onComplete: () => void,
) {
  const wsRef = useRef<WebSocket | null>(null)

  const disconnect = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
  }, [])

  useEffect(() => {
    if (!id) return

    wsRef.current = connectBacktestWS(
      id,
      (msg) => {
        if (msg.type === 'completed') {
          onComplete()
          disconnect()
        } else {
          onTick(msg)
        }
      },
      () => {
        // on error, let the poll mechanism take over
      },
      () => {
        wsRef.current = null
      },
    )

    return disconnect
  }, [id, onTick, onComplete, disconnect])
}
