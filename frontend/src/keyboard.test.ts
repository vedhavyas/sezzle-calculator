import { describe, expect, it } from 'vitest'

import { actionForKey } from './keyboard'

describe('actionForKey', () => {
  it('maps every digit', () => {
    for (let value = 0; value <= 9; value += 1) {
      expect(actionForKey(String(value)), String(value)).toEqual({ type: 'digit', digit: String(value) })
    }
  })

  it('maps every operator and the decimal point', () => {
    const cases: Array<[string, unknown]> = [
      ['.', { type: 'dot' }],
      ['+', { type: 'operator', operation: 'add' }],
      ['-', { type: 'operator', operation: 'subtract' }],
      ['*', { type: 'operator', operation: 'multiply' }],
      ['/', { type: 'operator', operation: 'divide' }],
      ['^', { type: 'operator', operation: 'power' }],
      ['%', { type: 'operator', operation: 'percent' }],
      ['Enter', { type: 'equals' }],
      ['=', { type: 'equals' }],
      ['Escape', { type: 'clear' }],
      ['Backspace', { type: 'backspace' }],
    ]
    for (const [key, action] of cases) {
      expect(actionForKey(key), key).toEqual(action)
    }
  })

  it('leaves browser shortcuts and unused keys alone', () => {
    for (const key of ['a', 'c', 'C', 'Shift', 'F5', 'ArrowLeft', '']) {
      expect(actionForKey(key), key).toBeNull()
    }
  })
})
