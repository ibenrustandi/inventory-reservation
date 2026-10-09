import { useQuery } from '@tanstack/react-query'
import { getStock } from '../api/client'
import { STOCK_POLL_INTERVAL_MS } from '../config'

export const stockQueryKey = (itemId: string) => ['stock', itemId] as const

/**
 * Live stock of one item. It refreshes itself on an interval, pauses while
 * the browser tab is hidden, and refreshes again when the tab is shown.
 */
export function useStock(itemId: string) {
  return useQuery({
    queryKey: stockQueryKey(itemId),
    queryFn: ({ signal }) => getStock(itemId, signal),
    refetchInterval: STOCK_POLL_INTERVAL_MS,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: true,
  })
}
