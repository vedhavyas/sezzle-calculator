# Calculator

A full-stack calculator: a Go REST service that does exact decimal arithmetic, and a React + TypeScript frontend that sends every operation to it.

```
POST /api/v1/calculate
{"operation": "add", "operands": ["0.1", "0.2"]}

200 {"result": "0.3"}
```

The result is exact, not a float. Everything here follows from that one decision.

## Run it

### Docker (one image, one port)

```sh
docker build -t sezzle-calculator .
docker run -p 8080:8080 sezzle-calculator
```

Open <http://localhost:8080>. One process serves the API, the UI, and `/health`.

### Locally

Backend, Go 1.23 or newer:

```sh
cd backend
go run ./cmd/server            # listens on :8080
```

Frontend, Node 24, matching the image; the locked tooling supports 22.22.2+ on 22.x, 24.15+ on 24.x, or 26+:

```sh
cd frontend
npm install
npm run dev                    # http://localhost:5273, proxies /api to :8080
```

The dev server proxies `/api` to the Go service, so there is no CORS configuration anywhere.

## API

One endpoint, versioned in the path. Requests and responses are JSON.

```http
POST /api/v1/calculate
Content-Type: application/json
```

| Operation | Operands | Result |
|---|---|---|
| `add` | 2 | `a + b` |
| `subtract` | 2 | `a - b` |
| `multiply` | 2 | `a * b` |
| `divide` | 2 | `a / b` |
| `power` | 2 | `a ^ b`, integer or fractional |
| `sqrt` | 1 | `√a` |
| `percent` | 2 | `a % of b`, that is `a * b / 100` |

Operands may be JSON strings or JSON numbers; both are parsed as exact decimals, and a number's digits never pass through a float. Results are always **strings**, so the exact value survives the wire. `Content-Type` is not enforced, but the body must be JSON.

Addition, subtraction, multiplication and percentage are exact. Division is exact when the quotient terminates within 100 decimal places. Everything else (a repeating division, an irrational root, a fractional power) is rounded to 16 significant digits: `1 / 3` is `0.3333333333333333` and `√2` is `1.414213562373095`. When a result has more than 16 digits before the point, the rounding happens at the whole-number level, so the fraction is lost and the last integer digit can round: `200000000000000000 / 3` is `66666666666666667`, and the square root of `15241578753238836651425088777625362.25` is `123456789012345679`, whose exact value ends in `.5`. A quotient small enough that 16 decimal places would write it as zero is not rounded to zero: `1 / 12345678901234567890` keeps 16 significant digits: `0.00000000000000000008100000072900001`.

### Examples

All of these are real responses from a running service.

```sh
curl -s -X POST localhost:8080/api/v1/calculate \
  -d '{"operation":"add","operands":["0.1","0.2"]}'
# {"result":"0.3"}

curl -s -X POST localhost:8080/api/v1/calculate \
  -d '{"operation":"divide","operands":["1","3"]}'
# {"result":"0.3333333333333333"}

curl -s -X POST localhost:8080/api/v1/calculate \
  -d '{"operation":"power","operands":["2","1001"]}'
# {"result":"21430172143725346418968500981200036211228096234110672148875007767407021022498722449863967576313917162551893458351062936503742905713846280871969155149397149607869135549648461970842149210124742283755908364306092949967163882534797535118331087892154125829142392955373084335320859663305248773674411336138752"}

curl -s -X POST localhost:8080/api/v1/calculate \
  -d '{"operation":"sqrt","operands":["2"]}'
# {"result":"1.414213562373095"}

curl -s -X POST localhost:8080/api/v1/calculate \
  -d '{"operation":"percent","operands":["15","200"]}'
# {"result":"30"}

curl -s -X POST localhost:8080/api/v1/calculate \
  -d '{"operation":"multiply","operands":[123456789012345678901,2]}'
# {"result":"246913578024691357802"}
```

