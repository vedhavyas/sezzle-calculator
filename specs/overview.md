# Specification - overview

What this system is, what it must do, and the decisions behind it. `backend.md` and `frontend.md` are the two implementation contracts; this file is the authority when they disagree.

## Purpose

A calculator. A React frontend over a Go REST service. All arithmetic happens on the server; the browser never computes a result.

## What it implements

| Requirement | Where it lives |
|---|---|
| Addition, subtraction, multiplication, division | `backend/internal/calculator` |
| Exponentiation, square root, percentage | Same package |
| React frontend with TypeScript | `frontend/`, Vite + React 19 + strict TS |
| Input validation and error handling, both sides | Domain rules in Go, envelope rendering in React |
| Responsive layout | Mobile-first from 360px, compact at landscape heights |
| REST endpoints returning JSON | `POST /api/v1/calculate`, `GET /health` |
| Unit tests on both layers | Go table tests, Vitest + Testing Library |
| End-to-end tests | Playwright against the real service |
| Coverage reports | `go tool cover`, `vitest --coverage`; frontend thresholds in Vitest, backend gated in CI |
| Docker deployment | One multi-stage image serving API and frontend |
| Continuous integration | GitHub Actions: both suites, end to end, and the image |
| Documentation | `README.md`, this folder, `PROMPTS.md` |

## System shape

```
sezzle-calculator/
  backend/     Go 1.23+, one dependency (shopspring/decimal)
  frontend/    React 19, Vite, TypeScript strict
  Dockerfile   node build -> go build -> alpine runtime, non-root
  specs/       these documents
```

Two runtime shapes, one binary:

| Shape | Frontend served by | API served by |
|---|---|---|
| Development | Vite on :5273, proxying `/api` to :8080 | Go on :8080 |
| Image | the Go process, from `-web /app/web` | the same Go process |

No CORS middleware exists: development proxies, and the image is same-origin.

## API contract

```
POST /api/v1/calculate
```

Request:

```json
{"operation": "add", "operands": ["2", "3"]}
```

Success:

```json
{"result": "5"}
```

Failure:

```json
{"error": {"code": "division_by_zero", "message": "division by zero is undefined"}}
```

Rules:

- `result` is always a JSON **string** holding a canonical decimal: no exponent notation, no trailing zeros.
- Operands may be JSON strings or JSON numbers. Numbers are decoded as `json.Number` so their exact digits reach the decimal type; no value passes through `float64`.
- The body is capped at 1 MiB and each operand at 100 characters.
- The endpoint is versioned in the path (`/api/v1`).
- `GET /health` returns `{"status":"ok"}`.

Operations:

| Operation | Operands | Rule |
|---|---|---|
| `add` | 2 | exact |
| `subtract` | 2 | exact |
| `multiply` | 2 | exact |
| `divide` | 2 | zero divisor refused; exact when the quotient terminates within 100 places, else at least 16 significant digits |
| `power` | 2 | integer or fractional exponent; refused before computing when the result would pass 10,000 characters |
| `sqrt` | 1 | negative refused; at least 16 significant digits |
| `percent` | 2 | `a % of b`, exact (a shift of the decimal point) |

Statuses: `400` for a request that cannot be read, `422` for one the calculator refuses, `404` for an unknown path, `405` with `Allow` for the wrong method, `413` for an oversized body, `500` only for a bug. A panic is recovered into the envelope when nothing has been written, and aborts the connection when a response is already committed.

Refusals cover a zero divisor, a negative square root, a negative base with a fractional exponent, any result longer than 10,000 characters, an operand over 100 characters, and a wrong operand count. Each has its own code, listed in `backend.md`.

## Decisions

| Decision | Choice | Why |
|---|---|---|
| Number representation | `shopspring/decimal`, strings on the wire | `0.1 + 0.2` must be `0.3`. A float API would return `0.30000000000000004`, and hiding that in the UI would leave the contract wrong |
| Rounding | exact within each operation's working precision; otherwise 16 **significant** digits, rounding at the whole number when the integer part exceeds 16 digits | Division of a non-terminating quotient has to round somewhere; rounding by decimal places would answer `0` for a tiny non-zero result |
| Endpoint shape | One route with `{operation, operands}` and per-operation arity | One validation path, one error envelope; the operation table owns arity |
| Error codes | Per-case 4xx with a stable `code` | The client branches on one envelope and can tell a bad request from a refusal |
| Power bound | Result size, not exponent size | The exponent alone does not bound the work; `2^1001` is legitimate, `1e10000^1000` is not |
| Evaluation model | Sequential, left to right, one operation per `=` | Arithmetic operations, not an expression language. No operator precedence, documented |
| Where arithmetic runs | Server only | A client-side path would duplicate the rules and let the two drift |
| Frontend state | A pure reducer over a small phase union | Every transition is testable without a DOM; the component only dispatches and performs the one request |
| Theme | Automatic dark mode from the system preference | No stored preference, no toggle to get out of sync |
| Deployment | One image, Go serves the built frontend | One file, one port, no proxy, no CORS |
| Dependencies | One Go dependency; frontend has React plus test and design tooling | Nothing else earns its place at this size |

## Operational behaviour

- Every request is logged with method, path, status and duration.
- `SIGINT` and `SIGTERM` drain in-flight requests for up to ten seconds.
- Static serving falls back to the app shell only for a request that wants a document, so a missing script is a 404 rather than HTML the browser would try to run.
- Client-shaped path failures (too long, through a file) are 404s; permission and I/O faults are logged and answered 500.

## Out of scope

Expression parsing with precedence, calculation history, persistence, authentication, rate limiting, multi-tenancy, a hosted demo, localisation, and a manual theme toggle. Each is a deliberate exclusion.

## How to verify this document

1. `README.md` shows the setup and real API output.
2. `backend.md` and `frontend.md` each end with a verification list that maps a rule to a command or a test.
3. Run `go test ./...` in `backend/` and `npm test -- --run` in `frontend/`; the test names state the property. `npm run e2e` drives the real API and UI in a browser.
