# Specification - frontend

The React calculator over the API in `backend.md`. Vite, React 19, TypeScript strict (`"strict": true` in both app and node projects). Vitest and React Testing Library for unit tests, Playwright for end-to-end. Plain CSS. No UI library, no state library, no router.

## Files

```
frontend/
  index.html
  vite.config.ts          dev proxy /api -> :8080, vitest config scoped to src, coverage thresholds
  playwright.config.ts    starts the Go service and the dev server, drives Chromium
  tsconfig.json           project references only; type-check with `tsc -b`
  src/
    main.tsx              mount, font imports
    App.tsx               reducer, the one API call, keyboard listener
    calculator.ts         State, Action, initialState, reducer, visibleValue
    operations.ts         Operation type, expression wording
    api.ts                calculate(), ApiError
    Display.tsx           expression line, result line, error styling, size scaling
    Keypad.tsx            the key table and buttons
    keyboard.ts           key -> action mapping
    theme.ts              theme type, storage, document attribute, cycle order
    styles.css            design tokens, light and dark palettes, responsive rules
  e2e/
    calculator.spec.ts    fifteen browser tests against the real service
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
  failed: CalculationRequest | null;  // the request that failed, for `=` to retry
};
```

The reducer never computes a result. Where an arithmetic answer would be needed, it sets `request` and the phase becomes `calculating`.

| Action | Effect |
|---|---|
| `digit` | appends; a leading `0` or `-0` is replaced; from `result` or `error`, starts a new calculation |
| `dot` | one only; `''` becomes `0.` |
| `sign` | toggles a leading `-` on a non-zero entry; a settled result becomes a new entry with its sign flipped; a zero stays zero; ignored in the error phase |
| `backspace` | drops one character; a resulting bare `-` becomes `''`; from `result` or `error`, resets |
| `clear` | back to the initial state; ignored while a request is in flight |
| `cancel` | back to the initial state from any phase; the user aborting a request in flight |
| `operator` | with no operand ready, stores the entry (or the carried result) and the operator; in the result phase it continues from that result; with a full pending operation, sends it and queues the new operator; ignored in the error phase |
| `equals` | retries the failed request in the error phase; otherwise sends the pending operation when both operands exist |
| `sqrt` | sends `sqrt` of the entry, or of the carried result; `0` when neither exists; ignored in the error phase |
| `requestSucceeded` | if an operator is queued, carries the result forward and makes the queued operator pending; if the request was a `sqrt` inside a pending operation, keeps that operation and uses the root as its right operand; otherwise phase `result` |
| `requestFailed` | phase `error`, shows the message, keeps the failed request for `equals` to retry, drops any queued operator |

Left-to-right evaluation is the point of `queued`: `2 + 3 × 4 =` sends `add(2,3)`, then `multiply(5,4)`, and displays `20`. Retrying `failed` rather than re-deriving from `pending` is what makes a failed square root retry the square root, and it is why a failed request cannot silently turn into a different calculation.

Input is ignored while `calculating`, including `clear`, so a response can never land on a state the user cleared in the meantime.

`visibleValue(state)` returns what the display shows: the error message in the error phase, the server result in the result phase, the entry while typing, otherwise the accumulator (or `0`).

## API client

```ts
calculate(operation: Operation, operands: string[], signal?: AbortSignal): Promise<string>
```

- `POST ${VITE_API_BASE ?? '/api/v1'}/calculate` with `{operation, operands}` and `Content-Type: application/json`.
- 200 → `result` must be a string, else `ApiError` with code `invalid_response` and status 200.
- Non-200 → `ApiError(message, code, status)` from the envelope; a body that does not parse still throws.
- The caller passes an `AbortSignal`. The app arms a 10 second timer that aborts with a `TimeoutError`, which becomes `ApiError('The calculator service took too long to answer.', 'timeout', 0)`; Escape aborts with an `AbortError`, which becomes the `cancelled` code and is not shown as an error.
- Network failure → `ApiError('Could not reach the calculator service.', 'network_error', 0)`.

Operands are always sent as strings, so no arithmetic happens in the browser and no value is reformatted.

## Rendering

