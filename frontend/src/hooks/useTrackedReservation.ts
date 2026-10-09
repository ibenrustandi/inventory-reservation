import { useCallback, useEffect, useState } from 'react'
import type { Confirmation, Reservation } from '../api/types'
import { loadTrackedReservation, saveTrackedReservation } from '../lib/reservationStorage'
import type { TrackedReservation } from '../lib/reservationStorage'

/**
 * The one reservation the dashboard follows from "reserved" to "confirmed"
 * or "expired". It is kept in localStorage, so reloading the page does not
 * lose a reservation that is still holding stock.
 */
export function useTrackedReservation() {
  const [tracked, setTracked] = useState<TrackedReservation | null>(() =>
    loadTrackedReservation(window.localStorage, Date.now()),
  )

  useEffect(() => {
    saveTrackedReservation(window.localStorage, tracked)
  }, [tracked])

  const start = useCallback((reservation: Reservation) => {
    setTracked({ reservation, outcome: 'pending' })
  }, [])

  const markConfirmed = useCallback((confirmation: Confirmation) => {
    setTracked((current) =>
      current && current.reservation.reservation_id === confirmation.reservation_id
        ? { ...current, outcome: 'confirmed', confirmedAt: confirmation.confirmed_at }
        : current,
    )
  }, [])

  const markExpired = useCallback(() => {
    // A confirmed reservation never becomes expired.
    setTracked((current) =>
      current && current.outcome === 'pending' ? { ...current, outcome: 'expired' } : current,
    )
  }, [])

  const clear = useCallback(() => setTracked(null), [])

  return { tracked, start, markConfirmed, markExpired, clear }
}
