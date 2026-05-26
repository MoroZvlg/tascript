# tascript — Language Specification

> Status: **draft / in design**. Sections fill in as decisions lock.

## 1. Purpose

tascript is a DSL for declaring fast streaming technical-analysis signal/alert
generators.

Pipeline: **candles → indicators + logic → signals**.

The runtime executes user programs over a live (or replayed) candle stream and
emits structured signal events. tascript is strictly a *signal generation*
language — it does not place trades, does not run backtests, and does not
deliver alerts to end channels. Trade execution and signal delivery live
outside the language.

## 2. Signal Output

The output of every tascript program is a stream of events emitted through
declared **output ports**. An event has the shape:

```
{
  output: string,        // declared output port name
  ts:     timestamp,     // candle time the event was produced at
  value:  String | null, // present for value outputs (String); null for structured
  data:   { ...fields }  // arbitrary user-defined payload fields
}
```

A program may emit zero or more events per candle, and may emit events to
different declared outputs within the same program. Emission is performed
via the built-in runtime action `emit(...)` — see section 5.2 for the full
signature.

Inputs and outputs are both declared in the script, but wired to real blocks
outside the DSL:

```js
input btc: CandleSeries

output alerts {
  kind: String
  price: Number
}
output logs: String
```

The host runtime / UI / deployment manifest maps `btc`, `alerts`, and `logs`
to concrete blocks. The language itself knows nothing about exchanges,
symbols, transports, formats, or credentials.

**Why this shape:**
- The script stays self-describing; `btc` and `alerts` are not magic external
  names.
- Runtime wiring stays outside the DSL, matching the block-based product
  model.
- `input` declarations are static dependencies; `emit(...)` remains runtime
  behaviour.
- The compiler can validate port names and output payload schemas.

**Out of scope for the DSL:**
- Delivery destinations (Telegram, Slack, webhooks, …)
- Output formatting (Markdown, JSON, plain text, …)
- Secrets, credentials, transport configuration
- Routing/fan-out rules

These belong to a separate routing/delivery layer that consumes the event
stream produced by the runtime.

## 3. Grammar

### 3.1 Surface style (locked decisions)

tascript leans on a JavaScript-flavoured surface syntax with deliberate
deviations:

- **No statement terminators.** Newlines end statements; `;` is not used.
  Newlines are ignored while inside an open `(`, `[`, or `{`, so output
  schemas and long calls may span multiple lines. Broader trailing-token
  continuation is deferred (see §"Gaps surfaced by these examples", item 5).
- **C-style blocks.** `if (cond) { ... } else { ... }` — parentheses around
  conditions, braces around bodies.
- **C-style logical operators.** `&&`, `||`, `!` (not `and`, `or`, `not`).
- **No variable-declaration keywords.** There is no `var` / `let` / `const`.
  Inside a function body, a bare assignment such as
  `uptrend = ema(50) > ema(200)` creates a binding that lives **only for the
  current invocation** of that function and is dropped when it returns.
- **Persistent state is namespaced.** Values that must survive across candles
  live on a special `state` object: `state.cooldown = 20`, `state["my key"]`.
  Both dot and bracket access are supported, matching JS conventions.
- **History reference.** `series[n]` reads the value `n` candles ago;
  `series[0]` (or just `series`) is the current candle.

### 3.2 Program structure

Every tascript program is composed of:

1. **Zero or more top-level constants.** A bare assignment at the top level
   (`COOLDOWN_BARS = 20`, `THRESHOLD = 0.5`) declares a *module constant*.
   The right-hand side is evaluated **once at program load**. The binding is
   readable from inside `Init()` and `Run()` by name, but **reassignment from
   inside any function is a parse-time error** — top-level constants are
   read-only after load.

2. **One or more input declarations.** A top-level declaration of the form
   `input name: Type` declares that the program consumes a runtime-wired
   input port. The binding is a read-only top-level value with the declared
   type. See section 3.3 for full semantics.

3. **One or more output declarations.** A top-level declaration of the form
   `output name: Type` declares that the program can emit to a runtime-wired
   output port. Optional output schemas describe valid payload fields.

4. **A required `function Init() { ... }`.** Runs **exactly once** before the
   first candle is processed. Intended for initialising `state.*` fields.

5. **A required `function Run() { ... }`.** Runs **once per candle**, in
   order. This is where indicators are read, conditions evaluated, and
   `emit(...)` calls produced.

Both `Init` and `Run` are mandatory. A program missing either is rejected at
parse time. No other top-level forms are permitted in the current language
revision; user-defined helper functions may be added later.

Canonical program shape:

```js
COOLDOWN_BARS = 20

input btc: CandleSeries
input eth: CandleSeries

output alerts {
  kind: String
  price: Number
  rsi: Number
  eth_rsi: Number
}

function Init() {
  state.cooldown = 0
}

function Run() {
  state.cooldown = math.max(0, state.cooldown - 1)

  uptrend    = btc.ema(50) > btc.ema(200)
  crossed    = ta.crossunder(btc.rsi(14), 30)
  context_ok = eth.rsi(14) < 50

  if (uptrend && crossed && context_ok && state.cooldown == 0) {
    emit(alerts,
         kind    = "btc_rsi_oversold_uptrend",
         price   = btc.closes[0],
         rsi     = btc.rsi(14),
         eth_rsi = eth.rsi(14))
    state.cooldown = COOLDOWN_BARS
  }
}
```

### 3.3 Ports — dependency-injected block IO

The DSL does not name exchanges, symbols, timeframes, Telegram chats, or
webhook URLs. Instead, every program declares named **ports** at the top
level:

```js
input btc: CandleSeries
input sentiment: Series

output alerts {
  kind: String
  price: Number
}
output logs: String
```

The actual blocks attached to those ports are configured **outside** the DSL
by the runtime, launching UI, or deployment manifest. The same program can
therefore run unchanged against Binance, Coinbase, a replay file, synthetic
test data, or future custom input blocks.