- Expression line: the pending operation (`12 +`, `15% of`, `√16`), or after a success the completed expression with ` =` appended in one place, so every path ends the same way.
- Result line: the server's string verbatim, or the server's error message. Error codes are never shown.
- No thousands separators, no rounding, no localisation.
- The result scales by length (`short`, `medium`, `long` data attributes) so a 21-digit exact result stays on one line rather than wrapping mid-number.
- A result longer than about two lines scrolls inside the display instead of growing the card and pushing the keypad off screen; the full value is in the element's `title`.
- While a request is in flight every key is disabled except the first, which becomes an enabled Cancel control with the accessible name `cancel request`; the container carries `aria-busy` and a visually hidden `role="status"` announces "Calculating". The display keeps the number being operated on. There is no spinner and no layout shift.
- A chain whose next request would carry an operand over 100 characters is refused in the reducer with "That number is too long to use in another operation." and no request is sent.
- In the error phase the keys the reducer ignores (operators, square root, sign) are disabled rather than left looking pressable; digits, clear, backspace and equals stay live.

## Keyboard

| Key | Action |
|---|---|
| `0`-`9` | digit |
| `.` | decimal point |
| `+` `-` `*` `/` | add, subtract, multiply, divide |
| `^` | power |
| `%` | percent |
| `Enter`, `=` | equals |
| `Escape` | clear, or cancel a request in flight |
| `Backspace` | one digit |
| `n`, `N` | change sign (`-` is subtract) |

Handled on `window`. `Escape` aborts a request in flight and resets, so a stalled calculation never leaves the keypad dead. The handler ignores anything pressed with Ctrl, Meta or Alt, so browser shortcuts such as Cmd+C are never intercepted. It leaves `Enter` to a focused keypad button, because that is how a keyboard user activates a button; once any other handled key is pressed the focused button is blurred, so typing `1+2` and then pressing Enter sends equals rather than re-activating whatever was tabbed to. Every handled key has its default action suppressed, and unmapped keys are ignored.

## Layout, theme and accessibility

- Mobile-first: one card, 12-24px page padding (safe-area aware), full width up to 520px, centred on wider screens.
- Keypad: four columns; `=` spans two rows, `0` spans two columns; keys are 56px tall on a phone, 72px on a desktop, and 44px in landscape. Below 360px of viewport height the keys compress to 38px and the expression line is hidden, because six rows at 44px cannot fit that height and scrolling would put `=` below the fold.
- Landscape phones (viewport height 440px or less) get a compact layout so the whole calculator fits without scrolling; the result line is capped at one line there and scrolls, so a long result cannot push the keypad off screen.
- One palette declared with the CSS `light-dark()` function, so light and dark values sit on the same line per token. `color-scheme` decides which side resolves: the system preference by default, or an explicit `data-theme` attribute.
- A `Theme: System` control above the card cycles System, Light and Dark. The choice is stored in `localStorage` under `calculator-theme`, applied to `<html>` by a pre-paint script in `index.html` (the key is duplicated there with a keep-in-sync comment), and reapplied by React on mount. System is the default and removes the attribute. The control is 36px tall and its change is announced through a visually hidden status. The theme bar is hidden below 620px of viewport height, and below 440px the landscape layout drops it entirely, so neither range gives up its no-scroll promise.
- Browser floor for `light-dark()`: Chrome 123+, Safari 17.5+, Firefox 120+.
- Two bundled typefaces: IBM Plex Sans for the interface, IBM Plex Mono for the display figures so digits align.
- Real `<button type="button">` elements with an `aria-label` wherever the glyph is ambiguous (`backspace`, `change sign`, `square root`, `power`, `percent of`, `decimal point`).
- `aria-live="polite"` and `aria-atomic` on the result line, so a screen reader announces the outcome and the error.
- Visible focus ring via `:focus-visible`; no `outline: none`.
- `prefers-reduced-motion` removes the key transitions.
- Colour pairs meet 4.5:1 for text in both themes. The keypad's visual boundary uses a `--control-edge` pair at 3.25:1 and 3.02:1 in light and 3.77:1 and 3.31:1 in dark against the card, and operator keys carry an accent border, so the controls meet WCAG 1.4.11.

## Tests

### Unit (Vitest, `src/**/*.test.ts(x)`)

