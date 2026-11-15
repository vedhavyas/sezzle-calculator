# Calculator

A full-stack calculator: a Go REST service that does exact decimal arithmetic, and a React + TypeScript frontend that sends every operation to it.

```
POST /api/v1/calculate
{"operation": "add", "operands": ["0.1", "0.2"]}

200 {"result": "0.3"}
```

The addend result is exact, not a float. Everything in this repo follows from that decision.

## Run it

### Docker (one image, one port)

```sh
docker build -t sezzle-calculator .
docker run -p 8080:8080 sezzle-calculator
```

Open <http://localhost:8080>. The same process serves the API and the built frontend.

### Locally

Backend, Go 1.23 or newer:

```sh
cd backend
go run ./cmd/server            # listens on :8080
```

Frontend, Node 20 or newer:

```sh
cd frontend
npm install
npm run dev                    # http://localhost:5173, proxies /api to :8080
```

The dev server proxies `/api` to the Go service, so there is no CORS configuration anywhere.

## API

One endpoint. Requests and responses are JSON.

```http
POST /api/v1/calculate
Content-Type: application/json
```

| Operation | Operands | Result |
|---|---|---|
| `add` | 2 | `a + b` |
| `subtract` | 2 | `a - b` |
| `multiply` | 2 | `a * b` |
| `divide` | 2 | `a / b`, rounded to 16 decimal places |
| `power` | 2 | `a ^ b`, integer or fractional |
| `sqrt` | 1 | `√a` |
| `percent` | 2 | `a % of b`, that is `a * b / 100` |

Operands may be JSON strings or JSON numbers; both are parsed as exact decimals. Results are always **strings**, so the exact value survives the wire instead of being rounded into a float. Requests need no `Content-Type` header (it is not enforced), but the body must be JSON.

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
  -d '{"operation":"power","operands":["2","10"]}'
# {"result":"1024"}

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

`400` means the request could not be read. `422` means it was read and refused.

| Case | Status | Code |
|---|---|---|
| Body is not JSON, has an unknown field, or has trailing data | 400 | `invalid_json` |
| An operand cannot be parsed as a decimal | 400 | `invalid_operand` |
| An operand is longer than 100 characters | 422 | `operand_too_long` |
| Operation missing or unknown | 422 | `unknown_operation` |
| Wrong operand count for the operation | 422 | `invalid_operand_count` |
| Divisor is zero, or `0` raised to a negative power | 422 | `division_by_zero` |
| Square root of a negative number | 422 | `negative_sqrt` |
| Negative base with a non-integer exponent | 422 | `undefined_power` |
| Exponent magnitude over 1000 | 422 | `exponent_out_of_range` |
| A power whose result would exceed 10,000 digits | 422 | `result_too_large` |
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
# {"error":{"code":"undefined_power","message":"-8 raised to the non-integer exponent 0.5 is not a real number"}}

curl -s -X POST localhost:8080/api/v1/calculate \
  -d '{"operation":"sqrtt","operands":["1"]}'
# {"error":{"code":"unknown_operation","message":"unknown operation \"sqrtt\"; supported: add, divide, multiply, percent, power, sqrt, subtract"}}
```

## Tests

```sh
cd backend
go vet ./...
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -1

cd ../frontend
npx tsc -b
npm test -- --run --coverage
```

`tsc -b` builds the project references in `tsconfig.json`; `tsc --noEmit` at the root compiles no files, because the root config only holds references.

Backend coverage is over `internal/...`; the `cmd/server` entry point is wiring and is not counted. Frontend coverage is over `src/`, excluding the mount file and test setup, and the run fails below the configured thresholds.

| Layer | Statement coverage |
|---|---|
| Backend `internal/...` | 100% |
| Frontend `src/` | 94% |

The tests are table-driven on both sides. They assert values and error **codes**, not merely that something failed: `0.1 + 0.2` is `0.3`, `1 / 3` is exactly `0.3333333333333333`, a zero divisor returns `division_by_zero` rather than a panic, and `0^0` is `1`.

## Design decisions

**Exact decimals, not floats.** `float64` gives `0.30000000000000004` for `0.1 + 0.2`, which is wrong for a calculator and unusable for money. Arithmetic runs on `github.com/shopspring/decimal`, results are returned as strings, and operand text is never decoded through a float on the way in. Division and percentage round to 16 decimal places; addition, subtraction and multiplication are exact.

**One endpoint, an operation table, exact arity.** `POST /calculate` with `{operation, operands}` keeps one validation path and lets the operation table own how many operands each operation takes. Seven routes would have meant seven copies of the same handler for no gain in clarity.

**Proper 4xx codes.** 400 for a request that cannot be read, 422 for one the calculator refuses, 404 for an unknown path, 405 with `Allow` for the wrong method, 413 for an oversized body. 500 is reserved for a bug; a panic is recovered and turned into the envelope so the client never sees a dropped connection.

**The library's edges are handled explicitly.** `decimal.Pow` returns `0` with no error for `0^0`, `0^-1`, and a negative base with a fractional exponent, and `Div`/`DivRound` panic on a zero divisor. Each is validated before the library is called, and each has a test that would fail if the guard were removed.

**The frontend never does arithmetic.** Every result comes from the API, including chained operations. A local fast path would duplicate the rules and let the two implementations drift.

**Sequential evaluation.** The calculator evaluates left to right, one operation per `=`, with no operator precedence: `2 + 3 × 4` is `20`. A proper calculator expression parser is a different program, and the brief asks for arithmetic operations, not an expression language.

**One Docker image.** A three-stage build compiles the frontend and the Go binary and copies both into a small Alpine image. The Go process serves the API and the built frontend on one port, so there is no proxy and no CORS.

**No framework on the backend.** `net/http` with method patterns (Go 1.22+) and `log/slog` cover everything a seven-operation service needs. The one dependency is the decimal library.

## Layout

```
backend/
  cmd/server/            flags, wiring, static serving
  internal/calculator/   operations, exact arithmetic, typed errors
  internal/httpapi/      routing, decoding, status mapping, SPA handler
frontend/
  src/                   reducer, keypad, display, API client, styles
Dockerfile               one image: frontend build, Go build, Alpine runtime
specs/                   the contract each side implements
PROMPTS.md               the prompts used while building this
```

## Specs

`specs/` holds the contract this code implements: `overview.md` (system and API), `backend.md`, `frontend.md`. They are written to be verified against the code line by line.

## Assumptions and limits

- Operands are capped at 100 characters, exponents at a magnitude of 1000, and a power result at 10,000 digits. The last one matters: `1e1000 ^ 99.5` would otherwise occupy a core for twenty seconds and answer with a 100,000-digit number.
- Fractional powers and square roots are approximations, rounded to at most 16 decimal places. Addition, subtraction, multiplication and terminating division are exact.
- `0^0` is `1` by definition here.
- No history, no persistence, no accounts: one request in, one result out.
