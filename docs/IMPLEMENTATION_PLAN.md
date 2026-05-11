# tascript — Implementation Plan

> Status: **draft v2 — vertical slicing**. Each slice ships a runnable
> tascript with a strict subset of the spec. The downstream consumer
> (private project repo) integrates after every slice, exercises the new
> capability end-to-end, and feeds friction back into the design.

## Why slices, not phases

The earlier draft of this document grouped work by component (lexer →
parser → analyser → eval). That's a textbook compiler-bring-up shape, but
it leaves the downstream project blocked until most of the compiler is
built. Vertical slicing inverts that: every slice produces a working
tascript that the project can wire against, expose to real candle data, and
report problems with.

It also forces extensibility points to be designed early. When slice 5
adds the *first* indicator, the registry API has to be sharp enough for
the *next* indicator to slot in. Same with helpers in slice 7. The result
is a language that's extendable by construction — projects can register
their own indicators and helpers against the same APIs we use internally.

## How slices interact with the spec

`SPEC.md` is the final destination. Every slice implements a strict
subset; constructs not yet supported produce a clear `NOT_IMPLEMENTED`
parse error with a pointer to the slice that will land them. Each slice
also extends the test corpus.

## Repository topology

- **`tascript/`** — public Go module. Lexer, parser, analyser, evaluator,
  registry framework, history wrapper. **Not** data sources, **not** sinks,
  **not** ergo wiring.
- **Private project repo** — depends on `tascript` as a Go module. Owns
  the runtime host: ergo actors, real data adapters (Binance, …), real
  sink adapters (Telegram, …), the synchronizer, durable state, hot
  reload, UI.
- **`talive/`** — public Go module. Stays calc-only.
- **`tascript-history/`** — proposed split-out: the standalone history
  wrapper library introduced in slice 4. Reusable for non-tascript talive
  users. May live inside `tascript/history/` until it stabilises, then be
  promoted to its own repo.

## Public extension points

These appear progressively as slices land. Once in, they are public Go
APIs that the private project (or any downstream consumer) can register
against:

| Extension point | First slice | What it lets you add |
|-----------------|-------------|----------------------|
| `DataSource`     | 1 | A source of candles to wire into an `input("...")` slot |
| `Sink`           | 0 | A destination for emitted events |
| Indicator registry | 5 | Custom indicators callable as `<series>.<name>(...)` in DSL |
| Helper registry   | 7 | Custom `ns.fn(...)` helpers under existing or new namespaces |
| Resource policies | 10 | Override default limits per deployment |

The private project never modifies `tascript/`; it only registers against
these APIs.

---

## Slices

### Slice 0 — Walking skeleton

**Goal:** parse and execute the smallest possible program. Project wires it
up, sees an event come out.

**Language subset:**

- Lexer + parser for: `Number` and `String` literals, identifiers,
  comments, function declarations with empty parameter lists, `{ }`
  blocks, statement-level `=` assignment.
- Top-level: `K = expr` constant declarations (literals only),
  `function Init() { ... }`, `function Run() { ... }`. Both `Init` and
  `Run` mandatory.
- `emit(NAME_LITERAL, ident=expr*)` where expressions are constants or
  references to top-level constants.
- `input("slot_name")` parsed and recorded but **the resulting binding
  carries no data** — it's a placeholder that the runtime registers as
  "this program declared an input slot."

**Not yet:** indicators, helpers, history, state, conditions, types beyond
`Number`/`String`, candles.

**Public Go API:**

```go
// Compile turns source into a runnable Program.
tascript.Compile(src []byte) (*Program, []Diagnostic, error)

// A Program is launched against a wiring map.
prog.Inputs()  []string   // slot names declared by the program
prog.Outputs() []string   // output names used in emit() calls

// Launch validates wiring and produces a Runner.
tascript.Launch(prog, wiring) (*Runner, error)

// Runner.Step performs one tick (the caller decides cadence).
runner.Init()
runner.Step()                  // runs Run() once
events := runner.DrainEvents() // returns []Event since last drain
```

**Project's integration test for this slice:**

```js
GREETING = "hello world"

function Init() {}
function Run() {
  emit("alerts", message=GREETING)
}
```

Project calls `Init` then `Step` once, asserts that a single
`Event{name:"alerts", data:{"message":"hello world"}}` came out.

---

### Slice 1 — Real candles flow

**Goal:** a candle stream reaches a program; close prices reach `emit`.

**Adds:**

- `Candle` and `CandleSeries` value types with their property surfaces
  (`.open`, `.close`, etc.; `.opens`, `.closes`, etc.; derived `.hl2`,
  `.hlc3` lazy-computed per tick).
- `input("slot")` now produces a real `CandleSeries`. The runtime feeds
  candles into the slot each tick.
