import { ID_PATTERN, MAX_QUANTITY, MIN_QUANTITY } from '../config'

export interface ReserveFormValues {
  userId: string
  quantity: string
}

export type ReserveFormErrors = Partial<Record<keyof ReserveFormValues, string>>

/**
 * Checks the form with the same rules the backend applies, so most
 * mistakes are caught before a request is sent. The backend still has the
 * final say.
 */
export function validateReserveForm(values: ReserveFormValues): ReserveFormErrors {
  const errors: ReserveFormErrors = {}

  const userId = values.userId.trim()
  if (userId === '') {
    errors.userId = 'Enter a user ID.'
  } else if (!ID_PATTERN.test(userId)) {
    errors.userId = 'Use 1 to 64 letters, digits, underscores or hyphens.'
  }

  const quantity = values.quantity.trim()
  if (quantity === '') {
    errors.quantity = 'Enter a quantity.'
  } else if (!/^\d+$/.test(quantity)) {
    errors.quantity = 'Enter a whole number.'
  } else {
    const n = Number(quantity)
    if (n < MIN_QUANTITY || n > MAX_QUANTITY) {
      errors.quantity = `Enter a number from ${MIN_QUANTITY} to ${MAX_QUANTITY}.`
    }
  }

  return errors
}
