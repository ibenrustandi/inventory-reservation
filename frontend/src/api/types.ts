// Shapes of the backend's JSON, as defined by its API contract.

export interface Stock {
  item_id: string
  total_stock: number
  reserved_stock: number
  available_stock: number
}

export interface ReserveRequest {
  user_id: string
  item_id: string
  quantity: number
}

export interface Reservation {
  status: 'success'
  reservation_id: string
  item_id: string
  quantity: number
  /** RFC 3339, UTC. */
  expires_at: string
}

export interface Confirmation {
  status: 'success'
  reservation_id: string
  /** RFC 3339, UTC. */
  confirmed_at: string
}

/** Error codes the backend can return. */
export type BackendErrorCode =
  | 'INVALID_REQUEST'
  | 'ITEM_NOT_FOUND'
  | 'INSUFFICIENT_STOCK'
  | 'RESERVATION_NOT_FOUND'
  | 'RESERVATION_EXPIRED'
  | 'NOT_FOUND'
  | 'METHOD_NOT_ALLOWED'
  | 'INTERNAL_ERROR'

/**
 * Every code an ApiError can carry: the backend's, plus two the client sets
 * itself when there is no usable backend answer.
 */
export type ErrorCode = BackendErrorCode | 'NETWORK_ERROR' | 'UNKNOWN_ERROR'

export interface FieldError {
  field: string
  reason: string
}

/** The backend's single error envelope. */
export interface ErrorEnvelope {
  status: 'error'
  error: {
    code: BackendErrorCode
    message: string
    details?: Record<string, unknown>
  }
  request_id: string
}
