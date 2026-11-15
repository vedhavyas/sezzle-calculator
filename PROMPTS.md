# Prompts used

The brief invited AI tooling and asked for the prompts used (`Instructions`, item 5). They are below in the order they were used, with what came of each, including the ones that produced something wrong and the correction that fixed it.

## Planning

**"Read the assessment, open the submission form in a browser, and write a deep-dive note before any code: requirements map, every ambiguity that has to be decided, risks, and a plan inside the 2-4 hour guidance."**

Produced a first plan with 19 numbered decisions. Correction on review: the form had been assumed to take a repository URL. The live page has no URL field at all, and its submit button stays disabled until a file is attached, so the repository link has to go in the note box and the submission needs at least one attached file. The plan changed accordingly.

**"Check whether they actually asked for a prompt log, and prove it from the source rather than from memory."**

It does: instruction 5, "Share any prompts that you used in your work", separate from instruction 1, "Use any AI tooling you would like". Proved by comparing the email text with the live page text: 42 normalised lines on each side, the only difference being the page's typo "perferred". Two negative controls were run first to show the comparison could fail.

**"For the number type, list every option, check that the library is maintained, and recommend one."**

Chose decimal over `float64`, because `0.1 + 0.2` must be `0.3` in a calculator and a payments context makes fixed-point arithmetic the right default. Rejected, each for a stated reason: `govalues/decimal` (19-digit ceiling), `cockroachdb/apd` (correct, but more ceremony than a seven-operation service earns), `math/big.Float` (binary, so `0.1` is still not exact), `int64` minor units (needs a scale the operands never declare) and a hand-rolled fixed point (the formatter is where a quiet bug hides). `shopspring/decimal` is MIT, 7.5k stars, released 18 Oct 2026.

**"Grill the plan one decision at a time. No softening."**

Decisions that changed as a result:

- One endpoint taking `{operation, operands}` instead of a route per operation, with arity owned by the operation table.
- Results as strings for every value, not numbers for integers and strings only where needed. A mixed type would force every consumer to branch, and a JSON number still loses precision in `JSON.parse`.
- Percentage defined as `a % of b`, not `x / 100`.
- Fractional exponents allowed as well as integer ones.
- Every optional operation included: power, square root and percentage.
- 4xx codes chosen per case rather than a blanket 400: 400 unreadable, 422 refused, 404, 405 with `Allow`, 413.
- One Docker image instead of two with compose.
- Private repository first, everything through pull requests, no squash merges, history cleaned before review rather than after.
- The expression parser deferred behind every in-scope item.

## Backend

**"Implement the calculator package with exact decimals. Validate every domain rule before calling the library, and table-test it."**

Probing the library before relying on it paid off. `decimal.Pow` returns `0` with no error for `0^0`, `0^-1` and `(-8)^0.5`, and `Div`/`DivRound` panic on a zero divisor. That meant three explicit domain checks on power and a zero-divisor guard before division. Each guard was mutation-verified: removed, watched the matching test fail, restored.

**"Add the HTTP layer: one route, one error envelope, a status per case, 404 and 405 in the envelope, body and operand limits."**

Two failures on the first run, both in my tests rather than the code: an oversized body of `x` characters failed JSON parsing before the size limit could fire, and a traversal path was being redirected by the router before the handler saw it. Both tests were rewritten to assert the property that actually matters.

**"Verify the Go version floor with a real toolchain instead of reading about it."**

Built the module with `GOTOOLCHAIN=go1.23.12`: method-plus-path routing, `MaxBytesError`, `fs.FS`, `FileServerFS` and `slog` all compile and run there, so the `go 1.23` directive in `go.mod` is true rather than aspirational.

## Frontend

**"Build the calculator UI over the API. No arithmetic in the browser, one request per equals, and the display shows exactly what the server sent."**

Working, and tested, but the first layout was wrong.

**"This does not look good. The layout on desktop is squished. Look at existing calculator apps for influence and make it presentable."**

The cause was structural: the layout container was a flex item sized to its content, so the card never reached its maximum width and sat at 417px in a 1496px window. Fixing the container gave the card its intended width. Two further defects were found while checking the result: a missing asset was answered with `index.html` (so a browser would parse HTML as a script), and a 21-digit exact result wrapped mid-number on a 360px screen until the display began scaling by length.

**"Curl the running app rather than trusting the tests."**

Drove the real page: `12 + 3` → `15`, `0.1 + 0.2` → `0.3`, `10 ÷ 0` → the server's message with the display still usable, then recovery by pressing a digit. Keyboard verified separately: `2 ^ 8` → `256`, `15 % 200` → `30`, `7 * 6` → `42`, backspace and Escape.

## Review and packaging

**"Run a review pass over the change with parallel reviewers on different axes, then fix what is blocking."**

Covered the arithmetic, the error paths, the transport, the specification against the code, the React state machine and both test suites. Findings and their disposition are listed in the pull request discussion; blocking findings were fixed, and the rest are recorded rather than silently dropped.

**"Build the image and exercise it, not just the binary."**

`docker build` and `docker run`, then the API and the UI through the container on one port, which is the deployment the README describes.
