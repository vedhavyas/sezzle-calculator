import type { Action } from './calculator'

const OPERATOR_KEYS: Record<string, Action> = {
  '+': { type: 'operator', operation: 'add' },
  '-': { type: 'operator', operation: 'subtract' },
  '*': { type: 'operator', operation: 'multiply' },
  '/': { type: 'operator', operation: 'divide' },
  '^': { type: 'operator', operation: 'power' },
  '%': { type: 'operator', operation: 'percent' },
}

export function actionForKey(key: string): Action | null {
  if (key.length === 1 && key >= '0' && key <= '9') return { type: 'digit', digit: key }
  if (Object.hasOwn(OPERATOR_KEYS, key)) return OPERATOR_KEYS[key]
  switch (key) {
    case '.':
      return { type: 'dot' }
    case 'Enter':
    case '=':
      return { type: 'equals' }
    case 'Escape':
      return { type: 'clear' }
    case 'Backspace':
      return { type: 'backspace' }
    // '-' is subtract, so a letter gives the keyboard a way to negate a value.
    case 'n':
    case 'N':
      return { type: 'sign' }
    default:
      return null
  }
}
