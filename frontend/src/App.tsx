import { ITEM_ID } from './config'

function App() {
  return (
    <main className="page">
      <header className="page-header">
        <h1>Inventory Reservation</h1>
        <p className="muted">
          Flash-sale dashboard for item <code>{ITEM_ID}</code>
        </p>
      </header>
    </main>
  )
}

export default App
