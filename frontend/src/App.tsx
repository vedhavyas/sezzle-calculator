import { useEffect, useReducer } from 'react'

import { calculate } from './api'
import { Display } from './Display'
import { initialState, reducer, visibleValue } from './calculator'
import { Keypad } from './Keypad'
import { actionForKey } from './keyboard'

export function App() {
  const [state, dispatch] = useReducer(reducer, initialState)
  const busy = state.phase === 'calculating'

  useEffect(() => {
    const request = state.request
    if (!request) return

    calculate(request.operation, request.operands)
      .then((result) => dispatch({ type: 'requestSucceeded', result, expression: request.expression }))
      .catch((error: unknown) => {
        const message = error instanceof Error ? error.message : 'Could not reach the calculator service.'
        dispatch({ type: 'requestFailed', message, expression: request.expression })
      })
  }, [state.request])

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (busy) return
      if (event.ctrlKey || event.metaKey || event.altKey) return
      // A focused keypad button owns Enter: native activation must win over equals.
      if (event.key === 'Enter' && event.target instanceof HTMLButtonElement) return
      const action = actionForKey(event.key)
      if (!action) return
      event.preventDefault()
      // Once the keyboard is driving, drop focus so Enter means equals rather than
      // re-activating whichever key was tabbed to.
      if (document.activeElement instanceof HTMLButtonElement) document.activeElement.blur()
      dispatch(action)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [busy])

  const { value, isError } = visibleValue(state)

  return (
    <main className="app">
      <h1 className="sr-only">Calculator</h1>
      <div className="calculator" data-busy={busy ? 'true' : undefined} aria-busy={busy}>
        <Display expression={state.expression} value={value} isError={isError} />
        <Keypad dispatch={dispatch} disabled={busy} />
      </div>
    </main>
  )
}
