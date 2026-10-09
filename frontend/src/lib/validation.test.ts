import { describe, expect, it } from 'vitest'
import { validateReserveForm } from './validation'

describe('validateReserveForm', () => {
  it('accepts a valid form', () => {
    expect(validateReserveForm({ userId: 'usr_9981', quantity: '2' })).toEqual({})
  })

  it('accepts the quantity limits', () => {
    expect(validateReserveForm({ userId: 'u', quantity: '1' })).toEqual({})
    expect(validateReserveForm({ userId: 'u', quantity: '1000' })).toEqual({})
  })

  it('ignores surrounding spaces', () => {
    expect(validateReserveForm({ userId: '  usr_1  ', quantity: ' 3 ' })).toEqual({})
  })

  it.each([
    ['empty', ''],
    ['spaces only', '   '],
    ['a space inside', 'usr 1'],
    ['a symbol', 'usr@1'],
    ['more than 64 characters', 'a'.repeat(65)],
  ])('rejects a user ID that is %s', (_name, userId) => {
    expect(validateReserveForm({ userId, quantity: '1' }).userId).toBeDefined()
  })

  it.each([
    ['empty', ''],
    ['zero', '0'],
    ['negative', '-1'],
    ['fractional', '2.5'],
    ['not a number', 'two'],
    ['exponent notation', '1e2'],
    ['above the maximum', '1001'],
  ])('rejects a quantity that is %s', (_name, quantity) => {
    expect(validateReserveForm({ userId: 'usr_1', quantity }).quantity).toBeDefined()
  })

  it('reports both fields at once', () => {
    const errors = validateReserveForm({ userId: '', quantity: '' })
    expect(Object.keys(errors).sort()).toEqual(['quantity', 'userId'])
  })
})
