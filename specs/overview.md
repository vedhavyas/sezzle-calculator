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
| Responsive layout | Mobile-first CSS, works from 360px up |
| REST endpoints returning JSON | `POST /api/v1/calculate` |
| Unit tests on both layers | Go table tests, Vitest + Testing Library |
| Coverage reports | `go tool cover`, `vitest --coverage` |
| Docker deployment | One multi-stage image serving API and frontend |
| Documentation | `README.md`, this folder, `PROMPTS.md` |

## System shape

```
sezzle-calculator/
  backend/     Go 1.23+, one dependency (shopspring/decimal)
  frontend/    React 19, Vite, TypeScript strict
  Dockerfile   node build -> go build -> alpine runtime
  specs/       these documents
```

Two runtime shapes, one binary:

| Shape | Frontend served by | API served by |
|---|---|---|
| Development | Vite on :5173, proxying `/api` to :8080 | Go on :8080 |
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

Operations:

| Operation | Operands | Rule |
|---|---|---|
| `add` | 2 | exact |
| `subtract` | 2 | exact |
| `multiply` | 2 | exact |
| `divide` | 2 | zero divisor refused; otherwise rounded to 16 decimal places |
| `power` | 2 | integer or fractional exponent; see `backend.md` |
| `sqrt` | 1 | negative refused |
| `percent` | 2 | `a % of b`, rounded to 16 decimal places |

Statuses: `400` for a request that cannot be read, `422` for one the calculator refuses, `404` for an unknown path, `405` with `Allow` for the wrong method, `413` for an oversized body, `500` only for a bug (a panic is recovered into the envelope).

Refusals cover more than a zero divisor: a negative square root, a negative base with a non-integer exponent, an exponent beyond ±1000, a power whose result would exceed 10,000 digits, an operand over 100 characters, and a wrong operand count. Each has its own code, listed in `backend.md`.

## Decisions

| Decision | Choice | Why |
|---|---|---|
| Number representation | `shopspring/decimal`, strings on the wire | `0.1 + 0.2` must be `0.3`. A float API would return `0.30000000000000004`, and hiding that in the UI would leave the contract wrong |
| Endpoint shape | One route with `{operation, operands}` and per-operation arity | One validation path, one error envelope; the operation table owns arity, so the route does not encode it |
| Error codes | Per-case 4xx with a stable `code` | The client branches on one envelope and can tell a bad request from a refusal |
| Evaluation model | Sequential, left to right, one operation per `=` | The brief asks for arithmetic operations, not an expression language. No operator precedence, documented |
| Where arithmetic runs | Server only | A client-side path would duplicate the rules and let the two drift |
| Frontend state | A pure reducer over a small phase union | Every transition is testable without a DOM; the component only dispatches and performs the one request |
| Deployment | One image, Go serves the built frontend | One file, one port, no proxy, no CORS |
| Dependencies | One Go dependency; frontend has React plus test/design tooling | Nothing else earns its place at this size |

## Out of scope

Expression parsing with precedence, calculation history, persistence, authentication, rate limiting, a hosted demo, localisation, theming, end-to-end browser tests, and CI. Each is a deliberate exclusion, not an oversight.

## How to verify this document

1. `README.md` shows the setup and real API output.
2. `backend.md` and `frontend.md` each end with a verification list that maps a rule to a command or a test.
3. Run `go test ./...` in `backend/` and `npm test -- --run` in `frontend/`; the test names state the property.