### Errors

Every failure uses one envelope:

```json
{"error": {"code": "division_by_zero", "message": "division by zero is undefined"}}
```

`400` means the request could not be read. `422` means it was read and the calculator refuses it.

| Case | Status | Code |
|---|---|---|
| Body is not JSON, has an unknown field, or has trailing data | 400 | `invalid_json` |
| An operand is `null`, empty, or not a number | 400 | `invalid_operand` |
| An operand is longer than 100 characters | 422 | `operand_too_long` |
| Operation missing or unknown | 422 | `unknown_operation` |
| Wrong operand count for the operation | 422 | `invalid_operand_count` |
| Divisor is zero, or `0` raised to a negative power | 422 | `division_by_zero` |
| Square root of a negative number | 422 | `negative_sqrt` |
| Negative base with a fractional exponent | 422 | `undefined_power` |
| A result longer than 10,000 characters (a power is refused before it is computed) | 422 | `result_too_large` |
| Unknown path | 404 | `not_found` |
| Method other than POST on the route | 405 | `method_not_allowed`, with `Allow: POST` |
| Body over 1 MiB | 413 | `request_too_large` |
| Anything else | 500 | `internal_error` |

```sh
curl -s -X POST localhost:8080/api/v1/calculate \
  -d '{"operation":"divide","operands":["10","0"]}'
# {"error":{"code":"division_by_zero","message":"division by zero is undefined"}}

curl -s -X POST localhost:8080/api/v1/calculate \
  -d '{"operation":"power","operands":["-8","0.5"]}'
# {"error":{"code":"undefined_power","message":"a negative base with the fractional exponent 0.5 is not supported"}}

curl -s -X POST localhost:8080/api/v1/calculate \
  -d '{"operation":"power","operands":["1e1000","99.5"]}'
# {"error":{"code":"result_too_large","message":"the result would exceed 10000 characters"}}
```

`result_too_large` is not pedantry: without it, `1e1000 ^ 99.5` occupies a core for about twenty seconds and answers with a 100,000-digit number, and a request under fifty bytes can ask for a ten-megabyte response.

## Health, logging and shutdown

```sh
curl -s localhost:8080/health
# {"status":"ok"}
```

Every request is logged with its method, path, status and duration. `SIGINT` or `SIGTERM` stops accepting connections and drains in-flight requests for up to ten seconds.

## Tests

```sh
cd backend
go vet ./...
go test ./... -count=1
go test ./internal/... -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -1

cd ../frontend
npx tsc -b
npm test -- --run --coverage
npx playwright install chromium   # once per machine
npm run e2e
```

`tsc -b` builds the project references in `tsconfig.json`; `tsc --noEmit` at the root compiles no files, because the root config only holds references.

| Layer | Tests | Coverage |
|---|---|---|
| Backend `internal/...` | table-driven, every operation and refusal | 98.0% statements |
| Frontend `src/` | 62 unit tests | 94.6% statements, 99.3% lines |
| End to end | 12 Playwright tests against the real API and UI | - |

The uncovered backend statements are five defensive branches the suite does not exercise: a decimal coefficient that fails to parse, a power error the size guard makes impossible, a scale that would go negative, one malformed-input path inside the decoder, and a file that disappears between stat and open.

The tests assert values and error **codes**, not merely that something failed: `0.1 + 0.2` is `0.3`, `1 / 3` is exactly `0.3333333333333333`, a zero divisor returns `division_by_zero` rather than a panic, `0^0` is `1`, and a failed square root retries itself rather than the operation underneath it.

The end-to-end suite starts both servers itself and drives the real UI in Chromium: a calculation through the API, a decimal that must stay exact, a server refusal and recovery, left-to-right chaining, keyboard input, a phone viewport with no horizontal scroll, four landscape sizes from 844x390 down to 568x320, the bundled fonts actually loading, and dark mode.

## Design decisions

