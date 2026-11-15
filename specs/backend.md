# Specification - backend

The Go service behind `POST /api/v1/calculate`. Go 1.23 or newer. One dependency: `github.com/shopspring/decimal` v1.6.0.

## Packages

```
backend/
  cmd/server/main.go        flags, listener, graceful shutdown
  internal/calculator/      operations, exact arithmetic, typed errors
  internal/httpapi/         router, decoding, status mapping, static serving, middleware
```

The domain package knows nothing about HTTP. The transport package knows nothing about arithmetic. They share one vocabulary: stable error codes.

Tests sit beside each package: `internal/calculator/calculator_test.go` and `internal/httpapi/httpapi_test.go`.

## Domain

```go
type Operation string             // add, subtract, multiply, divide, power, sqrt, percent
func Calculate(op Operation, operands []decimal.Decimal) (decimal.Decimal, error)
func Names() []string             // sorted, for error messages
type Error struct { Code, Message string }
```

`Calculate` validates arity from the operation table, then applies the operation, then enforces the result-size limit for every operation. Every refusal is a `*calculator.Error` carrying a code.

| Operation | Arity | Rule |
|---|---|---|
| `add` | 2 | `a + b`, exact |
| `subtract` | 2 | `a - b`, exact |
| `multiply` | 2 | `a * b`, exact |
| `divide` | 2 | zero divisor refused; otherwise exact when the quotient terminates within 100 places, else at least 16 significant digits |
| `sqrt` | 1 | at least 16 significant digits; a negative value's error → `negative_sqrt` |
| `percent` | 2 | `a * b / 100` by shifting the point two places, exact |
| `power` | 2 | see below |

Power, in this order:

1. `a == 0 && b == 0` → `1`.
2. `a == 0 && b < 0` → `division_by_zero`.
3. `a < 0 && b` is not an integer → `undefined_power`.
4. Estimated result too large → `result_too_large`.
5. `a.PowWithPrecision(b, places)` where the places are derived from the result's order so the result keeps at least 16 significant digits; its error → `result_too_large`.

The first three are explicit because `decimal.Pow` returns `0` with no error for all of them. Step 4 exists because the exponent alone does not bound the work: `1e1000 ^ 99.5` takes about twenty seconds and answers with 100,000 digits, `1e10000 ^ 1000` returns a 10 MB body, and a value with a long fractional part raised to an integer power is tens of thousands of characters.

The estimate has two terms, and only the first applies to every exponent:

- The value's order times the exponent must not exceed 10,000. This bounds magnitude.
- For an **integer exponent only**, the operand's meaningful fractional digits (trailing zeros do not count) times the exponent must not exceed 10,000. A fractional exponent is rounded, so its long fractional part no longer expands, and this term is skipped.

That is why `2^10001` is accepted (3,011 digits), `1.9...9` with 98 nines raised to 1000 is refused, and the same value raised to `200.5` is accepted with a 61-digit result. A base of exactly ±1 has neither measure and is never refused, whatever the exponent. `PowWithPrecision` is used rather than `Pow` because the latter reads package globals and returns a zero on a precision error, which would reach the client as a real result.

The zero-divisor check is not defensive style: `decimal.Div` and `DivRound` panic on a zero divisor, and an unrecovered panic would drop the connection instead of answering. `SqrtRound` rather than `Sqrt` for the same reason as `PowWithPrecision`: `Sqrt` reads `decimal.DivisionPrecision`, a package variable, so a local setting would not govern it.

### Rounding

Exactness is bounded by each operation's working precision, not by whether the exact answer terminates. Addition, subtraction and multiplication are exact; percentage is exact; division is exact when the quotient terminates within 100 decimal places; square roots and powers with a fractional exponent are always rounded. Everything rounded keeps 16 **significant digits**, not 16 decimal places, which is what stops `sqrt("1e-33")` answering `0` while `power("1e-33","0.5")` answers a non-zero value. When the result has more than 16 digits before the point, the places clamp at zero: the fraction is dropped and the last integer digit rounds, so `divide(200000000000000000, 3)` is `66666666666666667` and a square root that would end in `.5` at 17 integer digits is rounded up. `placesFor(order)` converts a required number of significant digits into decimal places; `decimalOrder(value)` estimates `log10(|value|)` from the leading digits.

## Transport

### Routes

| Pattern | Handler |
|---|---|
| `GET /health` | `{"status":"ok"}` |
| `/health`, any other method | 405 `method_not_allowed` with `Allow: GET` |
| `POST /api/v1/calculate` | calculate |
| `/api/v1/calculate`, any other method | 405 `method_not_allowed` with `Allow: POST` |
| `/api`, `/api/` | 404 `not_found` |
| `/` | the SPA handler when `-web` is set, otherwise 404 `not_found` |

`net/http` pattern routing picks the most specific pattern, so the 405 and 404 patterns fire only when nothing else matches.

### Decoding

1. The body is read through `http.MaxBytesReader` with a 1 MiB limit. A `*http.MaxBytesError` is 413 `request_too_large`, whatever the body's shape. Reading first is what makes size behave as the first rule rather than as a side effect of parsing.
2. The bytes are decoded into `calculateRequest{Operation string; Operands []json.RawMessage}` with `DisallowUnknownFields()`. A decode failure is 400 `invalid_json`; the decoder's own error is logged, not shown to the client.
3. A second `Decode` must return `io.EOF`; anything else is trailing data, 400 `invalid_json`.
4. The operation name must be present and known → 422 `unknown_operation`, and its operand count must match the operation table → 422 `invalid_operand_count`. Both happen before any operand is parsed, so a body claiming hundreds of thousands of operands is refused without allocating for them.
5. Per operand, in order: it must be a JSON string or number (a `null` is named as `null`, anything else as a type problem), its decoded text must be at most 100 characters counted in runes, then it must parse with `decimal.NewFromString`. Failures are 400 `invalid_operand` or 422 `operand_too_long`, and messages name the operand's position.
6. `Calculate` applies the domain rules and the result-size limit.

