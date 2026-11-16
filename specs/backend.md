# Specification - backend

The Go service behind `POST /api/v1/calculate`. Go 1.26.1 or newer. One dependency: `github.com/shopspring/decimal` v1.6.0.

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

`Calculate` validates arity from the operation table, then applies the operation. The result-size limit is enforced inside `power`, before the result is computed. Every domain refusal is a `*calculator.Error` carrying a code.

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
4. `|a| == 1` → `1`, or `-1` when `a` is negative and `b` is odd.
5. Estimated result too large → `result_too_large`.
6. `a.PowWithPrecision(b, places)` where the places are derived from the result's order so the result keeps at least 16 significant digits; its error → `result_too_large`.

The first three are explicit because `decimal.Pow` returns `0` with no error for all of them. Step 4 is explicit because an unnormalised `1.0` is coefficient 10 with exponent -1, and `PowWithPrecision` expands that coefficient by the exponent before rounding back to 1: at a ten-million exponent that is hundreds of milliseconds and a multi-megabyte integer, and at a larger one it does not finish. Step 5 exists because the exponent alone does not bound the work: `1e1000 ^ 99.5` takes about twenty seconds and answers with 100,000 digits, `1e10000 ^ 1000` returns a 10 MB body, and a value with a long fractional part raised to a positive integer power is tens of thousands of characters.

The estimate has two terms, and only the first applies to every exponent:

- The value's order times the exponent must not exceed the budget: 10,000 characters less a two-character margin for the estimate's rounding. This bounds magnitude.
- For a **positive integer exponent**, the exact decimal's length must stay inside the budget, because `PowWithPrecision` returns the exact power. A base below one contributes only its fractional digits plus the leading `0.`; a base at least one contributes the integer digits as well, and a non-integer result also counts the radix point between them. This is why `0.5^10000` is refused as a 10,002-character result and `0.999^5000000` never starts, `1.1^9602` is refused at 10,001 while `1.1^9601` is allowed at exactly 10,000, and `10^9999` is allowed at 10,000 while `10^10000` is refused at 10,001. A fractional exponent is rounded and a negative exponent is a reciprocal, so both skip this term.

That is why `2^10001` is accepted (3,011 digits), `1.9...9` with 98 nines raised to 1000 is refused, and the same value raised to `200.5` is accepted with a 61-digit result. `0.5^-10001`, `0.9^-10001` and `1.5^-10001` are accepted, because a reciprocal is bounded by the base's magnitude rather than its fractional part. `PowWithPrecision` is used rather than `Pow` because the latter reads package globals and returns a zero on a precision error, which would reach the client as a real result.

The zero-divisor check is not defensive style: `decimal.Div` and `DivRound` panic on a zero divisor, and an unrecovered panic would drop the connection instead of answering. `SqrtRound` rather than `Sqrt` for the same reason as `PowWithPrecision`: `Sqrt` reads `decimal.DivisionPrecision`, a package variable, so a local setting would not govern it.

### Rounding

Exactness is bounded by each operation's working precision, not by whether the exact answer terminates. Addition, subtraction and multiplication are exact; percentage is exact; division is exact when the quotient terminates within 100 decimal places, and a quotient that terminates only past that window is rounded like a repeating one (`1 / 2^332` keeps 16 significant digits); square roots and powers with a fractional exponent are always rounded. Everything rounded keeps 16 **significant digits**, not 16 decimal places, which is what stops `sqrt("1e-33")` answering `0` while `power("1e-33","0.5")` answers a non-zero value. When the result has more than 16 digits before the point, the places clamp at zero: the fraction is dropped and the last integer digit rounds, so `divide(200000000000000000, 3)` is `66666666666666667` and a square root that would end in `.5` at 17 integer digits is rounded up. `placesFor(order)` converts a required number of significant digits into decimal places; `decimalOrder(value)` estimates `log10(|value|)` from the leading digits.

## Transport

### Routes

| Pattern | Handler |
|---|---|
| `GET /health`, and `HEAD /health` | `{"status":"ok"}`; `HEAD` carries no body |
| `/health`, any other method | 405 `method_not_allowed` with `Allow: GET, HEAD` |
| `POST /api/v1/calculate` | calculate |
| `/api/v1/calculate`, any other method | 405 `method_not_allowed` with `Allow: POST` |
| `/` and every unmatched path | static handler when `-web` is set, otherwise 404 `not_found` |

`net/http` pattern routing picks the most specific pattern, so the 405 patterns fire only when nothing else matches. One catch-all handler answers everything else, so an unknown API path gets the same envelope as any other unknown path. A `cleanPaths` middleware answers a non-canonical path (`/../secret.txt`, `//health`) with the 404 envelope before the mux can redirect it.

### Decoding

1. The body is read through `http.MaxBytesReader` with a 1 MiB limit. A `*http.MaxBytesError` is 413 `request_too_large`, whatever the body's shape. Reading first is what makes size behave as the first rule rather than as a side effect of parsing.
2. A pre-scan of the top-level object refuses a repeated field with 400 `invalid_json`, comparing names case-insensitively because the standard decoder would silently keep the last value.
3. The bytes are decoded into `calculateRequest{Operation calculator.Operation; Operands []operand}` with `DisallowUnknownFields()`. A decode failure is 400 `invalid_json`; the decoder's own error is logged, not shown to the client.
4. A second `Decode` must return `io.EOF`; anything else is trailing data, 400 `invalid_json`.
5. The operand list is decoded by a streaming `operands` type that stops at the first element past `calculator.MaxOperands`, so a body naming thousands of them never allocates for them; it is 422 `invalid_operand_count`.
6. Each operand is an `operand`, a `decimal.Decimal` under a thin JSON wrapper. The wrapper refuses `null` with 400 `invalid_operand` and text longer than 100 runes with 422 `operand_too_long`; everything else is handed to `decimal.Decimal.UnmarshalJSON`, which reads a JSON string or number as its exact text. A parse failure is 400 `invalid_operand` with fixed text; the library error goes to the log.
7. `Calculate` checks the operation and arity, then applies the domain rules.

