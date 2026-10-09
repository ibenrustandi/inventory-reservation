import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useId, useState } from 'react'
import type { FormEvent } from 'react'
import { reserve } from '../api/client'
import type { Reservation } from '../api/types'
import { MAX_QUANTITY, MIN_QUANTITY } from '../config'
import { stockQueryKey } from '../hooks/useStock'
import { describeError, fieldErrorsOf } from '../lib/errorMessages'
import { validateReserveForm } from '../lib/validation'
import type { ReserveFormErrors } from '../lib/validation'

interface ReserveFormProps {
  itemId: string
  onReserved: (reservation: Reservation) => void
}

export function ReserveForm({ itemId, onReserved }: ReserveFormProps) {
  const [userId, setUserId] = useState('')
  const [quantity, setQuantity] = useState('1')
  const [formErrors, setFormErrors] = useState<ReserveFormErrors>({})
  const queryClient = useQueryClient()
  const id = useId()

  const mutation = useMutation({
    mutationFn: reserve,
    onSuccess: (reservation) => onReserved(reservation),
    // Whether it worked or not, the stock on screen may now be out of date.
    onSettled: () => queryClient.invalidateQueries({ queryKey: stockQueryKey(itemId) }),
  })

  // Messages the form found itself win over the backend's for the same field.
  const errors: ReserveFormErrors = { ...fieldErrorsOf(mutation.error), ...formErrors }
  const hasFieldError = errors.userId !== undefined || errors.quantity !== undefined

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const found = validateReserveForm({ userId, quantity })
    setFormErrors(found)
    if (Object.keys(found).length > 0) {
      mutation.reset()
      return
    }
    mutation.mutate({ user_id: userId.trim(), item_id: itemId, quantity: Number(quantity.trim()) })
  }

  return (
    <section className="card" aria-labelledby="reserve-heading">
      <h2 id="reserve-heading">Reserve stock</h2>
      <p className="muted">A reservation holds your units for 5 minutes.</p>

      <form className="form" onSubmit={handleSubmit} noValidate>
        <div className="field">
          <label htmlFor={`${id}-user`}>User ID</label>
          <input
            id={`${id}-user`}
            name="user_id"
            type="text"
            autoComplete="off"
            placeholder="usr_9981"
            value={userId}
            onChange={(event) => setUserId(event.target.value)}
            aria-invalid={errors.userId !== undefined}
            aria-describedby={errors.userId ? `${id}-user-error` : undefined}
            disabled={mutation.isPending}
          />
          {errors.userId && (
            <p className="field-error" id={`${id}-user-error`}>
              {errors.userId}
            </p>
          )}
        </div>

        <div className="field">
          <label htmlFor={`${id}-quantity`}>Quantity</label>
          <input
            id={`${id}-quantity`}
            name="quantity"
            type="number"
            inputMode="numeric"
            min={MIN_QUANTITY}
            max={MAX_QUANTITY}
            step={1}
            value={quantity}
            onChange={(event) => setQuantity(event.target.value)}
            aria-invalid={errors.quantity !== undefined}
            aria-describedby={errors.quantity ? `${id}-quantity-error` : undefined}
            disabled={mutation.isPending}
          />
          {errors.quantity && (
            <p className="field-error" id={`${id}-quantity-error`}>
              {errors.quantity}
            </p>
          )}
        </div>

        <button type="submit" className="button button-primary" disabled={mutation.isPending}>
          {mutation.isPending ? 'Reserving…' : 'Reserve'}
        </button>
      </form>

      {/* A failed request that is not about one field, such as sold out. */}
      {mutation.isError && !hasFieldError && (
        <p className="notice notice-error" role="alert">
          {describeError(mutation.error)}
        </p>
      )}
    </section>
  )
}
