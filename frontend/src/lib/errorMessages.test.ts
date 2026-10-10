import { describe, expect, it } from 'vitest'
import { ApiError } from '../api/client'
import type { ErrorCode } from '../api/types'
import { describeError, fieldErrorsOf } from './errorMessages'

const ALL_CODES: ErrorCode[] = [
  'INVALID_REQUEST',
  'PAYLOAD_TOO_LARGE',
  'ITEM_NOT_FOUND',
  'INSUFFICIENT_STOCK',
  'RESERVATION_NOT_FOUND',
  'RESERVATION_EXPIRED',
  'NOT_FOUND',
  'METHOD_NOT_ALLOWED',
  'INTERNAL_ERROR',
  'SERVICE_UNAVAILABLE',
  'NETWORK_ERROR',
  'UNKNOWN_ERROR',
]

describe('describeError', () => {
  it.each(ALL_CODES)('has a message for %s that hides the backend text', (code) => {
    const message = describeError(new ApiError(code, 'raw backend text', 500))

    expect(message.length).toBeGreaterThan(0)
    expect(message).not.toContain('raw backend text')
  })

  it('says how many units are left', () => {
    const error = new ApiError('INSUFFICIENT_STOCK', '', 409, { available_stock: 3, requested: 5 })
    expect(describeError(error)).toContain('Only 3 units available')
  })

  it('uses the singular for one unit', () => {
    const error = new ApiError('INSUFFICIENT_STOCK', '', 409, { available_stock: 1, requested: 2 })
    expect(describeError(error)).toContain('Only 1 unit available')
  })

  it('says sold out when nothing is left', () => {
    const error = new ApiError('INSUFFICIENT_STOCK', '', 409, { available_stock: 0, requested: 1 })
    expect(describeError(error)).toContain('Sold out')
  })

  it('copes with an INSUFFICIENT_STOCK that has no count', () => {
    expect(describeError(new ApiError('INSUFFICIENT_STOCK', '', 409))).toBe(
      'Not enough stock is available.',
    )
  })

  it('has a message for an error that is not an ApiError', () => {
    expect(describeError(new TypeError('boom'))).toBe('Something went wrong. Please try again.')
  })
})

describe('fieldErrorsOf', () => {
  it('maps backend field names to form fields', () => {
    const error = new ApiError('INVALID_REQUEST', '', 400, {
      fields: [
        { field: 'user_id', reason: 'is required' },
        { field: 'quantity', reason: 'must be an integer from 1 to 1000' },
      ],
    })

    expect(fieldErrorsOf(error)).toEqual({
      userId: 'This value is required.',
      quantity: 'This value must be an integer from 1 to 1000.',
    })
  })

  it('ignores fields the form does not show', () => {
    const error = new ApiError('INVALID_REQUEST', '', 400, {
      fields: [{ field: 'item_id', reason: 'is required' }],
    })

    expect(fieldErrorsOf(error)).toEqual({})
  })

  it('is empty for other errors', () => {
    expect(fieldErrorsOf(new ApiError('INSUFFICIENT_STOCK', '', 409))).toEqual({})
    expect(fieldErrorsOf(new Error('boom'))).toEqual({})
  })
})
