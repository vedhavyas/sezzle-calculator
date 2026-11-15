# Specification - frontend

The React calculator over the API in `backend.md`. Vite, React 19, TypeScript strict (`"strict": true` in `tsconfig.app.json`). Vitest and React Testing Library. Plain CSS. No UI library, no state library, no router.

## Files

```
frontend/
  index.html
  vite.config.ts          dev proxy /api -> :8080, vitest config, coverage thresholds
  tsconfig.json           project references only; type-check with `tsc -b`
  tsconfig.app.json       "strict": true, includes src
  src/
    main.tsx              mount, font imports
    App.tsx               reducer, the one API call, keyboard listener
    calculator.ts         State, Action, initialState, reducer, visibleValue
    operations.ts         Operation type, expression wording
    api.ts                calculate(), ApiError
    Display.tsx           expression line, result line, error styling, size scaling
    Keypad.tsx            the key table and buttons
    keyboard.ts           key -> action mapping
    styles.css            design tokens, layout, mobile rules
```

## State

```ts
type Phase = 'idle' | 'entering' | 'calculating' | 'result' | 'error';

type State = {
  entry: string;              // digits being typed, '' when none
  accumulator: string | null; // left operand, or the carried result
  pending: Operation | null;  // operator waiting for its right operand
  queued: Operation | null;   // operator pressed while a chain is being evaluated
  phase: Phase;
  result: string | null;      // last server result, shown verbatim
  error: string | null;       // last server message
  expression: string;         // the line above the result
  request: CalculationRequest | null; // set when a request must be sent
};
```

The reducer never computes a result. Where an arithmetic answer would be needed, it sets `request` and the phase becomes `calculating`.

| Action | Effect |
|---|---|
| `digit` | appends; a leading `0` or `-0` is replaced rather than extended; from `result` or `error`, starts a new calculation |
| `dot` | one only; `''` becomes `0.`, `-` becomes `-0.` |
| `sign` | toggles a leading `-` on a non-empty entry; ignored in the result and error phases, where the entry is not visible |
| `backspace` | drops one character; a resulting bare `-` becomes `''`; from `result` or `error`, resets |
| `clear` | back to the initial state |
| `operator` | with no operand ready, stores the entry (or carried result) and the operator; with a full pending operation, sends it and queues the new operator |
| `equals` | with an operator and both operands present, sends the request; otherwise no-op |
| `sqrt` | sends `sqrt` of the entry, or of the carried result when no entry is being typed; `0` when neither exists |
| `requestSucceeded` | if an operator is queued, carries the result forward and makes the queued operator pending; otherwise phase `result` |
| `requestFailed` | phase `error`, shows the message, keeps operands so `equals` retries, drops any queued operator |

Left-to-right evaluation is the point of `queued`: `2 + 3 × 4 =` sends `add(2,3)`, then `multiply(5,4)`, and displays `20`. The entry and accumulator are stored as strings and never parsed, so the browser cannot introduce a float.

Input is ignored while `calculating`, so a second `=` cannot race the first.

`visibleValue(state)` returns what the display shows: the error message in the error phase, the server result in the result phase, the entry while typing, otherwise the accumulator (or `0`).

## API client

```ts
calculate(operation: Operation, operands: string[]): Promise<string>
```

- `POST ${VITE_API_BASE ?? '/api/v1'}/calculate` with `{operation, operands}` and `Content-Type: application/json`.
- 200 → `result` must be a string, else `ApiError('…', 'invalid_response', 200)`.
- Non-200 → `ApiError(message, code, status)` from the envelope; a body that does not parse still throws.
- Network failure → `ApiError('Could not reach the calculator service.', 'network_error', 0)`.

Operands are always sent as strings, so no arithmetic happens in the browser and no value is reformatted.

## Rendering

