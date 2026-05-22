import type { OrderDTO, FillDTO, ClosedTradeDTO } from '../types/backtest'

export function OrdersTable({ orders }: { orders: OrderDTO[] }) {
  if (orders.length === 0) return <p className="empty">No orders</p>
  return (
    <table>
      <thead>
        <tr>
          <th>ID</th>
          <th>Symbol</th>
          <th>Side</th>
          <th>Type</th>
          <th>Status</th>
          <th>Price</th>
          <th>Qty</th>
          <th>Filled</th>
        </tr>
      </thead>
      <tbody>
        {orders.map((o) => (
          <tr key={o.id}>
            <td>{o.id}</td>
            <td>{o.symbol}</td>
            <td className={o.side === 'buy' ? 'buy' : 'sell'}>{o.side}</td>
            <td>{o.order_type}</td>
            <td>{o.status}</td>
            <td>{o.price}</td>
            <td>{o.quantity}</td>
            <td>{o.filled_qty}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

export function FillsTable({ fills }: { fills: FillDTO[] }) {
  if (fills.length === 0) return <p className="empty">No fills</p>
  return (
    <table>
      <thead>
        <tr>
          <th>ID</th>
          <th>Order</th>
          <th>Price</th>
          <th>Qty</th>
          <th>Commission</th>
          <th>Time</th>
        </tr>
      </thead>
      <tbody>
        {fills.map((f) => (
          <tr key={f.id}>
            <td>{f.id}</td>
            <td>{f.order_id}</td>
            <td>{f.price}</td>
            <td>{f.quantity}</td>
            <td>{f.commission}</td>
            <td>{new Date(f.timestamp).toLocaleString()}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

export function TradesTable({
  trades,
}: {
  trades: ClosedTradeDTO[]
}) {
  if (trades.length === 0) return <p className="empty">No closed trades</p>
  return (
    <table>
      <thead>
        <tr>
          <th>Symbol</th>
          <th>Entry</th>
          <th>Exit</th>
          <th>Qty</th>
          <th>PnL</th>
          <th>Opened</th>
          <th>Closed</th>
        </tr>
      </thead>
      <tbody>
        {trades.map((t, i) => (
          <tr key={i}>
            <td>{t.symbol}</td>
            <td>{t.entry_price}</td>
            <td>{t.exit_price}</td>
            <td>{t.quantity}</td>
            <td className={Number(t.pnl) >= 0 ? 'profit' : 'loss'}>
              {t.pnl}
            </td>
            <td>{new Date(t.opened_at).toLocaleString()}</td>
            <td>{new Date(t.closed_at).toLocaleString()}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
