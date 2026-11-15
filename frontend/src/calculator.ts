import { expressionFor, pendingExpression, type Operation } from './operations'

export type Phase = 'idle' | 'entering' | 'calculating' | 'result' | 'error'

export type CalculationRequest = {
  operation: Operation
  operands: string[]
  expression: string
}

export type State = {
  entry: string
  accumulator: string | null
  pending: Operation | null
  queued: Operation | null
  phase: Phase
  result: string | null
  error: string | null
  expression: string
  request: CalculationRequest | null
  failed: CalculationRequest | null
}

export type Action =
  | { type: 'digit'; digit: string }
  | { type: 'dot' }
  | { type: 'sign' }
  | { type: 'backspace' }
  | { type: 'clear' }
  | { type: 'operator'; operation: Operation }
  | { type: 'equals' }
  | { type: 'sqrt' }
  | { type: 'requestSucceeded'; result: string; expression: string }
  | { type: 'requestFailed'; message: string; expression: string }

export const initialState: State = {
  entry: '',
  accumulator: null,
  pending: null,
  queued: null,
  phase: 'idle',
  result: null,
  error: null,
  expression: '',
  request: null,
  failed: null,
}

function startFresh(entry: string): State {
  return { ...initialState, entry, phase: 'entering' }
}

function isSettled(phase: Phase): boolean {
  return phase === 'result' || phase === 'error'
}

// The entry is the string that will be sent as an operand, so it never holds a
// lone minus or a bare decimal point: both are unparseable as a decimal.
function appendDigit(entry: string, digit: string): string {
  if (entry === '0' || entry === '-0') {
    return (entry.startsWith('-') ? '-' : '') + digit
  }
  return entry + digit
}

export function reducer(state: State, action: Action): State {
  switch (action.type) {
    case 'digit': {
      if (state.phase === 'calculating') return state
      if (isSettled(state.phase)) return startFresh(action.digit)
      return { ...state, entry: appendDigit(state.entry, action.digit), phase: 'entering' }
    }
    case 'dot': {
      if (state.phase === 'calculating') return state
      if (isSettled(state.phase)) return startFresh('0.')
      if (state.entry.includes('.')) return state
      return { ...state, entry: state.entry === '' ? '0.' : state.entry + '.', phase: 'entering' }
    }
    case 'sign': {
      if (state.phase === 'calculating' || isSettled(state.phase) || state.entry === '') return state
      const entry = state.entry.startsWith('-') ? state.entry.slice(1) : '-' + state.entry
      return { ...state, entry }
    }
    case 'backspace': {
      if (state.phase === 'calculating') return state
      if (isSettled(state.phase)) return startFresh('')
      if (state.entry === '') return state
      const next = state.entry.slice(0, -1)
      return { ...state, entry: next === '-' ? '' : next, phase: 'entering' }
    }
    case 'clear':
      if (state.phase === 'calculating') return state
      return initialState
    case 'operator': {
      if (state.phase === 'calculating' || state.phase === 'error') return state
      if (state.pending !== null && state.accumulator !== null && state.entry !== '') {
        const expression = expressionFor(state.pending, state.accumulator, state.entry)
        return {
          ...state,
          phase: 'calculating',
          expression,
          request: { operation: state.pending, operands: [state.accumulator, state.entry], expression },
          queued: action.operation,
        }
      }
      const left = state.entry !== '' ? state.entry : state.accumulator
      if (left === null) return state
      return {
        ...state,
        accumulator: left,
        pending: action.operation,
        queued: null,
        entry: '',
        phase: 'entering',
        result: null,
        error: null,
        expression: pendingExpression(action.operation, left),
      }
    }
    case 'equals': {
      if (state.phase === 'calculating') return state
      // A failure retries the request that failed, not whatever the hidden operands
      // would produce: a failed square root has no pending binary operation.
      if (state.phase === 'error' && state.failed !== null) {
        return { ...state, phase: 'calculating', error: null, request: state.failed, expression: state.failed.expression }
      }
      if (!state.pending || state.accumulator === null || state.entry === '') return state
      const expression = expressionFor(state.pending, state.accumulator, state.entry)
      return {
        ...state,
        phase: 'calculating',
        expression,
        request: { operation: state.pending, operands: [state.accumulator, state.entry], expression },
      }
    }
    case 'sqrt': {
      if (state.phase === 'calculating' || state.phase === 'error') return state
      const value = state.entry !== '' ? state.entry : (state.accumulator ?? '0')
      const expression = expressionFor('sqrt', value)
      return {
        ...state,
        phase: 'calculating',
        expression,
        request: { operation: 'sqrt', operands: [value], expression },
      }
    }
    case 'requestSucceeded':
      // A square root is unary: when it was applied to the right operand of a
      // pending operation, the operation survives and waits for the result.
      if (state.request?.operation === 'sqrt' && state.pending !== null && state.accumulator !== null) {
        return {
          ...state,
          entry: action.result,
          phase: 'entering',
          result: action.result,
          error: null,
          request: null,
          failed: null,
          expression: pendingExpression(state.pending, state.accumulator),
        }
      }
      if (state.queued !== null) {
        return {
          ...state,
          accumulator: action.result,
          pending: state.queued,
          queued: null,
          entry: '',
          phase: 'entering',
          result: action.result,
          error: null,
          request: null,
          failed: null,
          expression: pendingExpression(state.queued, action.result),
        }
      }
      return {
        ...initialState,
        accumulator: action.result,
        result: action.result,
        phase: 'result',
        expression: `${action.expression} =`,
      }
    case 'requestFailed':
      return {
        ...state,
        phase: 'error',
        error: action.message,
        request: null,
        queued: null,
        failed: state.request,
        expression: action.expression,
      }
  }
}

export function visibleValue(state: State): { value: string; isError: boolean } {
  if (state.phase === 'error') return { value: state.error ?? '', isError: true }
  if (state.phase === 'result') return { value: state.result ?? '', isError: false }
  if (state.entry !== '') return { value: state.entry, isError: false }
  return { value: state.accumulator ?? '0', isError: false }
}