Input and output declarations are not normal function calls. They are static
port declarations used for validation, tooling, and runtime wiring.

#### Input declarations

An input declaration has the form:

```js
input <name>: <InputType>
```

Current input types:

| Type | Meaning |
|------|---------|
| `CandleSeries` | A stream of OHLCV candles. Needed by candle-based indicators such as `atr`/`dmi`. |
| `Series` | A numeric stream (`Series` of `Number`) from a custom block or external metric. Scalar indicators may be called on it. |

The declared name becomes a read-only top-level value. For example, `btc`
above is readable inside `Init()` and `Run()` as a `CandleSeries`.

#### Output declarations

An output is **either** a value output, a structured output, or both. The
type after `:` (if present) is the primary emitted `value`; a `{ … }` block
(if present) declares structured `data` fields.

```js
output <name>: <ValueType>             // value output  — emit a single value
output <name> { field: Type, … }       // structured    — emit keyword fields
output <name>: <ValueType> { field: Type, … }   // both: value + fields
```

`<ValueType>` is an ordinary value type. Current revision allows `String` and
`Number`; structured `data` fields may be any value type. The three shapes map
directly onto the event record: a value output sets `value` and leaves `data`
empty, a structured output leaves `value` `null` and fills `data`, and the
combined form sets both.

Output names are not readable values and cannot be assigned, passed around,
or called as objects. They are valid only as the first argument to
`emit(...)` inside `Run()`.

#### Port rules

- Port declarations may appear **only at the top level**.
- Port names are normal identifiers, not string literals.
- Input, output, constant, function, namespace, and reserved names share one
  top-level namespace; duplicate names are parse-time errors.
- An input binding is **read-only**; reassignment inside a function is a
  parse-time error.
- An output name is **emit-only**; reading or assigning it is a parse-time
  error.
- A declared input or output that the launcher did not wire is a
  **launch-time error before any data is processed**. The program never
  enters a partially-broken state.
- An `emit(...)` call that targets an undeclared output is a parse-time error.

**Out of scope for the current revision** (may be added later):

- User-tunable config inputs (`input period: Number = 14` or similar).
- **Named, reusable custom types** (e.g. `type Alert { … }` then
  `output x: Alert`). v1 only has the *anonymous* inline `{ … }` schema per
  output; a named type-declaration system is an open question deferred until
  real demand. Anonymous schemas cover the rich-payload use case without it.
- Per-input metadata (`btc.symbol`, `btc.exchange`, `btc.timeframe`). For
  now, payload identifiers are explicit fields emitted by the program.

### 3.4 Value types

The minimal type set is:

| Type           | Notes |
|----------------|-------|
| `Number`       | Single numeric type, `float64`. Both `14` and `1.5` are `Number`. Division always returns `Number` (no integer-division surprises). |
| `Bool`         | `true`, `false`. |
| `String`       | Double-quoted only: `"BTC/USDC"`. |
| `Null`         | A single bottom value `null`. Reserved for future static-nullable-analysis work — minimise direct use. Reading an unassigned `state.*` field or an out-of-range history is **not** null, it is a runtime error. |
| `Series`       | An ordered stream of `Number` values supporting the history operator `s[n]`. Sources: indicator outputs, plural properties of `CandleSeries` (`.closes`, `.opens`, …), names bound to such expressions. Not user-constructable. |
| `Candle`       | A single candlestick at one moment in time. Singular property access: `.open`, `.high`, `.low`, `.close`, `.volume`, `.ts`. Each yields a `Number` (or, for `.ts`, a timestamp). |
| `CandleSeries` | A stream of candles. Plural property access yields a `Series` of numbers: `.opens`, `.highs`, `.lows`, `.closes`, `.volumes`, `.timestamps`, plus the derived `.hl2` (`(high+low)/2`) and `.hlc3` (`(high+low+close)/3`). Indexable with `[n]` to read the *n*-th-ago candle (yielding a `Candle`). `cs[1].close` is equivalent to `cs.closes[1]`. Not user-constructable — `CandleSeries` values are injected by the runtime, one per declared input. |
| `Tuple`        | Ordered, fixed-arity collection produced by multi-output stdlib calls (e.g. `MACD`, `BB`, `DMI`). Indexed with `t[i]` where `i` is an integer literal or expression. Out-of-range = runtime error. No tuple literal syntax — tuples only come from function returns. Elements of an indicator tuple are themselves `Series`. |
| `Time`         | A point in time. Sources: `Candle.ts`, `CandleSeries.timestamps[n]`. All component methods are in UTC for v1 (no time-zone support). See § 3.5. |
| `Duration`     | A length of time. Produced by `Time - Time`, by multiplying a `Number` by a `time.*` Duration constant (`5 * time.MINUTE`), or by `Duration` arithmetic. See § 3.5. |

### Numeric validation at the indicator boundary

The single-`Number` design is preserved for grammar simplicity, but indicator
stdlib entries declare which of their parameters must be **whole, positive**
numbers (`period`, `length`, `lookback`, etc.):

- Non-integer literal at parse time → parse error
  (e.g. `rsi(14.21)` → `"rsi: 'period' must be a positive integer, got 14.21"`).
- Non-integer expression at runtime → runtime error.

A future revision MAY split `Number` into `Int` and `Float` for static
analysis. The migration in both directions is intentionally cheap: division
already returns a number-typed value, and indicator signatures already model
integer-only params, so adding/removing the distinction is mostly metadata.

### 3.5 Time and Duration

#### `Time`

A `Time` value represents a point in time. Sources:

- `Candle.ts` — the timestamp of a single candle
- `CandleSeries.timestamps[n]` — the timestamp `n` candles ago

There is no time-zone support in v1. All component access is in UTC. If
non-UTC behaviour is needed later, it will land as an explicit conversion
method (e.g. `t.in("America/New_York")`) without breaking the v1 API.

**Properties (UTC):**