- The **lift rule** (§3.6) so `Series` in scalar context auto-extracts to
  its current value.
- Binary `+ - * / %` and unary `-` on `Number`.
- `DataSource` interface: a method that produces the next candle.

**Not yet:** history (`[n]`), comparisons, conditions, state, indicators.

**Project's integration test:**

```js
btc = input("btc_feed")
function Init() {}
function Run() {
  emit("ticks", price=btc.closes, volume=btc.volumes)
}
```

Project feeds a synthetic 100-candle stream, asserts 100 events come out
with the right per-candle prices.

---

### Slice 2 — Conditions, Bool, comparisons

**Goal:** programs can filter when they emit.

**Adds:**

- `Bool` type, `true`/`false` literals.
- Comparison operators (`==`, `!=`, `<`, `<=`, `>`, `>=`) with strict
  same-type semantics (§3.6).
- Logical operators (`&&`, `||`, `!`) with strict-`Bool` operands and
  short-circuit evaluation.
- `if (cond) { ... }` and `if (cond) { ... } else { ... }`.
- The full operator-precedence table from §3.6.

**Project's integration test:**

```js
THRESHOLD = 100

btc = input("btc_feed")
function Init() {}
function Run() {
  if (btc.closes > THRESHOLD) {
    emit("alerts", price=btc.closes)
  }
}
```

---

### Slice 3 — State

**Goal:** programs can remember across ticks.

**Adds:**

- `state.*` namespace.
- Assignment to `state.X` persists; reading an unassigned `state.X` is a
  runtime error (`STATE_UNSET`).
- `Init()` is invoked once before the first `Run()` to bootstrap state.
- `math.max` and `math.min` (because almost every program with state needs
  one of them — easier to land them here than to wait for the full
  `math.*` namespace in slice 7).
- The `math` namespace identifier becomes reserved.

**Project's integration test:**

```js
COOLDOWN_BARS = 5

btc = input("btc_feed")
function Init() {
  state.cooldown = 0
}
function Run() {
  state.cooldown = math.max(0, state.cooldown - 1)
  if (btc.closes > 100 && state.cooldown == 0) {
    emit("alerts", price=btc.closes)
    state.cooldown = COOLDOWN_BARS
  }
}
```

---

### Slice 4 — History via `[n]` + the wrapper library

**Goal:** programs can read past values. The standalone history wrapper
library is introduced as `tascript/history/` (later promotable to its own
module).

**Adds:**

- `[n]` postfix on `Series` (the operator already exists on `Tuple` once
  multi-output indicators land, but in this slice tuples aren't a value
  type yet, so `[n]` means "history" exclusively).
- Static analysis: parser tracks the maximum literal `n` per series and
  sizes a ring buffer accordingly (§4.2).
- `HISTORY_OUT_OF_RANGE` runtime error.
- `HISTORY_LIMIT` parse error at the 5000-bar cap (§7).
- The wrapper package: takes any per-tick value source, exposes
  `current()` and `history(n)`.

**Project's integration test:**

```js
btc = input("btc_feed")
function Init() {}
function Run() {
  if (btc.closes > btc.closes[1]) {
    emit("alerts", curr=btc.closes, prev=btc.closes[1])
  }
}
```

---

### Slice 5 — First indicator (EMA) + indicator registry

**Goal:** real TA reaches the DSL. The indicator registry API ships,
designed for the second indicator to slot in without changes.

**Adds:**

- Method-style indicator calls on `CandleSeries`: `btc.ema(50)`.
- Indicator registry public API in the `stdlib` package:
  ```go
  type IndicatorSpec struct {
      Name       string
      Positional []ParamType
      Kwargs     map[string]KwargSpec
      IsScalar   bool                   // true means callable on Series too
      Build      func(args) talive.Indicator
  }
  stdlib.RegisterIndicator(spec IndicatorSpec) error
  ```
- The registry is open: the private project can `RegisterIndicator(...)`
  its own indicators against tascript at launch.
- Wires EMA from talive as the first registered indicator.
- **Warmup phase:** parser enumerates every indicator call site, runtime
  computes `max(WarmUpPeriod)`, requests that many historical candles from
  the `DataSource`, feeds them through, then begins live `Run()`.
- Indicator-output `Series` go through the history wrapper from slice 4
  for `[n]` support.
- Memoisation by `(receiver, indicator_class, normalised_args)` (§5.1).

**Project's integration test:**

```js
btc = input("btc_feed")
function Init() {}
function Run() {
  if (btc.closes > btc.ema(50)) {
    emit("alerts", price=btc.closes, ema=btc.ema(50))
  }
}
```

---

### Slice 6 — Indicator surface fills in

**Goal:** the bulk of talive becomes reachable.

**Adds:**

