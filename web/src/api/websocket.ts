import type { WSMessage } from '../types/backtest'

export function connectBacktestWS(
  id: string,
  onMessage: (msg: WSMessage) => void,
  onError: (err: Event) => void,
  onClose: () => void,
): WebSocket {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = `${protocol}//${location.host}/ws/backtests/${id}`
  const ws = new WebSocket(url)

  ws.onmessage = (event) => {
    try {
      const msg: WSMessage = JSON.parse(event.data)
      onMessage(msg)
    } catch {
      // skip invalid messages
    }
  }

  ws.onerror = onError
  ws.onclose = onClose

  return ws
}
