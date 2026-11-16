import type { Action } from './calculator'

type Key = {
  label: string
  ariaLabel: string
  action: Action
  className?: string
}

const KEYS: Key[] = [
  { label: 'C', ariaLabel: 'clear', action: { type: 'clear' }, className: 'key--clear' },
  { label: '⌫', ariaLabel: 'backspace', action: { type: 'backspace' }, className: 'key--function' },
  { label: '±', ariaLabel: 'change sign', action: { type: 'sign' }, className: 'key--function' },
  { label: '÷', ariaLabel: 'divide', action: { type: 'operator', operation: 'divide' }, className: 'key--operator' },

  { label: '√', ariaLabel: 'square root', action: { type: 'sqrt' }, className: 'key--function' },
  { label: 'xʸ', ariaLabel: 'power', action: { type: 'operator', operation: 'power' }, className: 'key--function' },
  { label: '%', ariaLabel: 'percent of', action: { type: 'operator', operation: 'percent' }, className: 'key--function' },
  { label: '×', ariaLabel: 'multiply', action: { type: 'operator', operation: 'multiply' }, className: 'key--operator' },

  { label: '7', ariaLabel: '7', action: { type: 'digit', digit: '7' } },
  { label: '8', ariaLabel: '8', action: { type: 'digit', digit: '8' } },
  { label: '9', ariaLabel: '9', action: { type: 'digit', digit: '9' } },
  { label: '−', ariaLabel: 'subtract', action: { type: 'operator', operation: 'subtract' }, className: 'key--operator' },

  { label: '4', ariaLabel: '4', action: { type: 'digit', digit: '4' } },
  { label: '5', ariaLabel: '5', action: { type: 'digit', digit: '5' } },
  { label: '6', ariaLabel: '6', action: { type: 'digit', digit: '6' } },
  { label: '+', ariaLabel: 'add', action: { type: 'operator', operation: 'add' }, className: 'key--operator' },

  { label: '1', ariaLabel: '1', action: { type: 'digit', digit: '1' } },
  { label: '2', ariaLabel: '2', action: { type: 'digit', digit: '2' } },
  { label: '3', ariaLabel: '3', action: { type: 'digit', digit: '3' } },
  { label: '=', ariaLabel: 'equals', action: { type: 'equals' }, className: 'key--equals' },

  { label: '0', ariaLabel: '0', action: { type: 'digit', digit: '0' }, className: 'key--zero' },
  { label: '.', ariaLabel: 'decimal point', action: { type: 'dot' } },
]

type Props = {
  dispatch: (action: Action) => void
  busy: boolean
  errorPhase: boolean
  canRetry: boolean
  onCancel: () => void
}

// In the error phase the reducer ignores these actions, so the keys are
// disabled rather than left looking pressable.
function inertInError(action: Action): boolean {
  return action.type === 'operator' || action.type === 'sqrt' || action.type === 'sign'
}

export function Keypad({ dispatch, busy, errorPhase, canRetry, onCancel }: Props) {
  return (
    <div className="keypad">
      {KEYS.map((key) => {
        // While a request is in flight every key is disabled except the first,
        // which becomes a touch-reachable cancel control.
        const cancels = busy && key.action.type === 'clear'
        // Equals retries a failed request; after a local refusal there is no
        // request to retry, so it is disabled along with the other inert keys.
        const inert =
          (errorPhase && inertInError(key.action)) ||
          (errorPhase && !canRetry && key.action.type === 'equals')
        return (
          <button
            key={key.ariaLabel}
            type="button"
            className={cancels ? 'key key--clear key--cancel' : key.className ? `key ${key.className}` : 'key'}
            aria-label={cancels ? 'cancel request' : key.ariaLabel}
            disabled={!cancels && (busy || inert)}
            onClick={(event) => {
              // detail > 0 means a pointer click; dropping focus keeps Enter meaning equals.
              if (event.detail > 0) event.currentTarget.blur()
              if (cancels) onCancel()
              else dispatch(key.action)
            }}
          >
            {cancels ? 'Cancel' : key.label}
          </button>
        )
      })}
    </div>
  )
}
