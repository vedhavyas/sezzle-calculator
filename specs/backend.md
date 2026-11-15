# Specification - backend

The Go service behind `POST /api/v1/calculate`. Go 1.23 or newer. One dependency: `github.com/shopspring/decimal` v1.6.0.

## Packages

```
backend/
  cmd/server/main.go        flags, listener, wiring, startup log
  internal/calculator/       operations, exact arithmetic, typed errors
  internal/httpapi/          router, decoding, status mapping, SPA handler, middleware
```

The domain package knows nothing about HTTP. The transport package knows nothing about arithmetic. They share one vocabulary: stable error codes.

Tests live beside each package: `internal/calculator/calculator_test.go` and `internal/httpapi/httpapi_test.go`.

## Domain

```go
type Operation string             // add, subtract, multiply, divide, power, sqrt, percent
func Calculate(op Operation, operands []decimal.Decimal) (decimal.Decimal, error)
func Names() []string             // sorted, for error messages
type Error struct { Code, Message string }
```

`Calculate` validates arity from the operation table, then applies the operation. Every refusal is a `*calculator.Error` carrying a code.

| Operation | Arity | Rule, in order |
|---|---|---|
| `add` | 2 | `a + b`, exact |
| `subtract` | 2 | `a - b`, exact |
| `multiply` | 2 | `a * b`, exact |
| `divide` | 2 | `b == 0` → `division_by_zero`; else `DivRound(b, 16)` |
| `sqrt` | 1 | `SqrtRound(16)`; a negative value's error → `negative_sqrt` |
| `percent` | 2 | `a * b / 100` via `DivRound(100, 16)` |
| `power` | 2 | see below |

Power, in this order:

1. `a == 0 && b == 0` → `1`.
2. `a == 0 && b < 0` → `division_by_zero`.
3. `a < 0 && b` is not an integer → `undefined_power`.
4. `abs(b) > 1000` → `exponent_out_of_range`.
5. estimated result magnitude over 10,000 → `result_too_large`.
6. `a.Pow(b)`.

The first three are explicit because `decimal.Pow` returns `0` with no error for all of them. The fifth exists because the exponent cap does not bound the work or the response: `1e1000 ^ 99.5` takes about twenty seconds and answers with a 100,000-digit number, and `1e10000 ^ 1000` returns a 10 MB body. The estimate is `abs(magnitude(a) * b)`, where `magnitude(x)` is the decimal order of `x`.

The zero-divisor check in `divide` is not defensive style: `decimal.Div` and `DivRound` panic on a zero divisor, and an unrecovered panic would drop the connection instead of answering.

`SqrtRound` rather than `Sqrt` is deliberate: `Sqrt` reads `decimal.DivisionPrecision`, a package-level variable, so the local constant would not govern it.

Rounding: everything that is not exact is rounded to at most 16 decimal places, half away from zero. Integer powers are exact; fractional powers and square roots are approximations.

## Transport

### Routes

| Pattern | Handler |
|---|---|
| `POST /api/v1/calculate` | calculate |
| `/api/v1/calculate`, any other method | 405 `method_not_allowed` with `Allow: POST` |
| `/api`, `/api/` | 404 `not_found` |
| `/` | the SPA handler when `-web` is set, otherwise 404 `not_found` |

Methods and paths are matched by `net/http`'s pattern routing; the more specific pattern wins, so the 405 and 404 patterns fire only when nothing else matches.

### Decoding

1. The body is read through `http.MaxBytesReader` with a 1 MiB limit. A `*http.MaxBytesError` is 413 `request_too_large`, whatever the body's shape. Reading first is what makes "size" behave like the first rule rather than a side effect of parsing.
2. The bytes are decoded into `calculateRequest{Operation string; Operands []json.RawMessage}` with `DisallowUnknownFields()`. A decode failure is 400 `invalid_json`, with the decoder's own error kept for the log rather than shown to the client.
3. A second `Decode` must return `io.EOF`; anything else is trailing data, 400 `invalid_json`.
4. Per operand, in order: the value must be a JSON string or number (a `null` is named as `null`, anything else is named as a type problem), then its decoded text must be at most 100 characters counted in runes, then it must parse with `decimal.NewFromString`. Failures are 400 `invalid_operand` or 422 `operand_too_long`, and messages name the operand's position.
5. The operation name must be known (or present) → 422 `unknown_operation`.
6. `Calculate` applies arity (422 `invalid_operand_count`) and the domain rules.

`json.RawMessage` preserves each operand's exact text, so a JSON number never passes through `float64`. `Content-Type` is not enforced; the body must parse regardless.

