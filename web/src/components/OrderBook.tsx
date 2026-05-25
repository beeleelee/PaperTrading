import type { DepthLevel } from '../types/backtest'

interface Props {
  markPrice: string
  bids: DepthLevel[]
  asks: DepthLevel[]
}

export function OrderBook({ markPrice, bids, asks }: Props) {
  const topBids = bids.slice(0, 8)
  const topAsks = asks.slice(0, 8)

  return (
    <div className="orderbook">
      <div className="orderbook-main">
        <table className="orderbook-table">
          <thead>
            <tr>
              <th>Price</th>
              <th>Qty</th>
            </tr>
          </thead>
          <tbody>
            {topBids.map((b, i) => (
              <tr key={i} className="bid">
                <td className="price">{b.price}</td>
                <td className="qty">{b.quantity}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="orderbook-mark">
        {markPrice}
      </div>
      <div className="orderbook-main">
        <table className="orderbook-table">
          <thead>
            <tr>
              <th>Price</th>
              <th>Qty</th>
            </tr>
          </thead>
          <tbody>
            {topAsks.map((a, i) => (
              <tr key={i} className="ask">
                <td className="price">{a.price}</td>
                <td className="qty">{a.quantity}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
