import { useCallback, useEffect, useReducer, useRef, useState } from 'react'

import { calculate } from './api'
import { Display } from './Display'
import { initialState, reducer, visibleValue } from './calculator'
import { Keypad } from './Keypad'
import { actionForKey } from './keyboard'
import { applyTheme, nextTheme, readTheme, saveTheme, themeLabel, type Theme } from './theme'

const REQUEST_TIMEOUT_MS = 10_000

export function App() {
  const [state, dispatch] = useReducer(reducer, initialState)
  const [theme, setTheme] = useState<Theme>(readTheme)
  const controller = useRef<AbortController | null>(null)
  const cancelled = useRef(false)
  const busy = state.phase === 'calculating'

  const cancelRequest = useCallback(() => {
    cancelled.current = true
    controller.current?.abort()
    dispatch({ type: 'cancel' })
  }, [])

  useEffect(() => {
    applyTheme(theme)
    saveTheme(theme)
  }, [theme])

  useEffect(() => {
    const request = state.request
    if (!request) return

    const current = new AbortController()
    controller.current = current
    cancelled.current = false
    let finished = false
    const timer = setTimeout(
      () => current.abort(new DOMException('The request timed out.', 'TimeoutError')),
      REQUEST_TIMEOUT_MS,
    )

    calculate(request.operation, request.operands, current.signal)
      .then((result) => {
        if (finished || cancelled.current) return
        dispatch({ type: 'requestSucceeded', result, expression: request.expression })
      })
      .catch((error: unknown) => {
        if (finished || cancelled.current) return
        const message = error instanceof Error ? error.message : 'Could not reach the calculator service.'
        dispatch({ type: 'requestFailed', message, expression: request.expression })
      })
      .finally(() => clearTimeout(timer))

    return () => {
      finished = true
      clearTimeout(timer)
      current.abort()
    }
  }, [state.request])

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      // Escape always cancels: in flight it aborts and resets, otherwise it
      // clears. It does not depend on the busy flag, so a rebind of the
      // listener cannot drop it mid-request.
      if (event.key === 'Escape') {
        event.preventDefault()
        cancelRequest()
        return
      }
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
  }, [busy, cancelRequest])

  const { value, isError } = visibleValue(state)

  return (
    <main className="app">
      <h1 className="sr-only">Calculator</h1>
      <span className="sr-only" role="status" data-testid="working-status">
        {busy ? 'Calculating' : ''}
      </span>
      <div className="app__bar">
        <span className="sr-only" role="status" data-testid="theme-status">
          Theme: {themeLabel(theme)}
        </span>
        <button
          type="button"
          className="theme-toggle"
          onClick={(event) => {
            if (event.detail > 0) event.currentTarget.blur()
            setTheme((current) => nextTheme(current))
          }}
        >
          Theme: {themeLabel(theme)}
        </button>
      </div>
      <div className="calculator" aria-busy={busy}>
        <Display expression={state.expression} value={value} isError={isError} />
        <Keypad dispatch={dispatch} busy={busy} errorPhase={state.phase === 'error'} onCancel={cancelRequest} />
      </div>
    </main>
  )
}