| Property      | Type   | Notes |
|---------------|--------|-------|
| `.unix_ms`    | Number | total milliseconds since the Unix epoch |
| `.year`       | Number | full year, e.g. 2026 |
| `.month`      | Number | 1 through 12 |
| `.day`        | Number | day of month, 1 through 31 |
| `.weekday`    | Number | 0 through 6, **0 = Sunday** (JS convention) |
| `.hour`       | Number | 0 through 23 |
| `.minute`     | Number | 0 through 59 |
| `.second`     | Number | 0 through 59 |
| `.millisecond`| Number | 0 through 999 |

**No `Time` literal syntax in v1.** Times are obtained from candles. If
"epoch zero" or a fixed anchor is needed, derive arithmetically from a
candle timestamp.

#### `Duration`

A `Duration` is a length of time, in milliseconds, signed. Produced by
`Time - Time`, by `Number * <constant>`, or by arithmetic on existing
durations.

**Duration constants live in the `time.*` namespace** (`time` is a reserved
namespace identifier alongside `math` and `ta`):

| Constant | Value     |
|----------|-----------|
| `time.MILLISECOND` | 1 ms |
| `time.SECOND`      | 1000 ms |
| `time.MINUTE`      | 60 × `time.SECOND` |
| `time.HOUR`        | 60 × `time.MINUTE` |
| `time.DAY`         | 24 × `time.HOUR` |
| `time.WEEK`        | 7 × `time.DAY` |

No `time.MONTH` or `time.YEAR` — those are calendar-dependent, not fixed
durations. A future `t.advance_months(n)` style method may cover that case if
real programs need it.

The `time` namespace will also host future time-related helpers (e.g.
`time.parse("2025-01-01T00:00:00Z")` once a `Time` literal source is needed).

**Properties:**

| Property      | Type   | Notes |
|---------------|--------|-------|
| `.unix_ms`    | Number | total milliseconds |
| `.seconds`    | Number | total seconds (may be fractional) |
| `.minutes`    | Number | total minutes |
| `.hours`      | Number | total hours |
| `.days`       | Number | total days |
| `.weeks`      | Number | total weeks |

#### Operator semantics

`Time`:

| Op                       | Result        |
|--------------------------|---------------|
| `Time - Time`            | `Duration`    |
| `Time + Duration`        | `Time`        |
| `Time - Duration`        | `Time`        |
| `Time < Time`, `<=`, `>`, `>=` | `Bool`  |
| `Time == Time`           | `Bool` (strict same-type equality) |
| `Time + Time`            | parse-time error |
| `Time + Number`, `Time - Number` | parse-time error (use `Duration`) |

`Duration`:

| Op                          | Result      |
|-----------------------------|-------------|
| `Duration + Duration`       | `Duration`  |
| `Duration - Duration`       | `Duration`  |
| `Number * Duration`         | `Duration`  |
| `Duration * Number`         | `Duration`  |
| `Duration / Number`         | `Duration`  |
| `Duration / Duration`       | `Number` (ratio) |
| `Duration < Duration` etc.  | `Bool`      |
| unary `-Duration`           | `Duration` (negation legal) |
| `Duration + Number`         | parse-time error |

`Time` and `Duration` are both first-class values. They may be stored in
`state.*`, passed in `emit(...)` kwargs (which serialise via `.unix_ms` at
the delivery boundary), and assigned to per-call locals.

#### Example

```js
COOLDOWN = 30 * time.MINUTE

input btc: CandleSeries

output alerts {
  price: Number
}

function Init() {
  state.last_signal = btc.timestamps[0] - time.DAY   // bootstrap to "yesterday"
}

function Run() {
  if (btc.rsi(14) < 30
      && btc.timestamps[0] - state.last_signal > COOLDOWN) {
    emit(alerts, price=btc.closes[0])
    state.last_signal = btc.timestamps[0]
  }
}
```

### 3.6 Operators, equality, and conditions

tascript deliberately rejects JavaScript's loose-equality / truthy-falsy
semantics. Strict rules apply throughout. The model is: catching bugs in a
DSL that runs continuously is worth more than the convenience of silent
coercions.

#### Lift rule for `Series` in scalar context

A `Series` value in any scalar context auto-evaluates to its current value
(equivalent to `s[0]`). This applies uniformly:

- Comparisons: `btc.rsi(14) < 30`, `btc.ema(50) > btc.ema(200)`
- Arithmetic: `btc.closes - eth.closes` (current spread, a `Number`)
- Function call arguments / `emit(...)` kwargs
- Right-hand side of assignment to a non-state binding

Arithmetic does **not** construct a derived `Series` in this revision —
`btc.closes - eth.closes` is a `Number`, not a `Series`. Series-producing
arithmetic may be added later as explicit methods (`series.add(other)`,
`series.sub(other)`, …) without grammar changes.

#### Equality (`==`, `!=`)

Equality requires the two operands to have the **same type**. Cross-type
comparison is a parse-time error when statically detectable, a runtime
error otherwise. No silent `false`, no implicit conversion.

| Operands | Result |
|----------|--------|
| `Number` vs `Number` | numeric equality |
| `String` vs `String` | byte-equal |
| `Bool`   vs `Bool`   | identity |
| `Null`   vs `Null`   | `true` |
| `Null`   vs anything else | error (use a future `math.is_null` helper for nullness checks) |
| any other cross-type pair | error |

`Series` operands auto-extract via the lift rule, so the rule above applies
to the extracted scalar values.

#### Ordering (`<`, `<=`, `>`, `>=`)

Ordering is defined **only on `Number` × `Number`**. Any other operand
combination is a type error. `Series` operands auto-extract first. String,
`Bool`, `Null` ordering will error — if string ordering is needed later,
it lands as a `str.compare(...)` helper, not as `<`.

#### Boolean position

`if (cond) { ... }`, `&&`, `||`, and the unary `!` all require `Bool`
operands. A `Number` (including `0`), `String`, `Null`, `Series` (after
lift), `Candle`, `CandleSeries`, or `Tuple` in boolean position is a type
error. No truthy / falsy values.

