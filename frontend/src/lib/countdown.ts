/**
 * Milliseconds left until an RFC 3339 deadline, never below zero. An
 * unreadable deadline counts as already passed.
 */
export function remainingMs(expiresAt: string, now: number): number {
  const deadline = Date.parse(expiresAt)
  if (Number.isNaN(deadline)) return 0
  return Math.max(0, deadline - now)
}

/**
 * Formats a remaining time as mm:ss. Partial seconds round up, so the
 * display reads 00:00 only once the time is really gone.
 */
export function formatCountdown(ms: number): string {
  const totalSeconds = Math.ceil(Math.max(0, ms) / 1000)
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
}

/**
 * A sentence for screen readers at the moments worth announcing, or an
 * empty string. Announcing every second would drown out everything else.
 */
export function countdownAnnouncement(ms: number): string {
  const seconds = Math.ceil(Math.max(0, ms) / 1000)
  if (seconds === 0) return 'Your reservation has expired.'
  if (seconds <= 10) return '10 seconds left to confirm.'
  if (seconds <= 60) return '1 minute left to confirm.'
  return ''
}