`decimal.Decimal.UnmarshalJSON` preserves each operand's exact text, so a JSON number never passes through `float64`. `Content-Type` is not enforced; the body must parse regardless.

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
- Request logging: one line per request with method, path, status and duration, including when the handler panics. The path, any decoder detail, a panic value and an unexpected handler error are capped at 200 characters, so an attacker-controlled field name cannot write an unbounded log line.
- Panic recovery: logs the panic with a stack. If nothing has been written, answers 500 `internal_error`. If a response is already committed, aborts the connection rather than appending a second body. `http.ErrAbortHandler` is re-panicked untouched.

### Static serving

Active only when `-web` is set. Backed by `os.DirFS`:

Only `GET` and `HEAD` reach static serving; anything else is a 405 with `Allow: GET, HEAD`.

- `/index.html` and `/` are served as the app shell with `Cache-Control: no-cache` and a content-hash `ETag`; a matching `If-None-Match` answers 304.
- Every other path must resolve to a file. A directory, a missing path, or a client-shaped failure (too long, runs through a file, invalid) is a 404 envelope. There are no client-side routes, so there is no shell fallback.
- A path that resolves to a file is opened once, so a permission fault becomes a logged 500 rather than the file server's plain-text 403, then served by `http.FileServerFS` with `Cache-Control: no-cache` and a size/modtime `ETag`.
- `Range` is dropped before the file is served: a multi-range header can turn a one-megabyte asset into tens of megabytes of response, and these are small static files.
- A symlink that resolves outside the canonical web root is refused with the 404 envelope, including one named `index.html`; `os.DirFS` alone is lexical.
- `..` cannot escape the directory: `fs.FS` rejects it, and the `cleanPaths` middleware answers a non-canonical path with the 404 envelope before the mux can redirect it.

### Configuration

| Setting | Source | Default |
|---|---|---|
| Listen address | `-addr`, else `PORT` (a leading colon is tolerated), else `:8080` | `:8080` |
| Frontend directory | `-web` | empty, static serving off |

`ReadHeaderTimeout` 5s, `ReadTimeout` 15s, `WriteTimeout` 30s, `IdleTimeout` 60s. `SIGINT` and `SIGTERM` trigger a graceful shutdown with a 10 second budget.

## Tests

`internal/calculator`: one table covering every operation, exact decimals, repeating division, exact small quotients, every refusal with its code, arity for every operation, unknown and missing operation, the power rules in order, `0^0`, negative bases, the value at the 2^1000 and 2^1001 boundaries, and the result-size bounds; plus `TestDecimalOrder`, `TestPlacesFor`, `TestFractionalDigits`, `TestMaxOperandsCoversEveryOperation`, `TestErrorCodeLiterals`, `TestNames`, `TestPowerOfOneDoesNotExpandTheCoefficient`, `TestNegativeExponentsAreNotRefusedForScale`, `TestQuotientTerminatingBeyondTheWindowIsRounded`, and a check that a refusal message stays short.

`internal/httpapi`: a table covering all seven operations, operands as strings and numbers, exponent notation, `+5`/`.5`/`5.`, the operand-length boundary at 100 and 101, every error code and status, message wording, content type, malformed and trailing JSON, unknown and repeated fields, oversized bodies in four shapes (including a body exactly at 1 MiB), more operands than any operation takes, `404`, `405` with `Allow` for several methods, `HEAD /health`, non-canonical paths, request logging and its 200-character cap, panic recovery before and after a partial write, the response recorder, code literals, and static serving across root, asset, directory, unknown path, traversal, symlink escape, multi-range, unreadable file and unreadable index.

`cmd/server`: `TestRunDrainsInFlightRequestOnCancel` cancels the context while a request is in the handler, asserts `run` has not returned, then releases the handler and asserts the client receives its response and `run` returns nil.

Both layers run offline; nothing binds a fixed port (`httptest`, `net.Listen(":0")` and `t.TempDir()` are used throughout).

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
| An unnormalised ±1 does not expand its coefficient | `TestPowerOfOneDoesNotExpandTheCoefficient` |
| A negative exponent is bounded by magnitude, not scale | `TestNegativeExponentsAreNotRefusedForScale` |
| A quotient terminating past 100 places is rounded | `TestQuotientTerminatingBeyondTheWindowIsRounded` |
| A list longer than any operation takes is refused | `TestOperandCountBeyondArityIsRefused` |
| A repeated top-level field is refused | `TestDecodingEdges/a_repeated_field` |
| A non-canonical path gets the envelope | `TestRouting` |
| A multi-range request is served whole | `TestStaticServing` |
| A symlink out of the web root is refused | `TestStaticServing` |
| The drain finishes an in-flight request | `TestRunDrainsInFlightRequestOnCancel` |
| JSON numbers keep their digits | `TestJSONNumbersKeepTheirExactDigits` |
| Small results are not rounded to zero | `TestCalculate/sqrt(1e-33)`, `TestCalculate/divide(1,12345678901234567890)` |
| Caps enforced at the boundary | operand rows at 100 and 101 characters, `TestBodyLimit` |
| 405 carries `Allow` | `TestRouting` |
| Health and logging | `TestHealth`, `TestRequestLogging` |
| Static serving cannot escape, list a directory, or serve an unknown path | `TestStaticServing` |
| Every code has a status | `TestEveryDomainCodeHasAStatus`, `TestErrorCodeLiterals` |
