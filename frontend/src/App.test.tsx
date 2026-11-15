import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { App } from './App'

const fetchMock = vi.fn()

const KEY_LABELS = [
  'clear',
  'backspace',
  'change sign',
  'divide',
  'square root',
  'power',
  'percent of',
  'multiply',
  '7',
  '8',
  '9',
  'subtract',
  '4',
  '5',
  '6',
  'add',
  '1',
  '2',
  '3',
  'equals',
  '0',
  'decimal point',
]

function succeed(result: string) {
  fetchMock.mockResolvedValueOnce({ ok: true, status: 200, json: async () => ({ result }) })
}

function refuse(status: number, code: string, message: string) {
  fetchMock.mockResolvedValueOnce({ ok: false, status, json: async () => ({ error: { code, message } }) })
}

function key(name: string) {
  return screen.getByRole('button', { name })
}

function value() {
  return screen.getByTestId('value')
}

function sentBody(call: number) {
  expect(fetchMock.mock.calls.length).toBeGreaterThan(call)
  return JSON.parse(fetchMock.mock.calls[call][1].body as string)
}

async function press(user: ReturnType<typeof userEvent.setup>, names: string[]) {
  for (const name of names) await user.click(key(name))
}

beforeEach(() => {
  fetchMock.mockReset()
  vi.stubGlobal('fetch', fetchMock)
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('app', () => {
  it('sends one request per equals and shows the server result', async () => {
    const user = userEvent.setup()
    succeed('15')
    render(<App />)

    await press(user, ['1', '2', 'add', '3', 'equals'])

    await waitFor(() => expect(value()).toHaveTextContent('15'))
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(sentBody(0)).toEqual({ operation: 'add', operands: ['12', '3'] })
    expect(screen.getByTestId('expression')).toHaveTextContent('12 + 3 =')
  })

  it.each([
    { keys: ['6', 'multiply', '7'], operation: 'multiply', operands: ['6', '7'] },
    { keys: ['8', 'divide', '2'], operation: 'divide', operands: ['8', '2'] },
    { keys: ['2', 'power', '3'], operation: 'power', operands: ['2', '3'] },
    { keys: ['1', '5', 'percent of', '2', '0', '0'], operation: 'percent', operands: ['15', '200'] },
    { keys: ['9', 'square root'], operation: 'sqrt', operands: ['9'] },
  ])('sends $operation for the matching key', async ({ keys, operation, operands }) => {
    const user = userEvent.setup()
    succeed('1')
    render(<App />)

    await press(user, [...keys, 'equals'])
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))

    expect(sentBody(0)).toEqual({ operation, operands })
  })

  it('evaluates left to right across requests', async () => {
    const user = userEvent.setup()
    succeed('5')
    succeed('20')
    render(<App />)

    await press(user, ['2', 'add', '3', 'multiply'])
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
    expect(sentBody(0)).toEqual({ operation: 'add', operands: ['2', '3'] })

    await press(user, ['4', 'equals'])
    await waitFor(() => expect(value()).toHaveTextContent('20'))
    expect(sentBody(1)).toEqual({ operation: 'multiply', operands: ['5', '4'] })
  })

  it('renders a refusal, hides the code, and recovers on clear', async () => {
    const user = userEvent.setup()
    refuse(422, 'division_by_zero', 'division by zero is undefined')
    render(<App />)

    await press(user, ['1', '0', 'divide', '0', 'equals'])

    await waitFor(() => expect(value()).toHaveTextContent('division by zero is undefined'))
    expect(value()).toHaveAttribute('data-error', 'true')
    expect(value()).not.toHaveTextContent('division_by_zero')

    await user.click(key('clear'))
    expect(value()).toHaveTextContent('0')
    expect(value()).not.toHaveAttribute('data-error')
  })

  it('starts a new calculation when a digit follows a refusal', async () => {
    const user = userEvent.setup()
    refuse(422, 'division_by_zero', 'division by zero is undefined')
    render(<App />)

    await press(user, ['1', '0', 'divide', '0', 'equals'])
    await waitFor(() => expect(value()).toHaveAttribute('data-error', 'true'))

    await user.click(key('7'))
    expect(value()).toHaveTextContent('7')
    expect(value()).not.toHaveAttribute('data-error')
  })

  it('retries the same operands when equals is pressed after a refusal', async () => {
    const user = userEvent.setup()
    refuse(422, 'division_by_zero', 'division by zero is undefined')
    succeed('5')
    render(<App />)

    await press(user, ['1', '0', 'divide', '0', 'equals'])
    await waitFor(() => expect(value()).toHaveAttribute('data-error', 'true'))

    await user.click(key('equals'))
    await waitFor(() => expect(value()).toHaveTextContent('5'))
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(sentBody(1)).toEqual({ operation: 'divide', operands: ['10', '0'] })
  })

  it('reports an unreachable service and stays usable', async () => {
    const user = userEvent.setup()
    fetchMock.mockRejectedValueOnce(new TypeError('Failed to fetch'))
    render(<App />)

    await press(user, ['2', 'add', '2', 'equals'])

    await waitFor(() => expect(value()).toHaveTextContent('Could not reach the calculator service.'))
    await user.click(key('clear'))
    expect(value()).toHaveTextContent('0')
  })

  it('disables every key while in flight and cannot send a second request', async () => {
    const user = userEvent.setup()
    let settle: ((value: unknown) => void) | undefined
    fetchMock.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          settle = resolve
        }),
    )
    render(<App />)

    await press(user, ['1', '2', 'add', '3', 'equals'])

    for (const label of KEY_LABELS) {
      expect(key(label), label).toBeDisabled()
    }
    expect(value()).toHaveTextContent('3')

    settle?.({ ok: true, status: 200, json: async () => ({ result: '15' }) })
    await waitFor(() => expect(key('1')).toBeEnabled())
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(value()).toHaveTextContent('15')
  })

  it('accepts keyboard input', async () => {
    const user = userEvent.setup()
    succeed('3')
    render(<App />)

    await user.keyboard('1+2{Enter}')

    await waitFor(() => expect(value()).toHaveTextContent('3'))
    expect(sentBody(0)).toEqual({ operation: 'add', operands: ['1', '2'] })
  })

  it('retries a failed square root on equals', async () => {
    const user = userEvent.setup()
    refuse(422, 'negative_sqrt', 'square root of the negative number -4 is undefined')
    succeed('2')
    render(<App />)

    await press(user, ['4', 'change sign', 'square root'])
    await waitFor(() => expect(value()).toHaveAttribute('data-error', 'true'))
    expect(sentBody(0)).toEqual({ operation: 'sqrt', operands: ['-4'] })

    await user.click(key('equals'))
    await waitFor(() => expect(value()).toHaveTextContent('2'))
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(sentBody(1)).toEqual({ operation: 'sqrt', operands: ['-4'] })
  })

  it('sends equals after typing even when a key has keyboard focus', async () => {
    const user = userEvent.setup()
    succeed('3')
    render(<App />)

    await user.tab()
    expect(document.activeElement?.tagName).toBe('BUTTON')

    await user.keyboard('1+2{Enter}')
    await waitFor(() => expect(value()).toHaveTextContent('3'))
    expect(sentBody(0)).toEqual({ operation: 'add', operands: ['1', '2'] })
  })

  it('lets a focused key activate on Enter instead of sending equals', async () => {
    const user = userEvent.setup()
    render(<App />)

    await user.click(key('1'))
    key('2').focus()
    await user.keyboard('{Enter}')

    expect(value()).toHaveTextContent('12')
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('does not clear the calculation on a browser shortcut', async () => {
    const user = userEvent.setup()
    render(<App />)

    await press(user, ['1', '2', 'add', '3'])
    await user.keyboard('{Meta>}c{/Meta}')

    expect(value()).toHaveTextContent('3')
    expect(screen.getByTestId('expression')).toHaveTextContent('12 +')
  })

  it('shows a long decimal result unaltered and scaled to fit', async () => {
    const user = userEvent.setup()
    const long = '246913578024691357802'
    succeed(long)
    render(<App />)

    await press(user, ['1', 'multiply', '2', 'equals'])

    await waitFor(() => expect(value()).toHaveTextContent(long))
    expect(value()).toHaveAttribute('data-size', 'long')
    expect(value()).toHaveAttribute('aria-live', 'polite')
  })
})