**Exact decimals, not floats.** `float64` gives `0.30000000000000004` for `0.1 + 0.2`, which is wrong for a calculator and unusable for money. Arithmetic runs on `github.com/shopspring/decimal`, results are returned as strings, and no operand passes through a float on the way in.

**Exact within each operation's working precision.** Addition, subtraction and multiplication are exact, and so is percentage, because dividing by 100 only moves the decimal point. Division is exact when the quotient terminates within 100 decimal places, and otherwise rounded to 16 significant digits, so a tiny result like `1 / 12345678901234567890` is not rounded to zero. Square roots and fractional powers are rounded to 16 significant digits. When a result has more than 16 digits before the point, the rounding lands on the whole number: the fraction is dropped and the last integer digit can round, which is why a quotient like `200000000000000000 / 3` ends in `7` rather than `6`.

**One endpoint, an operation table, exact arity.** `POST /api/v1/calculate` with `{operation, operands}` keeps one validation path and lets the operation table own how many operands each operation takes. The operation and its operand count are checked before any operand is parsed, so a body claiming half a million operands costs nothing to refuse, and adding a three-operand operation later needs no new guard.

**Proper 4xx codes.** 400 for a request that cannot be read, 422 for one the calculator refuses, 404 for an unknown path, 405 with `Allow` for the wrong method, 413 for an oversized body. 500 is reserved for a bug: a panic is recovered into the envelope when nothing has been written yet, and when a response is already on the wire the connection is aborted rather than a second body appended to it.

**The library's edges are handled explicitly.** `decimal.Pow` returns `0` with no error for `0^0`, `0^-1`, and a negative base with a fractional exponent; `Div` and `DivRound` panic on a zero divisor. Each is validated before the library is called, and each has a test that fails if the guard is removed.

**The frontend never does arithmetic.** Every result comes from the API, including chained operations. Evaluation is left to right, one operation per `=`, with no operator precedence: `2 + 3 × 4` is `20`, sent as two requests. A local fast path would duplicate the rules and let the two implementations drift.

**One Docker image.** A three-stage build compiles the frontend and the Go binary and copies both into a small Alpine image, running as an unprivileged user. The Go process serves the API and the built frontend on one port, so there is no proxy and no CORS. The built frontend falls back to its shell only for a request that wants a document, so a missing script is still a 404 rather than HTML the browser would try to run.

**No framework on the backend.** `net/http` with method patterns (Go 1.22+) and `log/slog` cover everything this service needs. The one dependency is the decimal library.

**The UI is treated as production surface.** Mobile-first from 360px, a layout that fits a landscape phone without scrolling, automatic dark mode from the system preference, keyboard operation, visible focus, and live regions so a screen reader announces results and errors.

## Layout

```
backend/
  cmd/server/            flags, listener, graceful shutdown
  internal/calculator/   operations, exact arithmetic, typed errors
  internal/httpapi/      routing, decoding, status mapping, static serving, middleware
frontend/
  src/                   reducer, keypad, display, API client, styles
  e2e/                   Playwright specs
.github/workflows/       CI: both suites, end to end, and the image
Dockerfile               one image: frontend build, Go build, Alpine runtime
specs/                   the contract each side implements
PROMPTS.md               the prompts used while building this
```

## Specs

`specs/` holds the contract this code implements: `overview.md` (system and API), `backend.md`, `frontend.md`. They are written to be checked against the code line by line.

## Limits and assumptions

- A body is capped at 1 MiB, an operand at 100 characters, and any result at 10,000 characters; a sign or a decimal point counts toward that. A power is additionally refused before it is computed when the estimate says it would cross the line, so a doomed request never runs.
- `0^0` is `1` by definition here. A negative base with a fractional exponent is refused rather than answered with a complex number.
- No history, no persistence, no accounts: one request in, one result out.
- The demo is not hosted; `docker run` is the deployment.
