# Design rationale & decision log

> **Not normative.** The authoritative surface lives in **`SPEC.md`** (core language) and
> **`SPEC_SIGNAL_HOST.md`** (the signal-block host). The work queue lives in
> **`DESIGN_TODO.md`**. This doc keeps only what those deliberately omit: the **why** behind
> the design, the **alternatives we rejected** (with reasons), and the **decisions still
> open** — so a future session doesn't re-derive or re-litigate them.
>
> (Filename is historical — this is no longer indicator-specific; it's the architecture
> decision log.)

## Why the core must be general-purpose (the product)

tascript is **not** a TA-DSL. The product is a visual node editor (Miro-like whiteboard):
users drag **blocks** and connect them with arrows; events flow along the arrows.

- **Input blocks** — streaming sources (candles, ticker, funding, order/balance updates, …).
- **Logic blocks** — tascript programs, of many *types*: signal, filter, trading
  (place/cancel order), entry → SL/TP, …
- **Sink blocks** — telegram/slack/webhook, **or** another logic block, **or** a trade action.

Users swap and share block *logic* (marketplace, GitHub). So tascript is the **common
language across all block types**, and **each block type is a different host** embedding the
same core with a different registered vocabulary. A trading block reuses the core with
**zero** TA in it. That is why the core is domain-blind — general-purpose is the product
architecture, not speculative generality.

(Full architecture is in memory: `project_block_graph_architecture`.)

## Key decisions and the reasoning behind them

The *what* is in the specs; here is *why* each call was made.

- **Per-event execution, not per-candle** (`SPEC.md` §4.1). "Once per candle" was TA
  framing. A block is a dataflow node / actor with a mailbox — a message on a port is an
  activation. Candle tick = the signal block's special case. This is where `ergo` fits:
  one actor *per block*, not per indicator.
- **Two orthogonal axes for the type system** (`SPEC.md` §4.4–4.5). Type *capabilities*
  (per type: comparable/arithmetic/historyable/replaceable/…) are separate from *slot
  policy* (per binding: read-only/assignable/fixed). Assignability = `slotWritable ∧
  typeReplaceable`. **Why:** "safety falls out of the type alone" was wrong — `input price:
  Float` must stay read-only though `Float` is replaceable, so binding kind survives as slot
  policy. Conflating the two axes was the bug.
- **Closed capabilities, open types** (`SPEC.md` §4.4). A host can claim the core `+`, never
  invent an operator. Keeps operators fixed-meaning and the language small.
- **`state` is a prelude registration, not a hardcoded keyword** (`SPEC.md` §4.6). It's a
  persistent slot with an *assignable* policy — the same substrate future host declarations
  use. **Why:** unifies `state` with `indicator`/future keywords by construction, with no
  loss (a standard prelude ships it so no host reinvents it), and safety lives in the closed
  slot-policy/capability core, not in the keyword.
- **Indicators are declared, not called** (`SPEC_SIGNAL_HOST.md` §3). `indicator rsi =
  ta.rsi(14)` — identity is the declared name, params frozen at load. **Why:** modeling an
  indicator as a pure call (`ta.rsi(14)`) is an abstraction lie that hides a stateful object
  and forces call-site keying; declaring it makes the object honest and the warmup set
  statically knowable.
- **Storage unified, declaration surface distinct.** `state`/`series`/`indicator` share one
  persistent-slot substrate but stay separate keywords — different contracts (assignability,
  clock, warmup, rollback). Full surface collapse was rejected (see below).

## Rejected alternatives (do not revisit casually)

- **Method-call indicator surface** (`btc.rsi(14)` as an expression). Reads worse for scalar
  sources + chaining, and hides lifecycle. Replaced by declarations. (This is what the
  pre-rework `SPEC.md` had.)
- **AST call-site identity** (React-hooks / Pine style — key a stateful instance by its
  position). Works, but it's a costume worn over a pure-function surface, and drags in the
  "rules of hooks" foot-guns. Named declarations make identity explicit; call-site keying
  rejected for v1.
- **Model Y — declared-source auto-step as the *only* mode** (runtime steps every indicator
  from a captured source expression; `Run` only reads). Elegant and safe, but its source can
  only see inputs/state/other-indicators, not `Run`-local values — so chaining + arbitrary
  math between indicators can't live in `Run`. Kept as the `on` mode; added `manual`/`step`
  for the runtime-source case.
- **`.next(x)` as an ordinary expression method with only a warning.** Silent
  sampling-frequency drift (stepping inside an `if`) is a correctness bug in a signal DSL,
  not style. Stepping is a checked `step` statement instead (cardinality enforced).
- **Full surface collapse** (`persistent name = …`, one keyword for everything). Erases the
  real contract differences between a reassignable value slot and a stateful process.
  Rejected in favor of distinct keywords over a shared substrate.
- **Dynamic indicator parameters** / **runtime construction of lifetime-bearing objects in
  `Run`**. Params are frozen at load; construction is top-level-only. A fresh instance per
  tick never warms up.
- **Flat `max(warmupPeriod)` warmup.** Wrong for chains — warmup propagates through the
  graph (`ready(chained) = ready(source) + own warmup`).
- **`[n]` on arbitrary local scalars** (`(btc.close*2)[1]`). Would need synthetic identity;
  require a named `series` instead. History is only on named historyable slots.

## Codex stress-test (2026-07-24)

A second reviewer, grounded on the actual codebase, forced four corrections now baked into
the specs: (1) `.next()`-as-expression+warning is unsafe → `step` statement; (2) warmup is
graph propagation, not a flat max; (3) safety is slot-policy ∩ type-capability, not type
alone; (4) multi-input needs clocks/epochs in the frame. It also over-reached by pulling TA
concepts (`series`/`step`/warmup/clocks) into the language — corrected by scoping them to
the signal host.

## Open decisions

1. **Config-driven N indicators** → load-time **keyed families** (`indicator rsi[symbol] =
   … on market[symbol].close`, key set fixed at load, redeploy to change). Still named, not
   call-site. Confirm this covers the per-symbol case.
2. **Warmup default:** prefeed-only (safe) vs opt-in strategy-replay. Leaning prefeed.
3. Whether `series` + `on` really covers common chaining before committing `manual`/`step`
   as merely an escape hatch.
4. Declaration keywords: 3 fixed forms (`state`/`series`/`indicator`) vs a small open set —
   leaning: core defines the *slot policies*, hosts alias keywords onto them.
5. Multi-clock synchronizer policy for async multi-input blocks (host-owned; the DSL only
   needs the epoch/clock surface).
6. Signal-host surface details still unlocked: multi-output indicator access form
   (`macd.line` vs `macd[0]`), `series`-vs-local aliasing for lookback attribution,
   `btc.close` vs `btc.closes` projection spelling.

## Where the rest went

- Core language surface → `SPEC.md`.
- Signal-block host surface (indicators, `ta`, warmup/clocks, examples) → `SPEC_SIGNAL_HOST.md`.
- Concrete next work items + risks → the "Architecture rework" banner in `DESIGN_TODO.md`.
