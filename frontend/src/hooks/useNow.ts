import { useEffect, useState } from 'react'

/**
 * The current time, refreshed every intervalMs while active. Callers
 * compute what they show from this and a fixed deadline, so a timer the
 * browser delays (a background tab, a sleeping laptop) never makes the
 * display drift: the next tick shows the true remaining time.
 */
export function useNow(intervalMs: number, active: boolean): number {
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    if (!active) return
    const tick = () => setNow(Date.now())
    const timer = setInterval(tick, intervalMs)
    // Catch up at once when the tab becomes visible again.
    document.addEventListener('visibilitychange', tick)
    return () => {
      clearInterval(timer)
      document.removeEventListener('visibilitychange', tick)
    }
  }, [intervalMs, active])

  return now
}
