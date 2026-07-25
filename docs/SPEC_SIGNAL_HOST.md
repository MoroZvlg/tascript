# tascript — Signal-Block Host Specification

> Status: **design, not yet implemented.** This is the **reference host**: the vocabulary a
> *signal block* registers on top of the domain-blind core (`SPEC.md`). It is one host among
> future ones (filter, trading, SL/TP). Nothing here is core language — a trading block
> reuses `SPEC.md` and registers none of it.
>
> The declaration/stepping model and its rationale live in
> `DESIGN_STREAMING_INDICATORS.md`; this spec is the surface reference. Where they differ,
> the design doc wins until this is locked.
>
> **Supersedes the pre-rework method-call surface** (`btc.rsi(14)` as an expression).
> Indicators are now **declared** persistent slots (§3), read/stepped explicitly.

## 1. What this host adds

A signal block consumes candle (and numeric) streams, computes streaming technical-analysis
indicators, and emits signal events. Embedding the core, it registers:

- **Value / stream types** — `CandleSeries`, `Series`, `Candle` (§2).
- **Declaration keywords** — `indicator`, `series` (§3), over the core declaration-form
  shape (`SPEC.md` §4.6).
- **A type registry entry per indicator** — mapping a constructor (`ta.rsi(…)`) to a talive
  instance, with capability + effect metadata.
- **The `ta` helper namespace** (§5).
- **History/warmup/clock machinery** (§6) — all host-side; the core has no such concept.

## 2. Host value types

Registered into the core type registry (`SPEC.md` §4.4), declaring core capabilities:

| Type | Capabilities | Notes |
|------|--------------|-------|
| `CandleSeries` | `historyable`, `has-methods` | A stream of OHLCV candles. `[n]` reads the `Candle` `n` activations ago. Plural numeric projections yield a `Series`: `.opens`, `.highs`, `.lows`, `.closes`, `.volumes`, plus derived `.hl2`, `.hlc3`; `.timestamps` yields a historyable `Time` stream. Not user-constructable — injected by the runtime, one per declared input. |
| `Candle` | `has-methods` | A single candle. Singular access: `.open`, `.high`, `.low`, `.close`, `.volume`, `.ts` (a `Time`). |
| `Series` | `historyable`, `has-methods` | An ordered numeric stream supporting `[n]`. Sources: candle projections (`.closes`), indicator outputs, names bound to such. Not user-constructable. |

`cs[1].close` ≡ `cs.closes[1]`.

**Multi-output indicators** (MACD, Bollinger, DMI) return a **`Tuple`** — a core type that
is *postponed* (`SPEC.md` §3.4). Until core tuples land, multi-output indicator access is a
host-provided indexing on the indicator slot (e.g. `macd.line` / `macd[0]`); the exact form
is locked with the declaration model.

### Series lift rule (host semantics)

A `Series` in a scalar context auto-evaluates to its current value (≡ `s[0]`): comparisons,
arithmetic, call arguments, `emit(...)` kwargs. Arithmetic does not construct a derived
`Series` in this revision — `a.closes - b.closes` is a number. Series-producing arithmetic
may arrive later as explicit `series` declarations (§3) or methods.

## 3. Declarations — `indicator` and `series`

Indicators are **named persistent slots**, not expression calls. Identity is the declared
name (no call-site keying). Parameters are evaluated once at load and frozen.

```js
input btc: CandleSeries

series btc_src = math.pow(btc.close, 2)   // named derived stream (historyable)
indicator rsi   = ta.rsi(14) on btc_src   // persistent host-object slot
indicator smooth = ta.sma(20) on rsi      // chaining = declare over another slot

function Run() {
  if (smooth > 30 && rsi > 70 && rsi[0] > rsi[1]) {
    emit(alerts, kind="hot", rsi=rsi)
  }
}
```

- **`series name = <expr>`** — a named derived stream. Exists so history (`[n]`) and
  chaining sources have explicit, statically-known identity (no synthetic identity for
  `(btc.close*2)[1]`). Read-only, historyable.
- **`indicator name = <ctor> [on <source>] [manual|sparse]`** — a persistent slot holding a
  talive instance. `<ctor>` (`ta.rsi(14)`) is a lifetime-bearing constructor, legal only in
  a declaration (never inside `Run`). Params frozen at load.