- Expression line: the pending operation (`12 +`, `15% of`, `√16`), or after a success, the completed expression with ` =` appended in one place, so every path ends the same way.
- Result line: the server's string verbatim, or the server's error message. Error codes are never shown.
- No thousands separators, no rounding, no localisation.
- The result scales by length (`short`, `medium`, `long` data attributes) so a 21-digit exact result stays on one line rather than wrapping mid-number.
- While a request is in flight every key is disabled and the display keeps the number being operated on. There is no spinner and no layout shift.

## Keyboard

| Key | Action |
|---|---|
| `0`-`9` | digit |
| `.` | decimal point |
| `+` `-` `*` `/` | add, subtract, multiply, divide |
| `^` | power |
| `%` | percent |
| `Enter`, `=` | equals |
| `Escape` | clear |
| `Backspace` | one digit |

Handled on `window`. The handler ignores anything pressed with Ctrl, Meta or Alt, so browser shortcuts such as Cmd+C are never intercepted. It also leaves `Enter` to a focused keypad button, because that is how a keyboard user activates a button; a pointer click on a key drops focus so the next `Enter` still means equals. Every handled key has its default action suppressed, and unmapped keys are ignored.

## Layout and accessibility

- Mobile-first: one card, 12-24px page padding, full width up to 520px, centred on wider screens.
- Keypad: four columns; `=` spans two rows, `0` spans two columns; every key at least 56px tall (44px minimum target).
- Two bundled typefaces: IBM Plex Sans for the interface, IBM Plex Mono for every figure so digits align.
- Real `<button type="button">` elements with an `aria-label` wherever the glyph is ambiguous (`backspace`, `sign`, `square root`, `power`, `percent of`, `decimal point`).
- `aria-live="polite"` and `aria-atomic` on the result line, so a screen reader announces the outcome and the error.
- Visible focus ring via `:focus-visible`; no `outline: none`.
- `prefers-reduced-motion` removes the key transitions.
- Colour pairs meet 4.5:1 for text.

## Tests

| File | Covers |
|---|---|
| `calculator.test.ts` | digits, leading-zero and negative canonicalisation, one dot, sign, backspace never leaving a bare minus, clear from every phase, operator replacement, left-to-right chaining, one request per equals, no-op equals, sqrt from entry, empty entry and after an operator, the number staying visible during sqrt, percent as `a % of b`, every key ignored in flight, result carried forward, digit after result starting over, retry after failure, no operator mutation in the error phase, a failed chain not carrying the queued operator, expression wording, `visibleValue` |
| `api.test.ts` | request shape and body, success, server error envelope, non-JSON error body, non-string result, network failure |
| `App.test.tsx` | one fetch per `=`, result rendered, the operation sent for each key, chaining across two requests, refusal rendered with the error attribute and the code hidden, recovery by clear and by digit, retry by equals, network failure, every key disabled in flight with the number still shown, keyboard input, Enter on a focused key, Cmd+C not clearing, long result unaltered and scaled |
| `keyboard.test.ts` | every digit, every operator, `Enter`/`=`, `Escape`, `Backspace`, and unused keys including `c` and `C` returning null |

## How to verify

```sh
cd frontend
npm ci
npx tsc -b
npm test -- --run
npm test -- --run --coverage
npm run build
```

`npm test -- --run` runs the suite once; `npm test` alone starts watch mode.

| Rule | Check |
|---|---|
| The browser never computes | `api.test.ts` asserts the request body; `App.test.tsx` asserts one fetch per `=`; no arithmetic exists outside `api.ts` |
| One request per `=` | `App.test.tsx` "sends one request per equals…" |
| Left to right | `calculator.test.ts` "evaluates left to right as you go", `App.test.tsx` "evaluates left to right across requests" |
| Every key sends the right operation | `App.test.tsx` "sends <operation> for the matching key" |
| Keys disabled in flight | `App.test.tsx` "disables every key while in flight…" |
| Server error is shown and recoverable | `App.test.tsx` "renders a refusal…", "retries the same operands…" |
| Result is displayed verbatim | `App.test.tsx` "shows a long decimal result unaltered…" |
| Keyboard map and browser shortcuts | `keyboard.test.ts`, `App.test.tsx` "does not clear the calculation on a browser shortcut" |
