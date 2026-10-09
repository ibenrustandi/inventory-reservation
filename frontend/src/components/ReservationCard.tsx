import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { ApiError, confirm } from '../api/client'
import type { Confirmation } from '../api/types'
import { stockQueryKey } from '../hooks/useStock'
import { useNow } from '../hooks/useNow'
import { countdownAnnouncement, formatCountdown, remainingMs } from '../lib/countdown'
import { describeError } from '../lib/errorMessages'
import type { TrackedReservation } from '../lib/reservationStorage'

interface ReservationCardProps {
  tracked: TrackedReservation
  onConfirmed: (confirmation: Confirmation) => void
  onExpired: () => void
  onStartOver: () => void
}

function formatDateTime(value: string): string {
  return new Date(value).toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

/** Errors that mean the reservation can no longer be confirmed at all. */
function isGone(error: unknown): boolean {
  return (
    error instanceof ApiError &&
    (error.code === 'RESERVATION_EXPIRED' || error.code === 'RESERVATION_NOT_FOUND')
  )
}

export function ReservationCard({ tracked, onConfirmed, onExpired, onStartOver }: ReservationCardProps) {
  const { reservation, outcome } = tracked
  const pending = outcome === 'pending'
  const queryClient = useQueryClient()

  // The countdown is always the deadline minus the current time.
  const now = useNow(250, pending)
  const remaining = remainingMs(reservation.expires_at, now)
  const timeIsUp = pending && remaining === 0

  const mutation = useMutation({
    mutationFn: () => confirm(reservation.reservation_id),
    onSuccess: onConfirmed,
    onError: (error) => {
      // The server is the authority: if it says the reservation is gone,
      // it is, whatever this device's clock shows.
      if (isGone(error)) onExpired()
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: stockQueryKey(reservation.item_id) }),
  })

  useEffect(() => {
    if (timeIsUp) onExpired()
  }, [timeIsUp, onExpired])

  // Stock held by this reservation comes back when it expires.
  useEffect(() => {
    if (outcome === 'expired') {
      void queryClient.invalidateQueries({ queryKey: stockQueryKey(reservation.item_id) })
    }
  }, [outcome, queryClient, reservation.item_id])

  return (
    <section className="card" aria-labelledby="reservation-heading">
      <h2 id="reservation-heading">Your reservation</h2>

      <dl className="details">
        <div>
          <dt>Reservation</dt>
          <dd>
            <code>{reservation.reservation_id}</code>
          </dd>
        </div>
        <div>
          <dt>Item</dt>
          <dd>
            <code>{reservation.item_id}</code>
          </dd>
        </div>
        <div>
          <dt>Quantity</dt>
          <dd>{reservation.quantity}</dd>
        </div>
        <div>
          <dt>{outcome === 'confirmed' ? 'Confirmed at' : 'Held until'}</dt>
          <dd>
            {formatDateTime(
              outcome === 'confirmed' && tracked.confirmedAt
                ? tracked.confirmedAt
                : reservation.expires_at,
            )}
          </dd>
        </div>
      </dl>

      {pending && (
        <>
          <div className={remaining <= 60_000 ? 'countdown countdown-urgent' : 'countdown'}>
            <span className="countdown-label">Time left to confirm</span>
            <span className="countdown-value" role="timer" aria-label="Time left to confirm">
              {formatCountdown(remaining)}
            </span>
          </div>
          {/* Read out only at milestones, not every second. */}
          <p className="visually-hidden" aria-live="polite">
            {countdownAnnouncement(remaining)}
          </p>

          <button
            type="button"
            className="button button-primary"
            onClick={() => mutation.mutate()}
            disabled={mutation.isPending || timeIsUp}
          >
            {mutation.isPending ? 'Confirming…' : 'Confirm Purchase'}
          </button>

          {mutation.isError && !isGone(mutation.error) && (
            <p className="notice notice-error" role="alert">
              {describeError(mutation.error)} Your reservation is still held; you can try again.
            </p>
          )}
        </>
      )}

      {outcome === 'confirmed' && (
        <>
          <p className="notice notice-success" role="status">
            Purchase confirmed. {reservation.quantity === 1 ? 'Your unit is' : 'Your units are'} yours.
          </p>
          <button type="button" className="button button-secondary" onClick={onStartOver}>
            Make another reservation
          </button>
        </>
      )}

      {outcome === 'expired' && (
        <>
          <p className="notice notice-error" role="alert">
            This reservation expired before it was confirmed. Its stock has been released.
          </p>
          <button type="button" className="button button-secondary" onClick={onStartOver}>
            Reserve again
          </button>
        </>
      )}
    </section>
  )
}
