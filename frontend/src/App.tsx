import { useState } from 'react'
import type { Reservation } from './api/types'
import { ReserveForm } from './components/ReserveForm'
import { StockPanel } from './components/StockPanel'
import { ITEM_ID } from './config'

function App() {
  const [reservation, setReservation] = useState<Reservation | null>(null)

  return (
    <main className="page">
      <header className="page-header">
        <h1>Inventory Reservation</h1>
        <p className="muted">Flash-sale dashboard</p>
      </header>

      <div className="layout">
        <StockPanel itemId={ITEM_ID} />
        <ReserveForm itemId={ITEM_ID} onReserved={setReservation} />
      </div>

      {reservation && (
        <section className="card" aria-labelledby="reservation-heading">
          <h2 id="reservation-heading">Reservation created</h2>
          <p className="notice notice-success" role="status">
            Reserved {reservation.quantity} of <code>{reservation.item_id}</code> as{' '}
            <code>{reservation.reservation_id}</code>. It expires at{' '}
            {new Date(reservation.expires_at).toLocaleTimeString()}.
          </p>
        </section>
      )}
    </main>
  )
}

export default App
