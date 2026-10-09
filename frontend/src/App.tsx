import { ReservationCard } from './components/ReservationCard'
import { ReserveForm } from './components/ReserveForm'
import { StockPanel } from './components/StockPanel'
import { ITEM_ID } from './config'
import { useTrackedReservation } from './hooks/useTrackedReservation'

function App() {
  const { tracked, start, markConfirmed, markExpired, clear } = useTrackedReservation()
  const holding = tracked?.outcome === 'pending'

  return (
    <main className="page">
      <header className="page-header">
        <h1>Inventory Reservation</h1>
        <p className="muted">Flash-sale dashboard</p>
      </header>

      <div className="layout">
        <StockPanel itemId={ITEM_ID} />
        {/* One reservation at a time: while one is waiting to be confirmed,
            the form steps aside so it cannot be lost track of. */}
        {holding ? (
          <section className="card" aria-labelledby="reserve-heading">
            <h2 id="reserve-heading">Reserve stock</h2>
            <p className="muted">
              You have a reservation waiting. Confirm it below, or let it expire, before making
              another.
            </p>
          </section>
        ) : (
          <ReserveForm itemId={ITEM_ID} onReserved={start} />
        )}
      </div>

      {tracked && (
        <ReservationCard
          // A new reservation gets a fresh card, with no leftover state.
          key={tracked.reservation.reservation_id}
          tracked={tracked}
          onConfirmed={markConfirmed}
          onExpired={markExpired}
          onStartOver={clear}
        />
      )}
    </main>
  )
}

export default App
