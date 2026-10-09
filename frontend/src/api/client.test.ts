import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, confirm, getStock, reserve } from './client'

function respondWith(status: number, body: unknown) {
  const text = typeof body === 'string' ? body : JSON.stringify(body)
  const fetchMock = vi.fn<typeof fetch>(async () => new Response(text, { status }))
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

async function errorFrom(promise: Promise<unknown>): Promise<ApiError> {
  try {
    await promise
  } catch (error) {
    expect(error).toBeInstanceOf(ApiError)
    return error as ApiError
  }
  throw new Error('expected the call to fail')
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('api client', () => {
  it('returns the stock breakdown', async () => {
    const stock = { item_id: 'item_4021', total_stock: 100, reserved_stock: 15, available_stock: 85 }
    const fetchMock = respondWith(200, stock)

    await expect(getStock('item_4021')).resolves.toEqual(stock)
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/inventory/stock?item_id=item_4021',
      expect.anything(),
    )
  })

  it('escapes the item id in the query string', async () => {
    const fetchMock = respondWith(200, {})

    await getStock('a b&c')

    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/inventory/stock?item_id=a%20b%26c')
  })

  it('posts a reservation as JSON', async () => {
    const fetchMock = respondWith(201, { status: 'success', reservation_id: 'res_1' })

    await reserve({ user_id: 'usr_1', item_id: 'item_4021', quantity: 2 })

    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/inventory/reserve')
    expect(init?.method).toBe('POST')
    expect(JSON.parse(init?.body as string)).toEqual({
      user_id: 'usr_1',
      item_id: 'item_4021',
      quantity: 2,
    })
  })

  it('posts the reservation id to confirm', async () => {
    const fetchMock = respondWith(200, { status: 'success' })

    await confirm('res_1')

    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/inventory/confirm')
    expect(JSON.parse(init?.body as string)).toEqual({ reservation_id: 'res_1' })
  })

  it('turns the error envelope into an ApiError', async () => {
    respondWith(409, {
      status: 'error',
      error: {
        code: 'INSUFFICIENT_STOCK',
        message: 'Not enough stock is available.',
        details: { available_stock: 1, requested: 2 },
      },
      request_id: 'req_abc',
    })

    const error = await errorFrom(reserve({ user_id: 'u', item_id: 'i', quantity: 2 }))

    expect(error.code).toBe('INSUFFICIENT_STOCK')
    expect(error.status).toBe(409)
    expect(error.message).toBe('Not enough stock is available.')
    expect(error.details).toEqual({ available_stock: 1, requested: 2 })
    expect(error.requestId).toBe('req_abc')
  })

  it('exposes the invalid fields of an INVALID_REQUEST', async () => {
    respondWith(400, {
      status: 'error',
      error: {
        code: 'INVALID_REQUEST',
        message: 'The request is invalid.',
        details: { fields: [{ field: 'quantity', reason: 'is required' }] },
      },
      request_id: 'req_abc',
    })

    const error = await errorFrom(reserve({ user_id: 'u', item_id: 'i', quantity: 0 }))

    expect(error.fields).toEqual([{ field: 'quantity', reason: 'is required' }])
  })

  it('has no fields when the error lists none', async () => {
    respondWith(410, {
      status: 'error',
      error: { code: 'RESERVATION_EXPIRED', message: 'The reservation has expired.' },
      request_id: 'req_abc',
    })

    const error = await errorFrom(confirm('res_1'))

    expect(error.code).toBe('RESERVATION_EXPIRED')
    expect(error.fields).toEqual([])
    expect(error.details).toEqual({})
  })

  it('reports a network failure as NETWORK_ERROR', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn<typeof fetch>(async () => Promise.reject(new TypeError('Failed to fetch'))),
    )

    const error = await errorFrom(getStock('item_4021'))

    expect(error.code).toBe('NETWORK_ERROR')
    expect(error.status).toBe(0)
  })

  it('reports an error that is not the envelope as UNKNOWN_ERROR', async () => {
    respondWith(502, '<html>Bad Gateway</html>')

    const error = await errorFrom(getStock('item_4021'))

    expect(error.code).toBe('UNKNOWN_ERROR')
    expect(error.status).toBe(502)
  })
})
