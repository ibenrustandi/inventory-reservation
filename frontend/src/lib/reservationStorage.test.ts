import { describe, expect, it } from 'vitest'
import type { Reservation } from '../api/types'
import { loadTrackedReservation, saveTrackedReservation } from './reservationStorage'
import type { KeyValueStore, TrackedReservation } from './reservationStorage'

function memoryStore(initial: Record<string, string> = {}): KeyValueStore & { data: Record<string, string> } {
  const data = { ...initial }
  return {
    data,
    getItem: (key) => data[key] ?? null,
    setItem: (key, value) => {
      data[key] = value
    },
    removeItem: (key) => {
      delete data[key]
    },
  }
}

const reservation: Reservation = {
  status: 'success',
  reservation_id: 'res_883291',
  item_id: 'item_4021',
  quantity: 2,
  expires_at: '2026-07-20T16:35:00Z',
}
const beforeDeadline = Date.parse('2026-07-20T16:31:00Z')
const afterDeadline = Date.parse('2026-07-20T16:36:00Z')

describe('tracked reservation storage', () => {
  it('returns what was saved', () => {
    const store = memoryStore()
    const tracked: TrackedReservation = { reservation, outcome: 'pending' }

    saveTrackedReservation(store, tracked)

    expect(loadTrackedReservation(store, beforeDeadline)).toEqual(tracked)
  })

  it('has nothing when nothing was saved', () => {
    expect(loadTrackedReservation(memoryStore(), beforeDeadline)).toBeNull()
  })

  it('forgets the reservation when saving null', () => {
    const store = memoryStore()
    saveTrackedReservation(store, { reservation, outcome: 'pending' })

    saveTrackedReservation(store, null)

    expect(loadTrackedReservation(store, beforeDeadline)).toBeNull()
    expect(Object.keys(store.data)).toEqual([])
  })

  it('marks a pending reservation expired if its deadline passed while away', () => {
    const store = memoryStore()
    saveTrackedReservation(store, { reservation, outcome: 'pending' })

    expect(loadTrackedReservation(store, afterDeadline)?.outcome).toBe('expired')
  })

  it('expires exactly at the deadline', () => {
    const store = memoryStore()
    saveTrackedReservation(store, { reservation, outcome: 'pending' })

    expect(loadTrackedReservation(store, Date.parse(reservation.expires_at))?.outcome).toBe('expired')
  })

  it('keeps a confirmed reservation confirmed after its deadline', () => {
    const store = memoryStore()
    const tracked: TrackedReservation = {
      reservation,
      outcome: 'confirmed',
      confirmedAt: '2026-07-20T16:32:00Z',
    }
    saveTrackedReservation(store, tracked)

    expect(loadTrackedReservation(store, afterDeadline)).toEqual(tracked)
  })

  it.each([
    ['broken JSON', '{"reservation":'],
    ['a different shape', '{"foo":1}'],
    ['an unknown outcome', JSON.stringify({ reservation, outcome: 'cancelled' })],
    ['a missing deadline', JSON.stringify({ reservation: { ...reservation, expires_at: undefined }, outcome: 'pending' })],
    ['a plain string', '"hello"'],
  ])('ignores %s', (_name, raw) => {
    const store = memoryStore({ 'inventory-reservation.tracked': raw })

    expect(loadTrackedReservation(store, beforeDeadline)).toBeNull()
  })

  it('does not throw when storage is unavailable', () => {
    const broken: KeyValueStore = {
      getItem: () => {
        throw new Error('blocked')
      },
      setItem: () => {
        throw new Error('quota exceeded')
      },
      removeItem: () => {
        throw new Error('blocked')
      },
    }

    expect(() => saveTrackedReservation(broken, { reservation, outcome: 'pending' })).not.toThrow()
    expect(() => saveTrackedReservation(broken, null)).not.toThrow()
    expect(loadTrackedReservation(broken, beforeDeadline)).toBeNull()
  })
})
