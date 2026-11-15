import { describe, expect, it } from 'vitest'

import { initialState, reducer, visibleValue, type Action, type State } from './calculator'
import { expressionFor, pendingExpression } from './operations'

function run(actions: Action[], start: State = initialState): State {
  return actions.reduce(reducer, start)
}

const digit = (value: number): Action => ({ type: 'digit', digit: String(value) })

const busy = run([digit(1), { type: 'operator', operation: 'add' }, digit(2), { type: 'equals' }])
const settled = reducer(busy, { type: 'requestSucceeded', result: '3', expression: '1 + 2' })
const failed = reducer(busy, { type: 'requestFailed', message: 'nope', expression: '1 + 2' })

describe('entries', () => {
  it('builds a number from digits, replacing a leading zero', () => {
    expect(run([digit(0), digit(5), digit(2)]).entry).toBe('52')
  })

  it('keeps a negative entry canonical', () => {
    expect(run([digit(0), { type: 'sign' }, digit(5)]).entry).toBe('-5')
  })

  it('accepts one decimal point and puts a zero in front of a leading one', () => {
    expect(run([{ type: 'dot' }, digit(5)]).entry).toBe('0.5')
    expect(run([digit(1), { type: 'dot' }, { type: 'dot' }, digit(2)]).entry).toBe('1.2')
    expect(run([digit(5), { type: 'sign' }, { type: 'backspace' }, { type: 'dot' }]).entry).toBe('0.')
  })

  it('toggles the sign of a non-empty entry only', () => {
    expect(run([digit(5), { type: 'sign' }]).entry).toBe('-5')
    expect(run([digit(5), { type: 'sign' }, { type: 'sign' }]).entry).toBe('5')
    expect(run([{ type: 'sign' }]).entry).toBe('')
  })

  it('never leaves a bare minus behind on backspace', () => {
    expect(run([digit(5), { type: 'sign' }, { type: 'backspace' }]).entry).toBe('')
    expect(run([digit(1), digit(2), { type: 'backspace' }]).entry).toBe('1')
  })

  it('returns to the initial state on clear, from every phase that is not in flight', () => {
    for (const state of [initialState, run([digit(1)]), settled, failed]) {
      expect(reducer(state, { type: 'clear' })).toEqual(initialState)
    }
  })
})

describe('operations', () => {
  it('holds the entry as the left operand when an operator is pressed', () => {
    const state = run([digit(1), digit(2), { type: 'operator', operation: 'add' }])
    expect(state).toMatchObject({ accumulator: '12', pending: 'add', entry: '', expression: '12 +' })
  })

  it('replaces the pending operator instead of stacking it', () => {
    const state = run([digit(1), { type: 'operator', operation: 'add' }, { type: 'operator', operation: 'multiply' }])
    expect(state.pending).toBe('multiply')
    expect(state.accumulator).toBe('1')
  })

  it('evaluates left to right as you go', () => {
    const chaining = run([digit(2), { type: 'operator', operation: 'add' }, digit(3), { type: 'operator', operation: 'multiply' }])

    expect(chaining.phase).toBe('calculating')
    expect(chaining.request).toMatchObject({ operation: 'add', operands: ['2', '3'] })

    const afterSum = reducer(chaining, { type: 'requestSucceeded', result: '5', expression: '2 + 3' })
    expect(afterSum).toMatchObject({ accumulator: '5', pending: 'multiply', entry: '', expression: '5 ×' })
    expect(visibleValue(afterSum)).toEqual({ value: '5', isError: false })

    expect(run([digit(4), { type: 'equals' }], afterSum).request).toMatchObject({
      operation: 'multiply',
      operands: ['5', '4'],
    })
  })

  it('asks for one calculation per equals, carrying both operands', () => {
    expect(busy.phase).toBe('calculating')
    expect(busy.request).toEqual({ operation: 'add', operands: ['1', '2'], expression: '1 + 2' })
  })

  it('does nothing on equals without a complete operation', () => {
    expect(run([digit(1), { type: 'equals' }]).phase).toBe('entering')
    expect(run([digit(1), { type: 'operator', operation: 'add' }, { type: 'equals' }]).request).toBeNull()
  })

  it('sends sqrt of the entry, of zero when empty, and of the shown number after an operator', () => {
    expect(run([digit(1), digit(6), { type: 'sqrt' }]).request).toMatchObject({ operation: 'sqrt', operands: ['16'] })
    expect(run([{ type: 'sqrt' }]).request).toMatchObject({ operation: 'sqrt', operands: ['0'] })
    expect(run([digit(9), { type: 'operator', operation: 'subtract' }, { type: 'sqrt' }]).request).toMatchObject({
      operation: 'sqrt',
      operands: ['9'],
    })
    expect(reducer(settled, { type: 'sqrt' }).request).toMatchObject({ operation: 'sqrt', operands: ['3'] })
  })

  it('keeps the number on screen while its square root is in flight', () => {
    const state = run([digit(1), digit(6), { type: 'sqrt' }])
    expect(visibleValue(state)).toEqual({ value: '16', isError: false })
  })

  it('keeps a pending operation when a square root succeeds inside it', () => {
    const pending = run([digit(9), { type: 'operator', operation: 'subtract' }, digit(1), digit(6)])
    const rooting = reducer(pending, { type: 'sqrt' })
    const rooted = reducer(rooting, { type: 'requestSucceeded', result: '4', expression: '√16' })

    expect(rooted).toMatchObject({ entry: '4', pending: 'subtract', accumulator: '9' })
    expect(reducer(rooted, { type: 'equals' }).request).toMatchObject({
      operation: 'subtract',
      operands: ['9', '4'],
    })
  })

  it('reads percent as a percent of b', () => {
    const state = run([digit(1), digit(5), { type: 'operator', operation: 'percent' }, digit(2), digit(0), digit(0), { type: 'equals' }])
    expect(state.request).toMatchObject({ operation: 'percent', operands: ['15', '200'] })
    expect(state.expression).toBe('15% of 200')
    expect(reducer(state, { type: 'requestSucceeded', result: '30', expression: '15% of 200' }).expression).toBe('15% of 200 =')
  })

  it('ignores every key while a calculation is in flight', () => {
    const ignored: Action[] = [
      digit(9),
      { type: 'dot' },
      { type: 'sign' },
      { type: 'backspace' },
      { type: 'operator', operation: 'multiply' },
      { type: 'sqrt' },
      { type: 'equals' },
      { type: 'clear' },
    ]
    for (const action of ignored) {
      expect(reducer(busy, action), action.type).toBe(busy)
    }
  })
})

