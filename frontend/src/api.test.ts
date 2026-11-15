import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, calculate } from './api'

const fetchMock = vi.fn()

function respond(body: unknown, status = 200) {
  fetchMock.mockResolvedValueOnce({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  })
}

async function captureError(): Promise<ApiError> {
  try {
    await calculate('add', ['1', '2'])
  } catch (error) {
    if (error instanceof ApiError) return error
    throw error
  }
  throw new Error('expected the call to fail')
}

beforeEach(() => {
  fetchMock.mockReset()
  vi.stubGlobal('fetch', fetchMock)
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('calculate', () => {
  it('posts the operation and operands as strings and returns the result', async () => {
    respond({ result: '3' })

    await expect(calculate('add', ['1', '2'])).resolves.toBe('3')

    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/calculate')
    expect(init.method).toBe('POST')
    expect(init.headers).toEqual({ 'Content-Type': 'application/json' })
    expect(init.body).toBe('{"operation":"add","operands":["1","2"]}')
  })

  it('surfaces the server code and message on a refusal', async () => {
    respond({ error: { code: 'division_by_zero', message: 'division by zero is undefined' } }, 422)

    const error = await captureError()
    expect(error.code).toBe('division_by_zero')
    expect(error.message).toBe('division by zero is undefined')
    expect(error.status).toBe(422)
  })

  it('falls back when the error body is not the envelope', async () => {
    fetchMock.mockResolvedValueOnce({
      ok: false,
      status: 502,
      json: async () => {
        throw new Error('not json')
      },
    })

    const error = await captureError()
    expect(error.code).toBe('invalid_response')
    expect(error.status).toBe(502)
  })

  it('rejects a success body without a string result', async () => {
    respond({ result: 3 })

    const error = await captureError()
    expect(error.code).toBe('invalid_response')
  })

  it('reports a timeout separately from an unreachable service', async () => {
    fetchMock.mockRejectedValueOnce(new DOMException('The operation timed out.', 'TimeoutError'))

    const error = await captureError()
    expect(error.code).toBe('timeout')
    expect(error.message).toContain('too long')
  })

  it('reports a user cancellation separately from a timeout', async () => {
    fetchMock.mockRejectedValueOnce(new DOMException('The operation was aborted.', 'AbortError'))

    const error = await captureError()
    expect(error.code).toBe('cancelled')
  })

  it('reports a network failure as such', async () => {
    fetchMock.mockRejectedValueOnce(new TypeError('Failed to fetch'))

    const error = await captureError()
    expect(error.code).toBe('network_error')
    expect(error.status).toBe(0)
  })
})