| File | Covers |
|---|---|
| `calculator.test.ts` | digits, leading-zero and negative canonicalisation, one dot, sign, backspace never leaving a bare minus, clear from every phase that is not in flight, operator replacement, left-to-right chaining, one request per equals, no-op equals, sqrt from entry, empty entry and after an operator, the number staying visible during sqrt, percent as `a % of b`, every key ignored in flight including clear, result carried forward, digit after result starting over, retry after failure, retrying a failed sqrt rather than the hidden operation, no operator in the error phase, a failed chain not carrying the queued operator, expression wording, `visibleValue` |
| `api.test.ts` | request shape and body, success, server error envelope, non-JSON error body, non-string result, network failure |
| `App.test.tsx` | one fetch per `=`, result rendered, the operation sent for every key including subtract, chaining across two requests, refusal rendered with the error attribute and the code hidden, recovery by clear and by digit, retry by equals, retrying a failed sqrt, network failure, every key disabled in flight except Cancel with the number still shown, a cancel that ignores the late response, the 10-second timeout on fake timers, an over-long result refused locally, inert keys disabled in the error phase, keyboard input, equals after typing and after a pointer click, Enter on a focused key, Meta-modified keys ignored, Cmd+C not clearing, sign and backspace, long result unaltered and scaled, the theme control cycling and storing its choice, Enter after a theme click not re-toggling |
| `keyboard.test.ts` | every digit, every operator, `Enter`/`=`, `Escape`, `Backspace`, `n`/`N`, and unused keys including `c`, `C` and prototype names returning null |
| `theme.test.ts` | system default, a stored explicit theme, a stored invalid value ignored, storage failure falling back to system, the cycle order, the document attribute, and the labels |

### End to end (Playwright, `e2e/`)

The suite starts the Go service and the dev server itself, then drives Chromium: a calculation through the API, a decimal that must stay exact, a server refusal and recovery, left-to-right chaining, keyboard input, a 360px viewport with no horizontal scroll, four landscape sizes from 844x390 down to 568x320, a 100-digit expression that stays inside 844x390, a 102-digit result that stays inside a 360px phone, the bundled fonts loading rather than falling back, exact light and dark background values when the system asks, and the theme control cycling and surviving a reload.

## How to verify

```sh
cd frontend
npm ci
npx tsc -b
npm test -- --run
npm test -- --run --coverage
npm run build
npx playwright install chromium
npm run e2e
```

`npm test -- --run` runs the unit suite once; `npm test` alone starts watch mode. `npm run e2e` needs the Go toolchain on PATH because it starts the backend, and needs the Chromium download once per machine.

| Rule | Check |
|---|---|
| The browser never computes | `api.test.ts` asserts the request body; `App.test.tsx` asserts one fetch per `=`; no arithmetic exists outside `api.ts` |
| One request per `=` | `App.test.tsx` "sends one request per equals and shows the server result" |
| Left to right | `calculator.test.ts` "evaluates left to right as you go", e2e "evaluates left to right" |
| Every key sends the right operation | `App.test.tsx` "sends \<operation\> for the matching key" |
| Keys disabled in flight | `App.test.tsx` "disables every key while in flight and cannot send a second request" |
| A failure is shown and recoverable | `App.test.tsx` "renders a refusal, hides the code, and recovers on clear", "retries the same operands when equals is pressed after a refusal", "retries a failed square root on equals" |
| Result displayed verbatim | `App.test.tsx` "shows a long decimal result unaltered and scaled to fit" |
| A pending operation survives a root | `calculator.test.ts` "keeps a pending operation when a square root succeeds inside it" |
| A stalled request is bounded | `api.test.ts` "reports a timeout separately from an unreachable service" |
| Keyboard map and shortcuts | `keyboard.test.ts`, `App.test.tsx` "does not clear the calculation on a browser shortcut" |
| Responsive and themed | e2e "fits a phone screen without horizontal scroll", "fits 844x390 / 667x375 / 640x360 / 568x320 without scrolling", "switches to dark colours when the system asks", "cycles the theme and keeps it after a reload" |
| Theme logic | `theme.test.ts`; `App.test.tsx` "cycles the theme and remembers the choice", "does not re-toggle the theme when Enter follows a click" |
| An over-long operand never reaches the wire | `calculator.test.ts` "operand length"; `App.test.tsx` "refuses to reuse a result longer than the operand limit" |
| A request can be cancelled by touch | `App.test.tsx` "cancels an in-flight request from the keypad and ignores its late response" |
| The request is bounded in time | `App.test.tsx` "reports a timeout instead of waiting forever" |
| Inert keys are visibly disabled after an error | `App.test.tsx` "disables the inert keys while the error is showing" |
