import { useStock } from '../hooks/useStock'
import { describeError } from '../lib/errorMessages'

interface StockPanelProps {
  itemId: string
}

function formatClock(timestamp: number): string {
  return new Date(timestamp).toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

export function StockPanel({ itemId }: StockPanelProps) {
  const { data: stock, error, isPending, isFetching, dataUpdatedAt, refetch } = useStock(itemId)
  const soldOut = stock !== undefined && stock.available_stock === 0

  return (
    <section className="card" aria-labelledby="stock-heading">
      <div className="card-header">
        <div>
          <h2 id="stock-heading">Live inventory</h2>
          <p className="muted">
            Item <code>{itemId}</code>
          </p>
        </div>
        <button
          type="button"
          className="button button-secondary"
          onClick={() => void refetch()}
          disabled={isFetching}
        >
          {isFetching ? 'Refreshing…' : 'Refresh'}
        </button>
      </div>

      {isPending && <p className="muted">Loading stock…</p>}

      {stock && (
        <>
          <dl className="stats">
            <div className="stat">
              <dt>Total</dt>
              <dd>{stock.total_stock}</dd>
            </div>
            <div className="stat">
              <dt>Reserved</dt>
              <dd>{stock.reserved_stock}</dd>
            </div>
            <div className={soldOut ? 'stat stat-empty' : 'stat stat-highlight'}>
              <dt>Available</dt>
              <dd>{stock.available_stock}</dd>
            </div>
          </dl>
          {soldOut && (
            <p className="notice notice-warning">
              Sold out for now. Units come back if a reservation expires.
            </p>
          )}
        </>
      )}

      {/* role="alert" announces a failure once; the numbers above stay on
          screen, marked as possibly out of date. */}
      {error && (
        <p className="notice notice-error" role="alert">
          {describeError(error)}
          {stock && ' Showing the last known numbers.'}
        </p>
      )}

      <p className="muted small">
        {dataUpdatedAt > 0 ? `Last updated ${formatClock(dataUpdatedAt)}. ` : ''}
        Refreshes every 2 seconds.
      </p>
    </section>
  )
}