`json.RawMessage` preserves each operand's exact text, so a JSON number never passes through `float64`. `Content-Type` is not enforced; the body must parse regardless.

### Responses

Success: 200 with `{"result": "<canonical decimal>"}`.

Failure: `{"error": {"code", "message"}}` with the status below. Every response is encoded into a buffer before the status is written, so a marshalling failure cannot produce a status with a half-written body. `writeCode` resolves the status from a map, so no handler picks one by hand.

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
| `result_too_large` | 422 |
| `not_found` | 404 |
| `method_not_allowed` | 405, with `Allow` |
| `request_too_large` | 413 |
| `internal_error` | 500 |

Content type is `application/json; charset=utf-8`. An unmapped code cannot reach the client as status 0: `writeCode` falls back to 500 and logs the programming error. Refusal messages name the operand or operation and truncate a very long operand rather than echoing it in full.

### Middleware

- Body limit: wraps `r.Body` in `http.MaxBytesReader`.
- Request logging: one line per request with method, path, status and duration, including when the handler panics.
- Panic recovery: logs the panic with a stack. If nothing has been written, answers 500 `internal_error`. If a response is already committed, aborts the connection rather than appending a second body. `http.ErrAbortHandler` is re-panicked untouched.

### Static serving

Active only when `-web` is set. Backed by `os.DirFS`:

Only `GET` and `HEAD` reach static serving; anything else is a 405 with `Allow: GET, HEAD`.

- `/index.html` and `/` are served as the app shell with `Cache-Control: no-cache`.
- A path that resolves to a directory is never listed: it is the app shell for a document request and a 404 otherwise.
- A path that resolves to a file is opened once, so a permission fault becomes a logged 500 rather than the file server's plain-text 403, then served by `http.FileServerFS`.
- A missing path is the app shell only when the request wants a document: `Sec-Fetch-Dest` of `document`, `iframe` or `frame`, or an `Accept` header that parses to `text/html` or `application/xhtml+xml` with a non-zero quality. A missing script, or a client that refuses HTML, gets a 404 envelope.
- Client-shaped failures (a path that is too long, runs through a file, or is otherwise invalid) are treated as missing, so they get the shell for a document request and a 404 otherwise, never a 500. Permission, I/O and loop errors are logged and answered 500.
- `..` cannot escape the directory: `fs.FS` rejects it, and the router redirects unclean paths before the handler runs. Symlinks inside the directory are followed, so a trusted build directory is assumed.

### Configuration

| Setting | Source | Default |
|---|---|---|
| Listen address | `-addr`, else `PORT` (a leading colon is tolerated), else `:8080` | `:8080` |
| Frontend directory | `-web` | empty, static serving off |

`ReadHeaderTimeout` 5s, `ReadTimeout` 15s, `WriteTimeout` 30s, `IdleTimeout` 60s. `SIGINT` and `SIGTERM` trigger a graceful shutdown with a 10 second budget.

## Tests

`internal/calculator`: one table covering every operation, exact decimals, repeating division, exact small quotients, every refusal with its code, arity for every operation, unknown and missing operation, the power rules in order, `0^0`, negative bases, the value at the 2^1000 and 2^1001 boundaries, and the result-size bounds; plus `TestDecimalOrder`, `TestPlacesFor`, `TestErrorCodeLiterals`, and a check that a refusal message stays short.

`internal/httpapi`: a table covering all seven operations, operands as strings and numbers, exponent notation, `+5`/`.5`/`5.`, the operand-length boundary at 100 and 101, every error code and status, message wording, content type, malformed and trailing JSON, unknown fields, oversized bodies in four shapes, `404`, `405` with `Allow` for several methods, `/health`, request logging, panic recovery before and after a partial write, and static serving across root, asset, deep link, missing asset, directory, traversal, unreadable file and unreadable index.

Both layers run offline; nothing binds a fixed port (`httptest` and `t.TempDir()` are used throughout).

## How to verify

```sh
cd backend
go vet ./...
go test ./... -count=1
go test ./internal/... -coverprofile=coverage.out && go tool cover -func=coverage.out | tail -1
go run ./cmd/server          # then curl the operations in ../README.md
```

| Rule | Check |
|---|---|
| Decimal exactness | `TestCalculate/add(0.1,0.2)` |
| Zero divisor is a 422, never a panic | `TestCalculate/divide(10,0)`, `TestCalculateEndpoint/divide_by_zero` |
| `0^0` is 1 | `TestCalculate/power(0,0)` |
| Non-real powers refused | `TestCalculate/power(-8,0.5)` |
| A power cannot blow up, and legitimate large results still work | `TestCalculate/power(1e1000,99.5)`, `TestPowerAcceptsResultsInsideTheDigitBudget`, `TestPowerAllowsALargeNegativeResult` |
| The operand count is checked before parsing | `TestOperandCountIsCappedBeforeParsing` |
| JSON numbers keep their digits | `TestJSONNumbersKeepTheirExactDigits` |
| Small results are not rounded to zero | `TestCalculate/sqrt(1e-33)`, `TestCalculate/divide(1,12345678901234567890)` |
| Caps enforced at the boundary | operand rows at 100 and 101 characters, `TestBodyLimit` |
| 405 carries `Allow` | `TestRouting` |
| Health and logging | `TestHealth`, `TestRequestLogging` |
| Static serving cannot escape, list a directory, or serve the shell for an asset | `TestStaticServing` |
| Every code has a status | `TestEveryDomainCodeHasAStatus`, `TestErrorCodeLiterals` |
