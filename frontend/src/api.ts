import type { Operation } from './operations'

const BASE_URL = import.meta.env.VITE_API_BASE ?? '/api/v1'

export class ApiError extends Error {
  readonly code: string
  readonly status: number

  constructor(message: string, code: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
  }
}

type ErrorEnvelope = { error?: { code?: string; message?: string } }

export async function calculate(operation: Operation, operands: string[], signal?: AbortSignal): Promise<string> {
  let response: Response
  try {
    response = await fetch(`${BASE_URL}/calculate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ operation, operands }),
      signal,
    })
  } catch (error) {
    // The caller's timer aborts with a TimeoutError; a cancel aborts with AbortError.
    if (error instanceof DOMException && error.name === 'TimeoutError') {
      throw new ApiError('The calculator service took too long to answer.', 'timeout', 0)
    }
    if (error instanceof DOMException && error.name === 'AbortError') {
      throw new ApiError('The calculation was cancelled.', 'cancelled', 0)
    }
    throw new ApiError('Could not reach the calculator service.', 'network_error', 0)
  }

  if (response.ok) {
    const body = (await response.json().catch(() => null)) as { result?: unknown } | null
    if (typeof body?.result !== 'string') {
      throw new ApiError('Unexpected response from the calculator service.', 'invalid_response', response.status)
    }
    return body.result
  }

  const envelope = (await response.json().catch(() => null)) as ErrorEnvelope | null
  const message = envelope?.error?.message
  const code = envelope?.error?.code
  throw new ApiError(
    typeof message === 'string' ? message : 'The calculator could not process that.',
    typeof code === 'string' ? code : 'invalid_response',
    response.status,
  )
}