### Stepping

- **`on <source>`** (default): the runtime advances the indicator **exactly once per
  activation, in dependency order, before `Run()`**. In `Run()` you only *read* (`rsi`,
  `rsi[1]`). Covers chaining + math (push math into a `series`).
- **`manual` + `step`** (escape hatch, source depends on a `Run`-local value): stepping is a
  **statement**, so the resolver enforces *stepped exactly once on every path, no
  read-before-step, no double-step*:
  ```js
  indicator custom = ta.rsi(14) manual
  function Run() {
    let src = compute_from_runtime(...)
    let r = step custom with src
  }
  ```
- **`sparse`**: opt-in uneven feeding; loses warmup guarantees; the script must gate on
  readiness.

> A stateful `.next()`-as-expression method with only a warning is rejected — silent
> sampling-frequency drift is a correctness bug. Enforcement rides on effect metadata
> (`SPEC.md` §4.6): the step method is `mutates-receiver`, cardinality once-per-tick,
> receiver ownership engine-owned-slot-only.

## 4. Indicator catalog

Each indicator has a registry entry mapping a DSL constructor to a talive constructor +
builder chain:

```
rsi:
  positional: [period: Integer(positive)]
  kwargs:     { source: SourceConst = CLOSE, ma: MaConst = SMMA, ... }
  build:      NewRSI(period).WithSource(...).WithMA(...)
```

Adding configuration as talive evolves means adding registry fields — no core change.