```js
if (state.cooldown)     { … }   // error — cooldown is a Number
if (state.cooldown > 0) { … }   // ok
```

#### Operator precedence and associativity

Standard C/JS precedence applies. Tightest to loosest:

| Tier | Operators                       | Associativity |
|------|---------------------------------|---------------|
| 1    | `()` grouping, `.` member access, `[]` indexing, function/method call | n/a |
| 2    | unary `!`, unary `-`            | right         |
| 3    | `*`, `/`, `%`                   | left          |
| 4    | binary `+`, binary `-`          | left          |
| 5    | `<`, `<=`, `>`, `>=`            | left          |
| 6    | `==`, `!=`                      | left          |
| 7    | `&&`                            | left          |
| 8    | `\|\|`                          | left          |

**Not present:** bitwise operators (`&`, `|`, `^`, `<<`, `>>`), ternary
`? :`, exponentiation operator (`**` / `^` — use `math.pow(x, y)`).

**Assignment (`=`) is a statement, not an expression.** Forms like
`a = b = c` or `x = (y = 1)` are parse-time errors. This avoids JS-style
"assignment returns a value" foot-guns and keeps the grammar smaller.

#### Comments

Single-line only:

```js
// this is a comment
state.cooldown = 0   // comments may follow code
```

No block-comment form (`/* */`). If a multi-line note is needed, use
several `//` lines. Keeps the lexer simpler.

#### Short-circuit evaluation

`&&` and `||` short-circuit. The right-hand side is not evaluated when the
left-hand side already determines the result. This matters chiefly for
expressions with side effects (rare in tascript) and as a documented
contract for users; per-tick indicator memoization makes the perf
difference negligible in typical programs.

### Indexing operator `[n]`

The `[n]` operator is **dispatched by the type of the left-hand side**:

- LHS is a `Series`       → `[n]` reads the value `n` candles ago. `s[0]` is current.
- LHS is a `CandleSeries` → `[n]` reads the `Candle` `n` candles ago. `cs[0]` is current.
- LHS is a `Tuple`        → `[n]` reads the `n`th element (0-based).

There is no ambiguity at parse time — all three forms share the same
syntactic shape — the runtime resolves by value type. Chained indexing such
as `MACD(12,26,9)[0][1]` (tuple-index → series → history) or `cs[1].close`
vs. `cs.closes[1]` (equivalent) is legal.

## 4. Semantics

### 4.1 Execution model

tascript uses a **per-candle imperative** execution model. For every candle in
the stream, the runtime:

1. Advances the internal state of every indicator referenced by the program
   (mapping onto `talive.Indicator.Next(candle)` semantics).
2. Executes the program body top-to-bottom in the context of "the current
   candle". Names like `close`, `high`, `rsi(14)`, `ema(50)` resolve to their
   value *at this candle*.
3. Collects any `emit(...)` calls performed during that execution into the
   output event stream.

Indicators referenced more than once within the same candle are memoized
per-candle so that repeated reads (e.g. `rsi(14) < 30` and a later
`data: { rsi: rsi(14) }`) do not re-advance state.

### Run() cadence is a runtime concern, not a DSL concern

When a program declares multiple input ports, the rate at which `Run()`
fires is **not** specified by the DSL. The host runtime sits a configurable
*synchronizer* in front of the program's execution loop. Today's planned
synchronizer modes (more may be added later):

- **None / async** — `Run()` fires on every candle of any input. Reads of
  other inputs return their last-known state.
- **Classic sync** — `Run()` fires only when all inputs have advanced past
  a shared timestamp boundary, optionally with a timeout threshold.

The selected mode is part of how the program is deployed (UI block config,
deployment manifest, etc.), **not** part of the program source. tascript
itself offers no `primary(...)`, no `synchronize(...)`, no way to read or
branch on the current cadence mode. Programs are written to be correct under
whichever cadence the operator configures — typically by reading candle
timestamps and using `state.*` to debounce when needed.

### Warmup is invisible to the DSL

Indicator warmup is **not** a language concern. It is handled entirely by the
runtime before a program goes live:

1. On program load, the runtime parses the program and discovers every
   indicator reference (regardless of branch reachability — a static AST scan).
2. The runtime instantiates every indicator and computes the maximum
   `WarmUpPeriod()` across them.
3. The runtime requests that many historical candles from the data source and
   feeds them through every indicator until each reports `IsWarmedUp()`.
4. **Only then** is the user's `Run()` invoked for the first time, against a
   live candle, with every indicator already producing reliable output.

Programs therefore never need to check `idle` / `warmed_up` flags — those
concepts do not exist in the DSL. Every read of an indicator inside `Run()`
returns a meaningful value.

### 4.2 History buffer sizing

Every `Series` value the program reads has a per-series ring buffer that
makes the `[n]` history operator possible. The buffer size is determined by
**static AST analysis** at parse time — users do not declare or manage it.

#### What contributes to a series' buffer size

For each `Series`-typed expression in the program, the static analyser takes
the maximum lookback across all of its references:

1. **Explicit literal indices.** `btc.closes[5]` contributes 5 to
   `btc.closes`.
2. **Helper-signature lookback.** Each helper that consumes a `Series`
   declares its lookback in the stdlib registry; the analyser adds that
   value to the series' bound. Initial entries:

   | Helper | Lookback contribution |
   |--------|-----------------------|
   | `ta.crossover(a, b)`   | 1 on each of `a`, `b` |
   | `ta.crossunder(a, b)`  | 1 on each of `a`, `b` |
   | `ta.rising(s, n)`      | `n` on `s` |
   | `ta.falling(s, n)`     | `n` on `s` |
   | `ta.highest(s, n)`     | `n - 1` on `s` |
   | `ta.lowest(s, n)`      | `n - 1` on `s` |

Buffer for the series is then `max(literal lookbacks ∪ helper contributions) + 1`.

#### Indicator-output series share buffer through memoization

