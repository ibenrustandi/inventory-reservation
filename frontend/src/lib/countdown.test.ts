import { describe, expect, it } from 'vitest'
import { countdownAnnouncement, formatCountdown, remainingMs } from './countdown'

describe('remainingMs', () => {
  const now = Date.parse('2026-07-20T16:30:00Z')

  it('is the time until the deadline', () => {
    expect(remainingMs('2026-07-20T16:35:00Z', now)).toBe(5 * 60 * 1000)
  })

  it('is zero at the deadline and after it', () => {
    expect(remainingMs('2026-07-20T16:30:00Z', now)).toBe(0)
    expect(remainingMs('2026-07-20T16:00:00Z', now)).toBe(0)
  })

  it('treats an unreadable deadline as passed', () => {
    expect(remainingMs('not a date', now)).toBe(0)
  })
})

describe('formatCountdown', () => {
  it.each([
    [5 * 60 * 1000, '05:00'],
    [4 * 60 * 1000 + 59_000, '04:59'],
    [61_000, '01:01'],
    [60_000, '01:00'],
    [59_001, '01:00'],
    [59_000, '00:59'],
    [1, '00:01'],
    [0, '00:00'],
    [-500, '00:00'],
  ])('formats %i ms as %s', (ms, expected) => {
    expect(formatCountdown(ms)).toBe(expected)
  })

  it('still works beyond an hour', () => {
    expect(formatCountdown(61 * 60 * 1000)).toBe('61:00')
  })
})

describe('countdownAnnouncement', () => {
  it('is silent with plenty of time left', () => {
    expect(countdownAnnouncement(5 * 60 * 1000)).toBe('')
    expect(countdownAnnouncement(61_000)).toBe('')
  })

  it('announces the last minute, the last ten seconds and expiry', () => {
    expect(countdownAnnouncement(60_000)).toBe('1 minute left to confirm.')
    expect(countdownAnnouncement(10_000)).toBe('10 seconds left to confirm.')
    expect(countdownAnnouncement(0)).toBe('Your reservation has expired.')
  })

  it('stays the same between milestones, so it is announced once', () => {
    expect(countdownAnnouncement(45_000)).toBe(countdownAnnouncement(11_000))
    expect(countdownAnnouncement(9_000)).toBe(countdownAnnouncement(1_000))
  })
})
