import { ApiError } from '../api/client'

function unitWord(n: number): string {
  return n === 1 ? 'unit' : 'units'
}

/**
 * Turns any error from an API call into a sentence a buyer can act on.
 * The backend's own message is never shown raw.
 */
export function describeError(error: unknown): string {
  if (!(error instanceof ApiError)) {
    return 'Something went wrong. Please try again.'
  }

  switch (error.code) {
    case 'INSUFFICIENT_STOCK': {
      const available = error.details.available_stock
      if (typeof available !== 'number') return 'Not enough stock is available.'
      if (available === 0) return 'Sold out. No units are available right now.'
      return `Only ${available} ${unitWord(available)} available. Lower the quantity and try again.`
    }
    case 'INVALID_REQUEST':
    case 'PAYLOAD_TOO_LARGE':
      return 'Some details are not valid. Check the form and try again.'
    case 'ITEM_NOT_FOUND':
      return 'This item is not available.'
    case 'RESERVATION_EXPIRED':
      return 'This reservation has expired. Its stock was released; reserve again to continue.'
    case 'RESERVATION_NOT_FOUND':
      return 'This reservation no longer exists. Reserve again to continue.'
    case 'NETWORK_ERROR':
      return 'Cannot reach the server. Check your connection and try again.'
    case 'SERVICE_UNAVAILABLE':
      return 'The server is busy right now. Wait a moment and try again.'
    case 'INTERNAL_ERROR':
    case 'NOT_FOUND':
    case 'METHOD_NOT_ALLOWED':
    case 'UNKNOWN_ERROR':
      return 'The server could not handle the request. Please try again.'
  }
}

/** Form field names, keyed by the backend's field names. */
const FIELD_NAMES: Record<string, 'userId' | 'quantity'> = {
  user_id: 'userId',
  quantity: 'quantity',
}

/**
 * Per-field messages from an INVALID_REQUEST error, for fields the form
 * shows. Anything else is covered by describeError.
 */
export function fieldErrorsOf(error: unknown): Partial<Record<'userId' | 'quantity', string>> {
  if (!(error instanceof ApiError) || error.code !== 'INVALID_REQUEST') return {}

  const result: Partial<Record<'userId' | 'quantity', string>> = {}
  for (const { field, reason } of error.fields) {
    const name = FIELD_NAMES[field]
    if (name) result[name] = `This value ${reason}.`
  }
  return result
}