**Two classes** (mirroring talive's `Scalar`):
- **Scalar indicators** — single output, composable on any numeric source: `sma`, `ema`,
  `smma`, `wma`, `dema`, `tema`, `rsi`, `cci`, …
- **Non-Scalar indicators** — need multiple candle fields (`atr`, `dmi`) or produce multiple
  outputs (`macd`, `bb`, `ichimoku`). Their source must be a `CandleSeries`.

**Configuration constants** are reserved identifiers in this host (reassignment is a
parse-time error):

| Category | Reserved identifiers |
|----------|----------------------|
| Source | `CLOSE`, `OPEN`, `HIGH`, `LOW`, `HL2`, `HLC3` |
| MA type | `SMA`, `EMA`, `SMMA`, `WMA`, `DEMA`, `TEMA` |
| Anchor | `NONE`, `DAILY`, `WEEKLY`, `MONTHLY`, `QUARTERLY`, `YEARLY` |

## 5. The `ta` helper namespace

Free-function TA helpers. `ta` is a reserved namespace identifier in this host (a passive
syntactic prefix, not a value).

| Call | Behaviour |
|------|-----------|
| `ta.crossover(a, b)` | `true` on the activation where `a` crosses above `b` |
| `ta.crossunder(a, b)` | `true` on the activation where `a` crosses below `b` |
| `ta.rising(s, n)` | `true` when `s` strictly rose for the last `n` |
| `ta.falling(s, n)` | `true` when `s` strictly fell for the last `n` |
| `ta.highest(s, n)` | maximum of `s` over the last `n` |
| `ta.lowest(s, n)` | minimum of `s` over the last `n` |

`s` accepts any `Series`; `a`/`b` accept a `Series` or a number on one side.

## 6. History sizing, warmup, and clocks (host-side)

### History window sizing

The core owns the `[n]` window (`SPEC.md` §4.2); this host contributes the lookback sizing.
For each historyable slot, the analyser takes the max lookback across all references:

1. **Explicit literal indices** — `btc.closes[5]` contributes 5.
2. **Helper-signature lookback** — each `ta` helper declares its lookback:

   | Helper | Contribution |
   |--------|--------------|
   | `ta.crossover(a,b)` / `ta.crossunder(a,b)` | 1 on each side |
   | `ta.rising(s,n)` / `ta.falling(s,n)` | `n` on `s` |
   | `ta.highest(s,n)` / `ta.lowest(s,n)` | `n-1` on `s` |

Window size = `max(literal ∪ helper contributions) + 1`. The `n` in `s[n]` and helper
lookbacks must be literal integers (core constraint, `SPEC.md` §4.2). Bound cap:
`HISTORY_LIMIT` (`SPEC.md` §7).

### Warmup (host-side, language-unaware)

Streaming indicators need N activations before output is reliable (`IdlePeriod`). Warmup is
entirely the host's job — the core has no warmup concept. Two mechanisms, kept distinct:

- **Prefeed** — advance indicators/history over historical data *without* running user logic
  (safe default; never mutates `state` on cold values).
- **Strategy replay** — run `Run()` over historical activations with `emit` suppressed
  (warms `state` too, but on possibly-cold indicator values — opt-in).

Readiness is **graph propagation**, not a flat max:
`ready(chained) = ready(source) + own warmup`. A script may read an
`isReady()`-style value if it wants to gate.

### Clocks / multi-input cadence

Each input stream has its own event timeline. With several inputs, an activation may advance
only some; the frame marks which advanced (per-input epoch/changed flag) so a non-advanced
input does not re-step an `on`-driven indicator. *When* `Run()` fires is a host sync
primitive (`SPEC.md` §4.1), not part of the DSL. Single-stream signal blocks don't need this
yet.

## 7. Host diagnostic codes

Extends the core code table (`SPEC.md` §6.4):

| Category | Phase | When |
|----------|-------|------|
| `INDICATOR_PARAM` | parse / runtime | Indicator parameter constraint violated (e.g. non-integer/non-positive period). |
| `SOURCE_KIND` | parse | Non-Scalar indicator given a `Series` source, or an invalid source projection. |
| `STEP_CARDINALITY` | parse | A `manual` indicator not stepped exactly once on every `Run()` path, read before step, or double-stepped. |

## 8. Examples

### 8.1 RSI oversold in an uptrend, with a cooldown

```js
const COOLDOWN_BARS = 20

input btc: CandleSeries

output alerts: {
  kind: String,
  price: Number,
  rsi: Number
}

state cooldown: Integer = 0

indicator ema_fast = ta.ema(50) on btc.closes
indicator ema_slow = ta.ema(200) on btc.closes
indicator rsi      = ta.rsi(14) on btc.closes

function Run() {
  state.cooldown = math.max(0, state.cooldown - 1)

  uptrend = ema_fast > ema_slow
  crossed = ta.crossunder(rsi, 30)

  if (uptrend && crossed && state.cooldown == 0) {
    emit(alerts, kind = "rsi_oversold_uptrend", price = btc.closes[0], rsi = rsi)
    state.cooldown = COOLDOWN_BARS
  }
}
```

### 8.2 MACD bullish crossover (multi-output indicator)

```js
input btc: CandleSeries

output alerts: { kind: String, line: Number, signal: Number }

indicator macd = ta.macd(12, 26, 9) on btc.closes

function Run() {
  if (ta.crossover(macd.line, macd.signal)) {
    emit(alerts, kind = "macd_bullish_cross", line = macd.line, signal = macd.signal)
  }
}
```

### 8.3 Cross-asset divergence with a time cooldown

Two inputs, a derived `series`, `Time`/`Duration`, `state.*` holding a `Time`.

```js
const SPREAD_THRESHOLD = 0.05
const COOLDOWN         = 15 * time.MINUTE

input btc: CandleSeries
input eth: CandleSeries

output alerts: { kind: String, divergence: Number }

state last_alert: Time

series btc_change = (btc.closes - btc.closes[1]) / btc.closes[1]
series eth_change = (eth.closes - eth.closes[1]) / eth.closes[1]

function Init() {
  state.last_alert = btc.timestamps[0] - time.HOUR
}

function Run() {
  divergence  = math.abs(btc_change - eth_change)
  cooled_down = btc.timestamps[0] - state.last_alert > COOLDOWN

  if (divergence > SPREAD_THRESHOLD && cooled_down) {
    emit(alerts, kind = "btc_eth_divergence", divergence = divergence)
    state.last_alert = btc.timestamps[0]
  }
}
```

> These examples use the target declaration surface. Details still open (multi-output access
> form, `series`-vs-local aliasing for lookback attribution, whether `btc.close` vs
> `btc.closes` is the projection spelling) are tracked in `DESIGN_STREAMING_INDICATORS.md`.