### Responses

Success: 200 with `{"result": "<canonical decimal>"}`.

Failure: `{"error": {"code", "message"}}` with the status below. Every response, success or failure, is encoded into a buffer before the status is written, so a marshalling failure cannot produce a status with a half-written body. `writeCode` resolves the status from a map, so no handler picks one by hand.

| Code | Status |
|---|---|
| `invalid_json` | 400 |
| `invalid_operand` | 400 |
| `operand_too_long` | 422 |
| `unknown_operation` | 422 |
| `invalid_operand_count` | 422 |
| `division_by_zero` | 422 |
| `negative_sqrt` | 422 |
| `undefined_power` | 422 |
| `exponent_out_of_range` | 422 |
| `result_too_large` | 422 |
| `not_found` | 404 |
| `method_not_allowed` | 405, with `Allow: POST` |
| `request_too_large` | 413 |
| `internal_error` | 500 |

Content type is `application/json; charset=utf-8`. An unmapped code cannot reach the client as status 0: `writeCode` falls back to 500 and logs the programming error.

### Middleware

- Body limit: wraps `r.Body` in `http.MaxBytesReader`.
- Panic recovery: logs the panic with a stack. If nothing has been written, answers 500 `internal_error`. If a response is already committed, it aborts the connection rather than appending a second body to it, because a second header cannot replace the first. `http.ErrAbortHandler` is re-panicked untouched.

No request logging, tracing or metrics. One `slog` line at startup, after the listener binds.

### Static serving

Active only when `-web` is set. Backed by `os.DirFS`:

- `/index.html` and `/` are served as the app shell with `Cache-Control: no-cache`.
- A path that resolves to a directory is served as the app shell, never as a listing.
- A path that resolves to a file is served by `http.FileServerFS`.
- A missing path is the app shell only when the request accepts HTML; a missing script or stylesheet is a 404 envelope. Serving `index.html` for a missing asset would make the browser parse HTML as a script.
- Any other filesystem error is logged and answered 500, so a permission fault is not reported as the client's 404.
- `..` cannot escape the directory: `fs.FS` rejects it, and the router redirects unclean paths before the handler runs.

### Configuration

| Setting | Source | Default |
|---|---|---|
| Listen address | `-addr`, else `PORT` (a leading colon is tolerated), else `:8080` | `:8080` |
| Frontend directory | `-web` | empty, static serving off |

The server sets `ReadHeaderTimeout`, `WriteTimeout` and `IdleTimeout`.

## Tests

`internal/calculator`: one table covering every operation, exact decimal results, repeating division, half-away-from-zero rounding at the 16th place including negatives, every refusal with its code, arity for every operation, unknown and missing operation, `0^0`, `0^-1`, negative bases, the exponent cap and the value at the cap, and the result-magnitude cap. `TestMagnitude` pins the estimator and `TestNames` the published operation list.

`internal/httpapi`: a table covering all seven operations, operands as strings and numbers, exponent notation, `+5`/`.5`/`5.`, the length cap at exactly 100 characters, every error code and status, message wording for the useful ones, response content type, malformed and trailing JSON, unknown fields, oversized bodies in four shapes, `404`, `405` with `Allow` for four methods, panic recovery before and after a partial write, static serving (root, asset, deep link with and without HTML in `Accept`, missing asset, directory, traversal, unreadable file, missing index), and a check that every domain code has a mapped status.

Both layers run offline; nothing binds a fixed port (`httptest` and `t.TempDir()` are used throughout).

## How to verify

```sh
cd backend
go vet ./...
go test ./... -count=1
go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out | tail -1
go run ./cmd/server          # then curl the operations in ../README.md
```

| Rule | Check |
|---|---|
| Decimal exactness | `TestCalculate/add(0.1,0.2)` |
| Zero divisor is a 422, never a panic | `TestCalculate/divide(10,0)`, `TestCalculateEndpoint/divide_by_zero` |
| `0^0` is 1 | `TestCalculate/power(0,0)` |
| Non-real powers refused | `TestCalculate/power(-8,0.5)` |
| A power cannot blow up | `TestCalculate/power(1e1000,99.5)`, `power(1e10000,1000)` |
| Caps enforced at the boundary | `TestCalculate/power(2,1001)`, operand rows at 100 and 101 characters, `TestBodyLimit` |
| 405 carries `Allow` | `TestRouting` |
| Static serving cannot escape, list a directory, or serve the shell for an asset | `TestStaticServing` |
| Every code has a status | `TestEveryDomainCodeHasAStatus` |