describe('results and failures', () => {
  it('shows the server result, appends the equals, and carries it forward', () => {
    expect(settled.phase).toBe('result')
    expect(settled.expression).toBe('1 + 2 =')
    expect(visibleValue(settled)).toEqual({ value: '3', isError: false })
    expect(run([{ type: 'operator', operation: 'add' }], settled).accumulator).toBe('3')
  })

  it('starts a new calculation when a digit follows a result', () => {
    expect(run([digit(7)], settled)).toMatchObject({ entry: '7', accumulator: null, expression: '' })
  })

  it('keeps the operands after a failure so equals retries', () => {
    expect(visibleValue(failed)).toEqual({ value: 'nope', isError: true })
    expect(reducer(failed, { type: 'equals' }).request).toMatchObject({ operation: 'add', operands: ['1', '2'] })
    expect(run([digit(7)], failed)).toMatchObject({ entry: '7', error: null })
  })

  it('cannot edit hidden operands from the error phase', () => {
    expect(reducer(failed, { type: 'sign' })).toBe(failed)
  })

  it('retries a failed square root, not a hidden binary operation', () => {
    const pendingAdd = run([digit(2), { type: 'operator', operation: 'add' }, digit(4), { type: 'sign' }])
    const attempted = reducer(pendingAdd, { type: 'sqrt' })
    expect(attempted.request).toMatchObject({ operation: 'sqrt', operands: ['-4'] })

    const refused = reducer(attempted, { type: 'requestFailed', message: 'negative_sqrt', expression: '√-4' })
    expect(reducer(refused, { type: 'equals' }).request).toMatchObject({ operation: 'sqrt', operands: ['-4'] })
  })

  it('ignores an operator in the error phase', () => {
    expect(reducer(failed, { type: 'operator', operation: 'multiply' })).toBe(failed)
  })

  it('does not carry over a queued operator when a chained request fails', () => {
    const chaining = run([digit(2), { type: 'operator', operation: 'add' }, digit(3), { type: 'operator', operation: 'multiply' }])
    const chainedFailure = reducer(chaining, { type: 'requestFailed', message: 'nope', expression: '2 + 3' })
    expect(chainedFailure.queued).toBeNull()
    expect(reducer(chainedFailure, { type: 'equals' }).request).toMatchObject({ operation: 'add', operands: ['2', '3'] })
  })
})

describe('expressions', () => {
  it('words each operation the way the display shows it', () => {
    expect(expressionFor('add', '1', '2')).toBe('1 + 2')
    expect(expressionFor('divide', '1', '2')).toBe('1 ÷ 2')
    expect(expressionFor('power', '2', '10')).toBe('2 ^ 10')
    expect(expressionFor('percent', '15', '200')).toBe('15% of 200')
    expect(expressionFor('sqrt', '16')).toBe('√16')
    expect(pendingExpression('percent', '15')).toBe('15% of')
    expect(pendingExpression('sqrt', '16')).toBe('√16')
  })
})

describe('visible value', () => {
  it('shows the accumulator while entering the right operand', () => {
    expect(visibleValue(run([digit(1), digit(2), { type: 'operator', operation: 'add' }])).value).toBe('12')
  })

  it('starts at zero', () => {
    expect(visibleValue(initialState).value).toBe('0')
  })
})