A call like `btc.rsi(14)` is memoized by
`(receiver_identity, indicator_class, normalized_args)` (§ 5.1). All
references to `btc.rsi(14)` across the program share the same wrapper, and
therefore the same buffer. The buffer size for that wrapper is the max
lookback computed across all reference sites.

#### Literal-only constraint

Both the `n` in `series[n]` and the lookback arguments of helpers
(`ta.highest(s, n)`, etc.) **must be literal `Number` expressions at parse
time**. Dynamic indices (e.g. `series[state.x]`, `ta.highest(s, state.n)`)
are rejected at parse time. This keeps static analysis tractable. In
practice TA programs always know their periods at edit time; if a real
program ever needs dynamic lookback, an escape hatch will be added without
breaking compatibility.

#### Free interaction with warmup

The runtime's warmup phase (§ 4.1) feeds historical candles through every
indicator before the first `Run()` invocation. Those warmup candles also
populate the per-series ring buffers. By the time `Run()` first executes,
every buffer is already filled to its static bound; users never observe
`HISTORY_OUT_OF_RANGE` errors during normal operation. The historical fetch
the runtime requests is therefore:

```
max( max(indicator.WarmUpPeriod),
     max(static series buffer bound) )
```

### 4.3 State and history

tascript exposes two complementary forms of memory across candles:

1. **Historical references.** A series-valued expression (a candle field such
   as `close`, or an indicator output such as `rsi(14)`) supports an `[n]`
   postfix that reads its value `n` candles in the past. `close[0]` (or just
   `close`) is the current candle; `close[1]` is the previous candle, and so
   on. History buffers are managed by the runtime; the program does not
   allocate them.

2. **User-declared persistent state.** A program writes persistent values to
   the namespaced `state` object (e.g. `state.cooldown = 20`). All `state.*`
   fields survive between candle executions. Initial values are established in
   the program's `Init()` function, which runs once before the first candle.
   Reading a `state.*` field that has never been assigned is a runtime error
   (no silent zero / null defaults); this forces every persistent field to be
   declared in `Init()` and prevents typo-driven foot-guns.

Plain bindings inside a function body are scoped to that single invocation and
do not persist.

## 5. Standard Library

### 5.1 Indicators

Indicators are sourced from the `talive` library and exposed in the DSL as
**methods** rather than free functions. Two call shapes exist:

```js
// (a) on a CandleSeries — every indicator is callable here
btc.rsi(14)              // Scalar indicator; defaults to btc.closes
btc.ema(50)
btc.macd(12, 26, 9)      // multi-output → Tuple<Series>
btc.bb(20, 2)            // multi-output → Tuple<Series>
btc.atr(14)              // non-Scalar; uses high/low/close internally

// (b) on a Series — ONLY Scalar indicators are callable here
btc.highs.rsi(14)        // RSI of the highs Series
btc.hlc3.rsi(14)         // RSI of typical price
btc.rsi(14).sma(15)      // smooth RSI with SMA — chaining
btc.closes.ema(5).rsi(14).sma(3)
```

