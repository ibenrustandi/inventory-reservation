import type { Reservation } from '../api/types'

/**
 * The reservation the dashboard is tracking and how it ended, if it has.
 * A reservation that is neither confirmed nor expired is still pending.
 */
export interface TrackedReservation {
  reservation: Reservation
  outcome: 'pending' | 'confirmed' | 'expired'
  /** RFC 3339, set once confirmed. */
  confirmedAt?: string
}

const STORAGE_KEY = 'inventory-reservation.tracked'

/** The part of the Storage API this module uses. */
export type KeyValueStore = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

function isTrackedReservation(value: unknown): value is TrackedReservation {
  if (typeof value !== 'object' || value === null) return false
  const { reservation, outcome } = value as Partial<TrackedReservation>
  return (
    typeof reservation === 'object' &&
    reservation !== null &&
    typeof reservation.reservation_id === 'string' &&
    typeof reservation.item_id === 'string' &&
    typeof reservation.quantity === 'number' &&
    typeof reservation.expires_at === 'string' &&
    (outcome === 'pending' || outcome === 'confirmed' || outcome === 'expired')
  )
}

/**
 * Reads the tracked reservation saved by an earlier page load. Anything
 * unreadable is ignored. A pending reservation whose deadline passed while
 * the page was closed comes back as expired.
 */
export function loadTrackedReservation(store: KeyValueStore, now: number): TrackedReservation | null {
  let parsed: unknown
  try {
    parsed = JSON.parse(store.getItem(STORAGE_KEY) ?? 'null')
  } catch {
    return null
  }
  if (!isTrackedReservation(parsed)) return null

  const deadline = Date.parse(parsed.reservation.expires_at)
  if (parsed.outcome === 'pending' && !(deadline > now)) {
    return { ...parsed, outcome: 'expired' }
  }
  return parsed
}

/** Saves the tracked reservation, or forgets it when given null. */
export function saveTrackedReservation(store: KeyValueStore, tracked: TrackedReservation | null): void {
  try {
    if (tracked === null) store.removeItem(STORAGE_KEY)
    else store.setItem(STORAGE_KEY, JSON.stringify(tracked))
  } catch {
    // Storage can be full or blocked. The dashboard works without it; a
    // reload just will not remember the reservation.
  }
}