- Scalar indicators: `rsi`, `sma`, `smma`, `wma`, `dema`, `tema` (all the
  ones implementing talive's `Scalar` interface).
- Non-scalar / multi-output: `macd`, `bb`, `atr`, `dmi` — return values
  become `Tuple`s.
- `Tuple` becomes a first-class value type.
- `[n]` on a `Tuple` is element access; on a `Series` it's history. The
  evaluator dispatches by type (§3.4 Indexing).
- Scalar indicators callable on `Series` (the chaining form
  `btc.rsi(14).sma(15)`).
- Reserved constants: `CLOSE`, `OPEN`, `HIGH`, `LOW`, `HL2`, `HLC3`, `SMA`,
  `EMA`, `SMMA`, `WMA`, `DEMA`, `TEMA`, `NONE`, `DAILY`, `WEEKLY`,
  `MONTHLY`, `QUARTERLY`, `YEARLY`, `ALL`.
- Indicator kwarg config: `source=HLC3`, `ma=SMMA`, etc. with
  parse-time validation against the registry spec.

**Project's integration test:** an MACD-cross or BB-breakout program (the
§8.2 / §8.3 examples).

---

### Slice 7 — Math + TA helpers, helper registry

**Goal:** cleaner programs, second public registry API.

**Adds:**

- Full `math.*`: `max`, `min`, `abs`, `sqrt`, `pow`, `floor`, `ceil`,
  `round`.
- Full `ta.*`: `crossover`, `crossunder`, `rising`, `falling`, `highest`,
  `lowest`.
- Each helper has a registry entry that declares its lookback contribution
  (§4.2), feeding static buffer sizing.
- Helper registry public API:
  ```go
  type HelperSpec struct {
      Namespace string
      Name      string
      Args      []ArgSpec
      Lookback  func(args) map[seriesArg]int   // per-Series-arg lookback
      Eval      func(args) Value
  }
  stdlib.RegisterHelper(spec HelperSpec) error
  ```
- The `ta` namespace identifier becomes reserved.
- Project can register its own helpers under custom namespaces (subject to
  reserved-name rules).

---

### Slice 8 — Multi-input

**Goal:** cross-asset logic in one program.

**Adds:**

- Multiple `input(...)` declarations.
- `INPUT_DUPLICATE` parse error for repeated slot names.
- Wiring: `Launch` accepts a map `slot_name → DataSource`. Missing wirings
  produce `INPUT_NOT_WIRED` at launch.
- The synchronizer that decides when `Run()` fires under multiple feeds
  lives **in the private project**, not in tascript. tascript exposes a
  simple "drive one tick now" entry point; the project wraps it.

**Project's integration test:** the §8.5 divergence example.

---

### Slice 9 — Time and Duration

**Goal:** time-aware filters and cooldowns.

**Adds:**

- `Time` and `Duration` value types.
- `Candle.ts`, `CandleSeries.timestamps[n]`.
- Time properties: `.year`, `.month`, `.day`, `.weekday`, `.hour`,
  `.minute`, `.second`, `.millisecond`, `.unix_ms`.
- Duration arithmetic per §3.5.
- The `time` namespace identifier and its constants (`time.MILLISECOND`
  through `time.WEEK`).

**Project's integration test:** the §8.3 BB-breakout-with-cooldown and
§8.4 weekday-filter examples.

---

### Slice 10 — Diagnostics polish + resource limits

**Goal:** production-quality error output and enforced caps.

**Adds:**

- Rust/Elm-style error rendering: file path, line, column, source line
  with caret highlight (§6.2).
- All §7 resource limits enforced.
- All §6.4 / §7 category codes wired through.
- Negative-sample test suite covering every category code.

---

### Stretch — beyond slice 10

Possibilities, ordered by likely value:

- User-defined functions (helper functions, not multiple Init/Run).
- `return` statement for early exit in `Run()` (surfaced as a gap in §8).
- Time-zone conversion (`time.in("America/New_York")`).
- String formatting (template literals, JS-style — direction noted in
  §5.3).
- More indicators (Anchored — VWAP, Pivot, ADR — once anchor wiring is
  designed).
- Type-level integer (`Int`) for indicator parameters and history bounds
  (the migration is intentionally cheap — see §3.4 numeric validation).

---

## Test corpus

The corpus grows alongside the slices. Layout:

```
tascript/
└── testdata/
    ├── slice0/
    │   ├── positive/
    │   │   └── greeting.tas + events.json
    │   └── negative/
    │       └── missing_init.tas + diagnostics.json
    ├── slice1/
    │   ├── positive/
    │   │   └── close_pipe.tas + feed.csv + events.json
    │   └── ...
    └── ...
```

Each slice's CI gate runs all slices' tests, ensuring earlier-slice
programs never regress.

## What to build first

Start **Slice 0**. Hours of work. Zero external dependencies. Drops a real
public Go API the project can integrate against. From there, slice-by-slice.