**Two classes of indicators** (mirroring `talive`'s `Scalar` interface):

- **Scalar indicators** — single output, composable on any numeric `Series`.
  Examples: `sma`, `ema`, `smma`, `wma`, `dema`, `tema`, `rsi`, `cci`, …
- **Non-Scalar indicators** — may require multiple candle fields (`atr`,
  `dmi`), produce multiple outputs (`macd`, `bb`, `ichimoku`), or both.
  Callable **only** on `CandleSeries`. Attempting to call them on a `Series`
  is a parse-time error.

**Indicator configuration via keyword arguments.** Indicators accept
configuration beyond their positional parameters (e.g. RSI's MA type, gain
method, loss method). The DSL exposes these as keyword arguments after the
positional list:

```js
btc.rsi(14, source=HLC3)
btc.rsi(14, ma=SMMA)
btc.rsi(14, source=HLC3, ma=SMMA)
```

**Source override — two equivalent forms.** Calling a Scalar indicator on a
non-default Series property is equivalent to passing the corresponding
`source=` keyword:

```js
btc.hlc3.rsi(14)         //  ≡  btc.rsi(14, source=HLC3)
btc.highs.rsi(14)        //  ≡  btc.rsi(14, source=HIGH)
```

Either form is legal; pick whichever reads more clearly in context. They
produce the same memoized instance.

**Built-in indicator constants** are reserved identifiers in the DSL.
Reassigning any of them at top level or inside a function is a parse-time
error.

| Category | Reserved identifiers |
|----------|----------------------|
| Source   | `CLOSE`, `OPEN`, `HIGH`, `LOW`, `HL2`, `HLC3` |
| MA type  | `SMA`, `EMA`, `SMMA`, `WMA`, `DEMA`, `TEMA` |
| Anchor   | `NONE`, `DAILY`, `WEEKLY`, `MONTHLY`, `QUARTERLY`, `YEARLY` |

Future stdlib growth may add more reserved identifiers; redefinition remains
a parse error.

**Memoization.** Indicator calls are memoized by the tuple
`(receiver_identity, indicator_class, normalized_args)`, where
`normalized_args` folds defaults. `btc.rsi(14)`, `btc.rsi(14, source=CLOSE)`,
and `btc.closes.rsi(14)` all share one underlying indicator instance and
advance state exactly once per tick. The receiver identity distinguishes
`btc.rsi(14)` from `eth.rsi(14)`.

**Stdlib registry.** Each indicator has a registry entry mapping a DSL call
to a talive constructor and builder chain:

```
rsi:
  positional: [period: Int(positive)]
  kwargs:     { source: SourceConst = CLOSE,
                ma:     MaConst     = SMMA,
                ... }
  build:      NewRSI(period).WithSource(...).WithMA(...)
```

Adding new indicator configuration as talive evolves means adding fields to
the registry — no DSL change required.

### 5.2 emit(...) — signal emission

```
emit(OUTPUT [, ident=expr]*)
emit(OUTPUT, value_expr [, ident=expr]*)
```

Where:

- `OUTPUT` is a declared output identifier, not a string literal.
- `emit(...)` is a built-in runtime action. It is valid **only inside
  `Run()`**. Use in `Init()` or at the top level is a parse-time error.
- For a **structured** output (`{ … }`), the payload is keyword arguments
  only — no leading value.
- For a **value** output (`: <ValueType>`), the second argument is the value
  and must match the declared type. If the output *also* declares a `{ … }`
  schema, keyword arguments may follow.
- `ident` is a normal identifier — letters and digits and underscores
  (initial revision may restrict to letters only and relax later).
- `expr` must evaluate to a serialisable value: `Number`, `Bool`, `String`,
  `Null`, `Time`, or `Duration`. A `Series` is read at its current value per
  the lift rule (§3.6). Passing a `CandleSeries`, `Candle`, or `Tuple` is a
  runtime error.
- Empty structured payload is legal: `emit(heartbeat)` produces an event
  with `data: {}` when `heartbeat` is a declared structured output.

Examples:

```js
input btc: CandleSeries

output logs: String

function Init() {}

function Run() {
  emit(logs, "BTC crossed above EMA")
}
```

```js
input btc: CandleSeries

output price_alert: String {
  price: Number
}

function Init() {}

function Run() {
  emit(price_alert, "BTC crossed above EMA", price=btc.closes[0])
}
```

There is no in-language string interpolation in v1. Parameters attached to a
value output are structured `data` for the host-side renderer, not template
variables interpreted by tascript.

**Reserved kwarg names** — runtime-injected, cannot be passed by the user.
Using any reserved name as a kwarg is a parse-time error. The set is
expected to grow as the runtime exposes more context. Currently reserved:

- `ts` — the current candle timestamp at the moment `emit(...)` is called.
- `output` — the declared output port name.

**Schema enforcement.** A structured output's `{ … }` schema is **strict and
closed** in this revision:

- every declared field **must** be supplied by each `emit(...)` to that output;
- **no** undeclared fields may be supplied;
- each field's value **must** match the declared type.

Any violation is a parse-time error when statically detectable, otherwise a
runtime error before the event is delivered. (Strict is the safe starting
point: relaxing to optional fields later stays backward-compatible, whereas
tightening a loose schema would break existing programs.)

**Output discovery.** Tooling can statically enumerate available outputs by
reading top-level `output` declarations. Routing and fan-out rules remain a
host concern.

Broadcast helpers such as `emit(ALL, ...)` are not part of v1. If fan-out is
needed, configure one output port to route to multiple delivery blocks in the
host layer.

### 5.3 String formatting — deferred

The v1 language does **not** offer any in-program string composition
mechanism. Programs emit either a literal/string-valued message or structured
fields via `emit(...)`; rendering to human-readable form is the
responsibility of the output sink (Telegram template, webhook formatter,
etc.), configured outside the DSL.

No template syntax is reserved in v1. If string composition is needed later,
it will be designed separately.

### 5.4 Helpers — `math` and `ta` namespaces

Free-function helpers live in two namespaces. Namespaces are **passive
syntactic prefixes**, not first-class values — `math` and `ta` cannot be
assigned, passed around, or otherwise reflected on. `math.max(a, b)` is one
parse-time syntactic form.

#### `math` — pure math

| Call | Behaviour |
|------|-----------|
| `math.max(a, b)`   | larger of two numbers |
| `math.min(a, b)`   | smaller of two numbers |
| `math.abs(x)`      | absolute value |
| `math.sqrt(x)`     | square root |
| `math.pow(x, y)`   | x to the y |
| `math.floor(x)`    | round toward −∞ |
| `math.ceil(x)`     | round toward +∞ |
| `math.round(x)`    | round half-away-from-zero |

#### `ta` — technical-analysis helpers

| Call | Behaviour |
|------|-----------|
| `ta.crossover(a, b)`  | `true` on the bar where `a` crosses above `b` |
| `ta.crossunder(a, b)` | `true` on the bar where `a` crosses below `b` |
| `ta.rising(s, n)`     | `true` when `s` has strictly risen for the last `n` bars |
| `ta.falling(s, n)`    | `true` when `s` has strictly fallen for the last `n` bars |
| `ta.highest(s, n)`    | maximum of `s` over the last `n` bars (current bar included) |
| `ta.lowest(s, n)`     | minimum of `s` over the last `n` bars |

Arguments such as `s` accept any `Series` (a candle-field series, an
indicator-output series, etc.). `a` and `b` in `crossover` / `crossunder`
also accept `Number` for one of the two sides, e.g. `ta.crossunder(btc.rsi(14), 30)`.

#### Reserved namespace identifiers

The names `math` and `ta` are reserved. Reassigning either at the top level
or inside a function is a parse-time error. This is the entire reserved-name
cost of the helper library — new helpers are added inside existing namespaces
without burning any further identifiers.

**Indicator-of-derived restriction.** This revision permits chaining only
through Scalar indicators (each chain link must yield a `Series`). Calls
like `btc.macd(12,26,9).sma(5)` are rejected — a `Tuple` cannot be chained
directly. Extract a series first: `btc.macd(12,26,9)[0].sma(5)`.

## 6. Diagnostics

### 6.1 Two phases

Every error a tascript program can produce belongs to one of two phases:

- **Parse-time** — produced before any candle is processed. Examples:
  unknown identifier, reassignment of a reserved name, indicator called with
  a non-integer literal for a whole-number parameter, a port declaration used
  inside a function, `emit(...)` used outside `Run()`, `==` between
  mismatched types when statically detectable.
- **Runtime** — produced during execution of `Init()` or `Run()`. Examples:
  read of an unassigned `state.*` field, history index out of range,
  cross-type comparison the analyser could not statically rule out,
  zero-period indicator from a runtime expression.

### 6.2 Every error carries

- **Phase** — `parse` or `runtime`.
- **Category code** — stable, machine-readable identifier (see § 6.4).
- **Source location** — file path, line, column.
- **Source snippet with caret** — at minimum line+column, ideally rendered
  in the Rust/Elm style:
  ```
  error[STATE_UNSET]: 'state.cooldwon' has never been assigned
    --> alerts/rsi.tas:14:5
     |
  14 |   if (state.cooldwon == 0) {
     |       ^^^^^^^^^^^^^^^
  ```
- **Human-readable message** — describes what's wrong; may include a hint
  ("did you mean `state.cooldown`?").

### 6.3 Parse-time policy

The compiler **collects multiple parse-time errors before aborting**, with a
sensible upper bound (initial target: 100 errors per program; configurable).
Users should be able to fix many errors in one editing pass, not chase them
one-at-a-time. Parser recovery uses statement-level resynchronisation:
after a syntax error, the parser skips to the next statement boundary and
continues; the analyser then reports additional static errors on the AST that
was recovered successfully.

### 6.4 Stable category codes

Errors carry a category code that is stable across language revisions —
external tooling (UI, CI, IDE) can match against codes without parsing
message strings. The initial set, expanded as the implementation lands:

| Category | Phase | When |
|----------|-------|------|
| `TYPE_MISMATCH`         | parse / runtime | Operator or function applied to operands of incompatible types. |
| `BOOL_REQUIRED`         | parse / runtime | Non-`Bool` used in `if`, `&&`, `\|\|`, `!`. |
| `RESERVED_REASSIGN`     | parse | Attempt to assign to a reserved identifier or namespace. |
| `STATE_UNSET`           | runtime | Read of a `state.*` field never assigned. |
| `HISTORY_OUT_OF_RANGE`  | runtime | `series[n]` where insufficient history. |
| `INPUT_NOT_WIRED`       | launch | A declared input port has no source block configured. |
| `OUTPUT_NOT_WIRED`      | launch | A declared output port has no destination block configured. |
| `PORT_DUPLICATE`        | parse | Two top-level ports/bindings declare the same name. |
| `UNKNOWN_OUTPUT`        | parse | `emit(...)` targets a name that is not a declared output. |
| `EMIT_OUTSIDE_RUN`      | parse | `emit(...)` appears outside `function Run()`. |
| `EMIT_PAYLOAD`          | parse / runtime | Emitted value or kwargs do not match the output declaration. |
| `INDICATOR_PARAM`       | parse / runtime | Indicator parameter constraint violated (e.g. non-integer period). |
| `TOP_LEVEL_FORM`        | parse | A construct used at the top level that is not permitted there (e.g. `state.*`, `if`). |
| `MISSING_REQUIRED_FN`   | parse | Program does not declare `function Init()` or `function Run()`. |
| `EMIT_RESERVED_KWARG`   | parse | User passed a reserved kwarg name to `emit(...)` (e.g. `ts=`). |

Future categories are additive; existing codes never change meaning.

## 7. Resource Limits

The language enforces conservative resource limits to prevent pathological
programs from monopolising runtime resources. Initial values are starting
points; the host may tighten them via deployment configuration but cannot
relax them above the spec ceilings.

| Limit | Initial value | Phase | Behaviour on breach |
|-------|---------------|-------|---------------------|
| Max series buffer length        | 5000         | parse   | Parse error (`HISTORY_LIMIT`). Triggers when static analysis computes a per-series bound exceeding the cap (e.g. `ta.highest(s, 6000)`). |
| Max string literal length       | 4096 chars   | parse   | Parse error. |
| Max string value at runtime     | 4096 chars   | runtime | Runtime error if a string-valued expression evaluates to a longer string. Currently only relevant for kwarg payloads; v1 has no string-building, so this is mostly future-proofing. |
| Max `emit(...)` kwargs per call | 32           | parse   | Parse error. |
| Max identifier length           | 128          | parse   | Parse error. |
| Max nested expression depth     | 64           | parse   | Parse error. |
| Max source file size            | 256 KB       | parse   | Parse error. |
| Max parse errors collected      | 100          | parse   | After 100 errors the parser aborts the collection pass (§ 6.3). |

Wall-clock budgets per `Init()` and per `Run()` invocation are **operational
limits** enforced by the host runtime, not by the language. They are
documented in `RUNTIME.md`.

### Reserved category codes for limits

| Category | Phase | When |
|----------|-------|------|
| `HISTORY_LIMIT`       | parse   | Static analysis computed a per-series buffer bound exceeding the limit. |
| `STRING_LIMIT`        | parse / runtime | String literal or runtime string value exceeded length cap. |
| `KWARG_LIMIT`         | parse   | `emit(...)` call with more than the allowed number of kwargs. |
| `IDENT_LIMIT`         | parse   | Identifier longer than the allowed cap. |
| `DEPTH_LIMIT`         | parse   | Expression nested deeper than the allowed cap. |
| `SOURCE_SIZE_LIMIT`   | parse   | Source file larger than the allowed cap. |

These extend the category table in § 6.4.

## 8. Examples

The following programs exercise every locked language feature against
realistic TA signal scenarios. They are documentation, not test fixtures —
treat them as canonical reading material.

### 8.1 RSI oversold in an uptrend, with a bar-count cooldown

The simplest realistic alert. Uses one input, one persistent counter, an
indicator crossing condition, and a context-trend filter.

```js
COOLDOWN_BARS = 20

input btc: CandleSeries

output alerts {
  kind: String
  price: Number
  rsi: Number
}

function Init() {
  state.cooldown = 0
}

function Run() {
  state.cooldown = math.max(0, state.cooldown - 1)

  uptrend = btc.ema(50) > btc.ema(200)
  crossed = ta.crossunder(btc.rsi(14), 30)

  if (uptrend && crossed && state.cooldown == 0) {
    emit(alerts,
         kind  = "rsi_oversold_uptrend",
         price = btc.closes[0],
         rsi   = btc.rsi(14))
    state.cooldown = COOLDOWN_BARS
  }
}
```

### 8.2 MACD bullish crossover (multi-output indicator)

Exercises the `Tuple` return shape and per-tick local aliasing of indicator
output series.

```js
input btc: CandleSeries

output alerts {
  kind: String
  price: Number
  line: Number
  signal: Number
}

function Init() {
}

function Run() {
  line   = btc.macd(12, 26, 9)[0]
  signal = btc.macd(12, 26, 9)[1]

  if (ta.crossover(line, signal)) {
    emit(alerts,
         kind   = "macd_bullish_cross",
         price  = btc.closes[0],
         line   = line,
         signal = signal)
  }
}
```

### 8.3 Bollinger band breakout with volume confirmation and a time cooldown

Exercises `Duration`, `Time` arithmetic, `state.*` holding a `Time` value,
and a multi-output indicator with one slot ignored.

```js
COOLDOWN = 10 * time.MINUTE

input btc: CandleSeries

output alerts {
  kind: String
  price: Number
  volume: Number
}

function Init() {
  state.last_alert = btc.timestamps[0] - time.DAY
}

function Run() {
  upper = btc.bb(20, 2)[0]                // [upper, middle, lower] — order per talive
  // middle and lower not used

  breakout     = btc.closes > upper
  volume_spike = btc.volumes > btc.volumes[1] * 1.5
  cooled_down  = btc.timestamps[0] - state.last_alert > COOLDOWN

  if (breakout && volume_spike && cooled_down) {
    emit(alerts,
         kind   = "bb_breakout_up",
         price  = btc.closes[0],
         volume = btc.volumes[0])
    state.last_alert = btc.timestamps[0]
  }
}
```

### 8.4 EMA cross with weekday filter

Exercises `Time` properties and avoidance of weekend bars. Demonstrates
that without an early-return form the natural style is to wrap the body in
a filter `if`.

```js
input btc: CandleSeries

output alerts {
  kind: String
  price: Number
}

function Init() {
  state.cooldown = 0
}

function Run() {
  state.cooldown = math.max(0, state.cooldown - 1)

  weekday = btc.timestamps[0].weekday
  on_weekday = weekday != 0 && weekday != 6

  if (on_weekday) {
    cross = ta.crossover(btc.ema(20), btc.ema(50))
    if (cross && state.cooldown == 0) {
      emit(alerts,
           kind  = "ema_golden_cross",
           price = btc.closes[0])
      state.cooldown = 10
    }
  }
}
```

### 8.5 Cross-asset divergence with cooldown

Exercises two inputs, arithmetic across inputs (current spread), and a
time-based cooldown. Note: under "no synchronizer" cadence this may emit on
every BTC tick AND every ETH tick when conditions hold; under "classic
sync" it emits at most once per shared candle. The author handles either
with the `state.last_alert` time cooldown.

```js
SPREAD_THRESHOLD = 0.05
COOLDOWN         = 15 * time.MINUTE

input btc: CandleSeries
input eth: CandleSeries

output alerts {
  kind: String
  btc_change: Number
  eth_change: Number
  divergence: Number
}

function Init() {
  state.last_alert = btc.timestamps[0] - time.HOUR
}

function Run() {
  btc_change = (btc.closes - btc.closes[1]) / btc.closes[1]
  eth_change = (eth.closes - eth.closes[1]) / eth.closes[1]

  divergence    = math.abs(btc_change - eth_change)
  large_split   = divergence > SPREAD_THRESHOLD
  cooled_down   = btc.timestamps[0] - state.last_alert > COOLDOWN

  if (large_split && cooled_down) {
    emit(alerts,
         kind       = "btc_eth_divergence",
         btc_change = btc_change,
         eth_change = eth_change,
         divergence = divergence)
    state.last_alert = btc.timestamps[0]
  }
}
```

### Gaps surfaced by these examples

Writing the examples surfaced a few details the spec has not yet pinned
down. They will be picked up when implementation begins:

1. **Empty `Init()` body.** `function Init() { }` is legal in §8.2 — no
   `state.*` to bootstrap. Confirm: an empty function body is valid (it is,
   under the locked grammar).
2. **`return` statement.** No early-return form is locked. The natural
   substitute is wrapping body code in a filter `if` (see §8.4). If real
   programs grow nested, `return` should be a small addition.
3. **Negative numeric literals.** `state.last_alert = btc.timestamps[0] - time.DAY`
   uses subtraction; the unary minus operator covers writing `-5` directly.
   Confirmed by §3.6 precedence (unary `-` at tier 2). No separate negative
   literal token needed.
4. **Aliasing of indicator output through a local.** `line = btc.macd(...)[0]`
   binds a per-tick local to a `Series`. The static analyser must trace
   through such aliases to attribute lookback (`ta.crossover(line, signal)`)
   to the underlying `Series`. Doable but worth calling out explicitly in
   the static-analysis pass.
5. **Multi-line expressions / line continuation.** Bracket-depth suppression
   is locked: a NEWLINE is swallowed while inside an open `(` `[` `{`. This
   permits multi-line output schemas, multi-line `emit(...)` calls, and
   parenthesised splits:
   ```js
   x = (long_a + long_b +
        long_c)
   ```

   A broader trailing-token continuation rule is still deferred. Under that
   future rule, a NEWLINE only ends a statement when the preceding token can
   legally end an expression. If a line ends on a binary operator, `&&`,
   `||`, `,`, `.`, or an open bracket, the statement continues — so
   unparenthesised splits work:
   ```js
   x = long_a + long_b +
     long_c
   ```

   **Locked invariant for trailing-token continuation:** continuation is
   decided by the token at the **end** of a line, never the start. A line
   ending in a complete expression terminates there; a *leading* operator on
   the next line begins a new statement (unary), it does not retroactively
   continue the previous one. This avoids the JS automatic-semicolon-insertion
   ambiguity where `x = a` / `  + b` could be misread as `x = a + b`.
