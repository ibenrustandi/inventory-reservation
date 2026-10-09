import { API_BASE } from '../config'
import type {
  Confirmation,
  ErrorCode,
  ErrorEnvelope,
  FieldError,
  Reservation,
  ReserveRequest,
  Stock,
} from './types'

/**
 * The one error type the API functions throw. `code` is what the UI
 * switches on; `status` is 0 when no HTTP response was received.
 */
export class ApiError extends Error {
  readonly code: ErrorCode
  readonly status: number
  readonly details: Record<string, unknown>
  readonly requestId: string | undefined

  constructor(
    code: ErrorCode,
    message: string,
    status: number,
    details: Record<string, unknown> = {},
    requestId?: string,
  ) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
    this.details = details
    this.requestId = requestId
  }

  /** The invalid fields listed by an INVALID_REQUEST error. */
  get fields(): FieldError[] {
    const fields = this.details.fields
    return Array.isArray(fields) ? (fields as FieldError[]) : []
  }
}

function isErrorEnvelope(body: unknown): body is ErrorEnvelope {
  if (typeof body !== 'object' || body === null) return false
  const error = (body as { error?: unknown }).error
  return (
    typeof error === 'object' &&
    error !== null &&
    typeof (error as { code?: unknown }).code === 'string'
  )
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let response: Response
  try {
    response = await fetch(API_BASE + path, init)
  } catch {
    throw new ApiError('NETWORK_ERROR', 'Could not reach the server.', 0)
  }

  let body: unknown
  try {
    body = await response.json()
  } catch {
    body = undefined
  }

  if (response.ok) return body as T

  if (isErrorEnvelope(body)) {
    throw new ApiError(
      body.error.code,
      body.error.message,
      response.status,
      body.error.details,
      body.request_id,
    )
  }
  // Not our envelope: a proxy or gateway answered instead of the backend.
  throw new ApiError('UNKNOWN_ERROR', `Unexpected response (HTTP ${response.status}).`, response.status)
}

function postJSON<T>(path: string, payload: unknown): Promise<T> {
  return request<T>(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
}

export function getStock(itemId: string, signal?: AbortSignal): Promise<Stock> {
  return request<Stock>(`/stock?item_id=${encodeURIComponent(itemId)}`, { signal })
}

export function reserve(payload: ReserveRequest): Promise<Reservation> {
  return postJSON<Reservation>('/reserve', payload)
}

export function confirm(reservationId: string): Promise<Confirmation> {
  return postJSON<Confirmation>('/confirm', { reservation_id: reservationId })
}
