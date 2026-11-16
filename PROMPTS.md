# Prompts used

The brief invited AI tooling and asked for the prompts I used (Instructions, item 5). These are the directions I gave while building the application, grouped by the part of the system they shaped. The wording is cleaned up from dictated input. I made the design decisions and reviewed each result; the model wrote the implementation and tests from those directions, and I corrected it where it drifted.

## The API and the data model

**"Propose the API shape and the number type, with the trade-offs, and let me choose before you write code."**

I chose one endpoint, `POST /api/v1/calculate`, with `{operation, operands}` rather than a route per operation, and fixed-point decimals rather than floats: `0.1 + 0.2` must be `0.3`, not `0.30000000000000004`.

**"Every result comes back as a string. A JSON number would lose precision in the browser, and a mixed number-or-string type would force every caller to branch."**

**"Give every failure its own code and the right 4xx: 400 for a request that cannot be read, 422 for one the calculator refuses, 404, 405 with `Allow`, 413 for size. One error envelope."**

**"Include the optional operations: exponentiation, square root and percentage. Percentage means `a % of b`. Fractional exponents are allowed, `0^0` is 1, and anything that is not a real number is refused."**

## Backend structure

**"The calculator package owns the arithmetic. The HTTP package only decodes, calls it and maps the result. The first pass mixed them; move operand parsing, arity and the operand-length limit into the calculator."**

**"`Calculate` should take decimal operands, not strings, and the JSON should be decoded by the decimal library rather than by hand. Keep the request layer to a thin wrapper that refuses `null` and over-long text."**

**"A request must not be able to make the service do unbounded work. Cap the body, the operand and the result, and refuse a power before computing it when the result would pass the limit. Measure the exact length, including a base below one, so `0.999^5000000` cannot run."**

The model's first guard skipped the fractional expansion for bases below one, which let that request return fifteen million characters. I caught it in review, had the guard measure the exact decimal length, and pinned the boundary cases in the tests.

**"Use the latest Go, and keep the server wiring short: flags, one `serve()` call, and a drain on SIGINT or SIGTERM."**

## Frontend structure and behaviour

**"The browser never calculates. Every result, including each step of a chain, comes from the API."**

The reducer sends one request per `=`, evaluates left to right without operator precedence, and displays exactly what the server sent.

**"Make entering a calculation feel normal: a keypad, an expression line, a result line, and the keyboard mapped to the same actions."**

**"When the service refuses, show its message, keep the display usable, and let `=` retry the failed request rather than deriving a different one."**

**"Fit a phone and a landscape phone without scrolling, keep a long result from pushing the keypad off screen, and stop a chain that would send an operand longer than the API accepts."**

**"Add a theme control: system appearance by default, light and dark one click away, remembered per browser."**

**"A request in flight must be cancellable on touch, not only with Escape."**

## Packaging

**"One image, one port: Go serves the API and the built frontend, so there is no proxy and no CORS."**

**"Keep the README and the three specs in the repo, matching the code, and add CI and the Dockerfile."**

**"Publish the image to GHCR once the repository is public, so it can be pulled and run without building."**
