// The one item this dashboard sells. The backend accepts any item ID; the
// dashboard is fixed to this one.
export const ITEM_ID = 'item_4021'

export const API_BASE = '/api/v1/inventory'

// How often the stock panel refreshes itself.
export const STOCK_POLL_INTERVAL_MS = 2000

// Limits the backend enforces, mirrored here so the form can validate
// before sending.
export const ID_PATTERN = /^[A-Za-z0-9_-]{1,64}$/
export const MIN_QUANTITY = 1
export const MAX_QUANTITY = 1000
