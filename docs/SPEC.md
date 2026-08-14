# tascript — Core Language Specification

> Status: **draft / in design**. Sections fill in as decisions lock.
>
> **Scope: this is the CORE language — it is domain-blind.** It knows nothing about
> candles, indicators, orders, or any trading concept. Domain vocabulary is supplied by a
> **host** (one per block type). The reference host — candles + technical-analysis
> indicators — is specified separately in **`SPEC_SIGNAL_HOST.md`**. See §4.6 for the
> host extension mechanism and the core/host boundary.

## 1. Purpose

tascript is a small, general-purpose language for **per-event block logic**. A program is
the logic inside one *block* on a dataflow graph: it declares typed **input ports**,
reacts when data arrives, holds **persistent state** across activations, and **emits**
structured events to **output ports** wired to other blocks or sinks.

Model: **inputs → logic + state → emitted events.**

The core language knows only primitives (numbers, booleans, strings, time), control flow,
ports, state, and a type/capability system through which a host injects domain types and
functions. It does not know what a candle, an indicator, an exchange order, or a Telegram
message is — those live in hosts and delivery layers outside the language. The same core
runs a technical-analysis signal block, a signal-filter block, an order-placement block,
or a stop-loss block; only the host vocabulary differs.

## 2. Event Output

The output of every tascript program is a stream of events emitted through declared
**output ports**. An event has the shape:

```
{
  output: string,        // declared output port name
  ts:     timestamp,     // activation timestamp the event was produced at
  value:  String | null, // present for value outputs (String); null for structured
  data:   { ...fields }  // arbitrary user-defined payload fields
}
```

A program may emit zero or more events per activation, and may emit to different declared
outputs within the same program. Emission is performed via the built-in runtime action
`emit(...)` — see §5.2 for the full signature.

Inputs and outputs are both declared in the script, but wired to real blocks outside the
DSL:

```js
input metric: Float

output alerts: {
  level: String,
  value: Float
}
output logs: String
```

The host runtime / UI / deployment manifest maps `metric`, `alerts`, and `logs` to
concrete blocks. The language itself knows nothing about the sources, transports, formats,
or credentials behind them.

**Why this shape:**
- The script stays self-describing; port names are not magic external names.
- Runtime wiring stays outside the DSL, matching the block-based product model.
- `input` declarations are static dependencies; `emit(...)` remains runtime behaviour.
- The compiler can validate port names and output payload schemas.

**Out of scope for the DSL:**
- Delivery destinations (Telegram, Slack, webhooks, …)
- Output formatting (Markdown, JSON, plain text, …)
- Secrets, credentials, transport configuration
- Routing / fan-out rules

These belong to a separate routing / delivery layer that consumes the event stream.

## 3. Grammar

### 3.1 Surface style (locked decisions)

tascript leans on a JavaScript-flavoured surface syntax with deliberate deviations:

- **No statement terminators.** Newlines end statements; `;` is not used. The lexer
  suppresses newlines inside an open `(` or `[`, so long calls may span multiple lines, and
  before an `else`, so the keyword may start its own line.
  Inline type schemas (`{field: Type, ...}`) may also span lines — the parser skips newlines
  while reading them — but `{` **blocks** keep newlines significant, since there they are the
  statement separators. Broader trailing-token continuation is deferred (see §8 gaps).
- **C-style blocks.** `if (cond) { ... } else { ... }` — parentheses around conditions,
  braces around bodies. `else` may also start its own line (Allman style).
- **C-style logical operators.** `&&`, `||`, `!` (not `and`, `or`, `not`).
- **Trailing comma in comma-separated lists.** Both call arguments (`f(a, key=b,)`) and
  inline type schemas (`{a: Integer,}`) accept one.
- **Function-local bindings use `let`.** Inside `Init()` and `Run()`, `let x = a > b`
  creates a binding that lives **only for the current invocation** of that function and is
  dropped when it returns. There is no `var`, and `const` is not valid inside function
  bodies.
- **Top-level constants use `const`.** `const` is a declaration keyword only at the top
  level.
- **Persistent state is declared and namespaced.** Values that must survive across
  activations are declared at the top level (`state cooldown: Integer = 0`) and accessed
  via dot on the reserved `state` object: `state.cooldown = 20`. Dot access only. See §4.3.
  (The state surface may later unify with other persistent declarations — see the host
  extension model, §4.6 — but the current revision keeps the reserved `state` object.)
- **History reference.** For a type that declares the `historyable` capability (§4.2),
  `x[n]` reads the value `n` activations ago; `x[0]` (or just `x`) is the current value.

### 3.2 Program structure

Every tascript program is composed of:

1. **Zero or more top-level constants.** `const THRESHOLD = 0.5` declares a *module
   constant*. The right-hand side is evaluated **once at program load**. Readable from
   `Init()` and `Run()` by name; **reassignment from inside any function is a parse-time
   error**. By convention, names use `UPPER_SNAKE_CASE`.

2. **One or more input declarations.** `input name: Type` declares a runtime-wired input
   port. The binding is a read-only top-level value with the declared type. See §3.3.

3. **One or more output declarations.** `output name: Type` or `output name: { ... }`
   declares a runtime-wired output port.

4. **Zero or more state declarations.** `state name: Type` or `state name: Type =
   <const-expr>` declares a typed persistent entry, read and written as `state.name` from
   inside `Init()` and `Run()`. The optional initializer is restricted to the same
   expression domain as a `const` right-hand side and is evaluated once at load, after
   constants. An entry with no initializer **must** be definitely assigned inside `Init()`.
   See §4.3.

5. **An optional `function Init() { ... }`.** Runs **exactly once** before the first
   activation. Intended for seeding `state.*` entries whose initial value depends on input
   data. If omitted, behaves as an empty `Init`.

6. **A required `function Run() { ... }`.** Runs **once per activation** (see §4.1). This
   is where conditions are evaluated and `emit(...)` calls are produced.

Only `Run` is mandatory. No other top-level forms are permitted in the current revision;
user-defined helper functions may be added later.

Canonical program shape:

```js
const THRESHOLD = 100

input metric: Float

output alerts: {
  level: String,
  value: Float
}

state cooldown: Integer = 0

function Run() {
  state.cooldown = math.max(0, state.cooldown - 1)

  if (metric > THRESHOLD && state.cooldown == 0) {
    emit(alerts, level = "high", value = metric)
    state.cooldown = 20
  }
}
```

### 3.3 Ports — dependency-injected block IO

The DSL does not name sources, symbols, timeframes, chats, or webhook URLs. Every program
declares named **ports** at the top level; the actual blocks attached to those ports are
configured **outside** the DSL by the runtime / UI / deployment manifest.

Input and output declarations are not normal function calls. They are static port
declarations used for validation, tooling, and runtime wiring.

#### Input declarations

```js
input <name>: <InputType>
```

`<InputType>` is any type registered by the host (§4.6), plus the core value types. The
core defines the scalar value types (§3.4); **concrete streaming/domain input types (e.g.
`CandleSeries`) are host-supplied** — see `SPEC_SIGNAL_HOST.md`. The declared name becomes
a read-only top-level value.

#### Output declarations

An output is **either** a value output or a structured output. The form after `:`
determines which: a value type emits a single `value`; an anonymous `{ … }` schema emits
structured `data` fields. Combining both is not allowed.

```js
output <name>: <ValueType>              // value output — emit a single value
output <name>: { field: Type, ... }     // structured output — emit keyword fields
```

`<ValueType>` is a built-in value type. Current revision allows `String` and numeric
types; structured `data` fields may be any value type. A value output sets `value` and
leaves `data` empty; a structured output leaves `value` `null` and fills `data`.

Output names are not readable values and cannot be assigned, passed around, or called.
They are valid only as the first argument to `emit(...)` inside `Run()`.

#### Port rules

- Port declarations may appear **only at the top level**.
- Port names are normal identifiers, not string literals.
- Input, output, constant, function, namespace, and reserved names occupy one top-level
  namespace. Duplicate `Init`/`Run` declarations are parse-time errors.
- A declaration over a name the prelude already binds — a namespace, a registered type
  name, `Init`, `Run`, `emit` — is `RESERVED_NAME`. A declaration over an earlier
  *script* declaration is `DUPLICATE_DECLARATION`.
- **Shadowing is not permitted.** A `let` may not reuse a name bound in any enclosing
  scope, top level included. Sibling blocks may each bind the same name.
- An input binding is **read-only**; reassignment inside a function is a parse-time error
  (this is a *slot policy*, §4.5).
- An output name is **emit-only**; reading or assigning it is a parse-time error.
- A declared input the host does not supply on an activation is a **runtime error**, and the
  activation is rejected before the body runs. The same holds for a value whose type does not
  match the declaration and cannot be coerced to it, and for a supplied name the script never
  declared.
- An `emit(...)` call targeting an undeclared output is a parse-time error.

**Out of scope for the current revision** (may be added later): user-tunable config inputs
(`input period: Integer = 14`); named reusable custom types (`type Alert { … }`).

### 3.4 Value types

The core scalar value types:

| Type       | Notes |
|------------|-------|
| `Integer`  | Signed integer. `14` is an `Integer`. |
| `Float`    | `float64`. `1.5` is a `Float`. `Integer` coerces to `Float`; `/` always returns `Float`. |
| `Bool`     | `true`, `false`. |
| `String`   | Double-quoted only: `"BTC/USDC"`. |
| `Null`     | A single bottom value `null`. Reserved for future static-nullable analysis — minimise direct use. Reading an unassigned `state.*` field is **not** null, it is a runtime error. |
| `Time`     | A point in time. See §3.5. |
| `Duration` | A length of time. See §3.5. |

> **The `Integer`/`Float` split is the shipped truth.** `Number` in older prose meant a single
> `float64` type; it is not a type name and does not resolve. Where this document writes
> "numeric", it means either `Integer` or `Float`, and `Integer → Float` coercion applies at
> operator, argument, and assignment boundaries.
>
> Residual sharp edges, all current behaviour: `/` always returns `Float`, even for two
> `Integer`s; `Integer` arithmetic **wraps silently** on overflow (`9223372036854775807 + 1`);
> and `-9223372036854775808` fails to parse, since `-` is an operator and the bare literal
> overflows on its own.

**Collection types (`Tuple`, arrays/slices) are core but POSTPONED.** They are planned
core language types (fixed-arity tuples from multi-return calls; indexable arrays). They
are **not built in the current revision** and their `[n]`/`[i]` access is defined by the
core index mechanism (§4.2) once they land. Until then, no core type is indexable.

**Domain value types (candles, series, etc.) are host-supplied** — see
`SPEC_SIGNAL_HOST.md`.

### 3.5 Time and Duration

`Time` and `Duration` are core value types, supplied by the built-in `time` prelude module
(§4.6). Their code is part of the core; a host may later disable or partially configure the
`time` module (e.g. a clock source), but in the current revision it is always available.

#### `Time`

A `Time` value represents a point in time. Sources:

- `time.from_unix_ms(ms)` — an explicit fixed timestamp from epoch milliseconds.
- Host-provided time values (e.g. an activation/event timestamp exposed by a host type).

There is no time-zone support in v1. All component access is in UTC.

**Properties (UTC):**

| Property      | Type    | Notes |
|---------------|---------|-------|
| `.unix_ms`    | Integer | total milliseconds since the Unix epoch |
| `.year`       | Integer | full year, e.g. 2026 |
| `.month`      | Integer | 1 through 12 |
| `.day`        | Integer | day of month, 1 through 31 |
| `.weekday`    | Integer | 0 through 6, **0 = Sunday** (JS convention) |
| `.hour`       | Integer | 0 through 23 |
| `.minute`     | Integer | 0 through 59 |
| `.second`     | Integer | 0 through 59 |
| `.millisecond`| Integer | 0 through 999 |

**Methods (UTC):**

| Method | Type | Notes |
|--------|------|-------|
| `.truncate(bucket: Duration)` | `Time` | Floors the timestamp to the start of the fixed UTC bucket, e.g. `t.truncate(time.DAY)`. `bucket` must be positive, no larger than `time.DAY`, and must evenly divide `time.DAY`; zero, negative, larger, or non-dividing buckets are runtime errors. Flooring is toward negative infinity, not toward zero. |

Valid truncate buckets include `90 * time.MINUTE`, `4 * time.HOUR`, `6 * time.HOUR`, and
`time.DAY`. `7 * time.HOUR` is invalid because `time.DAY` is not evenly divisible by seven
hours.

**No dedicated `Time` literal syntax in v1.** Fixed anchors are created with
`time.from_unix_ms(ms)`.

#### `Duration`

A `Duration` is a length of time, in milliseconds, signed. Produced by `Time - Time`, by
a numeric literal times a `time.*` constant, or by arithmetic on existing durations.

**Duration constants live in the `time.*` namespace** (`time` is a reserved namespace
identifier alongside `math`):

| Constant | Value |
|----------|-------|
| `time.MILLISECOND` | 1 ms |
| `time.SECOND`      | 1000 ms |
| `time.MINUTE`      | 60 × `time.SECOND` |
| `time.HOUR`        | 60 × `time.MINUTE` |
| `time.DAY`         | 24 × `time.HOUR` |
| `time.WEEK`        | 7 × `time.DAY` |

**Weekday constants also live in `time.*`:**

| Constant | Value |
|----------|-------|
| `time.SUNDAY` | 0 |
| `time.MONDAY` | 1 |
| `time.TUESDAY` | 2 |
| `time.WEDNESDAY` | 3 |
| `time.THURSDAY` | 4 |
| `time.FRIDAY` | 5 |
| `time.SATURDAY` | 6 |

No `time.MONTH` or `time.YEAR` — those are calendar-dependent, not fixed durations.
Although `time.WEEK` is a valid duration, `t.truncate(time.WEEK)` is not v1 (epoch-modulo
week truncation would anchor to Thursday 00:00 UTC).

The `time` namespace also hosts pure helpers:

| Helper | Type | Notes |
|--------|------|-------|
| `time.from_unix_ms(ms)` | `Time` | Creates a fixed timestamp from Unix epoch milliseconds. |

**Duration properties:**

| Property      | Type     | Notes |
|---------------|----------|-------|
| `.milliseconds` | Integer  | total milliseconds |
| `.seconds`    | Float    | total seconds (may be fractional) |
| `.minutes`    | Float    | total minutes |
| `.hours`      | Float    | total hours |
| `.days`       | Float    | total days |
| `.weeks`      | Float    | total weeks |
| `.abs`        | Duration | absolute duration |

#### Operator semantics

`Time`:

| Op | Result |
|----|--------|
| `Time - Time` | `Duration` |
| `Time + Duration` | `Time` |
| `Time - Duration` | `Time` |
| `Time < Time`, `<=`, `>`, `>=` | `Bool` |
| `Time == Time` | `Bool` (strict same-type equality) |
| `Time + Time` | parse-time error |
| `Time + <numeric>`, `Time - <numeric>` | parse-time error (use `Duration`) |

`Duration`:

| Op | Result |
|----|--------|
| `Duration + Duration` | `Duration` |
| `Duration - Duration` | `Duration` |
| `<numeric> * Duration` | `Duration` |
| `Duration * <numeric>` | `Duration` |
| `Duration / <numeric>` | `Duration` |
| `Duration / Duration` | `Float` (ratio) |
| `Duration < Duration` etc. | `Bool` |
| `Duration == Duration`, `!=` | `Bool` |
| unary `-Duration` | `Duration` |
| `Duration + <numeric>` | parse-time error |

`Duration / <numeric>` and `Duration / Duration` with a zero divisor are runtime errors.
`<numeric> * Duration`, `Duration * <numeric>`, and `Duration / <numeric>` round the resulting
milliseconds to the nearest integer, half away from zero. Invalid `Time.truncate(...)`
buckets are runtime errors following the failure protocol (§6.5).

Implementation note for the `Integer`/`Float` split: duration arithmetic with both numeric
types must be registered explicitly; infix resolution uses exact binary-rule lookup without
numeric coercions.

`Time` and `Duration` are first-class values: storable in `state.*`, passable in
`emit(...)` kwargs, assignable to per-call locals. At the host boundary, `Time` serialises
as Unix epoch milliseconds (`.unix_ms`) and `Duration` as signed milliseconds
(`.milliseconds`). State persistence must preserve the declared field type.

### 3.6 Operators, equality, and conditions

tascript rejects JavaScript's loose-equality / truthy-falsy semantics. Strict rules apply.

#### Equality (`==`, `!=`)

Equality requires the two operands to have the **same type**. Cross-type comparison is a
parse-time error when statically detectable, a runtime error otherwise. No silent `false`,
no implicit conversion.

| Operands | Result |
|----------|--------|
| numeric vs numeric | numeric equality (`Integer`/`Float` coerce) |
| `String` vs `String` | byte-equal |
| `Bool` vs `Bool` | identity |
| `Null` vs `Null` | `true` |
| `Null` vs anything else | error |
| any other cross-type pair | error |

#### Ordering (`<`, `<=`, `>`, `>=`)

Ordering is defined **only on numeric × numeric** (plus `Time`/`Duration` per §3.5). Any
other operand combination is a type error. String / `Bool` / `Null` ordering errors.

#### Boolean position

`if (cond) { ... }`, `&&`, `||`, and unary `!` all require `Bool` operands. A numeric
(including `0`), `String`, `Null`, or any non-`Bool` value in boolean position is a type
error. No truthy / falsy values.

```js
if (state.cooldown)     { … }   // error — cooldown is numeric
if (state.cooldown > 0) { … }   // ok
```

#### Operator precedence and associativity

Standard C/JS precedence. Tightest to loosest:

| Tier | Operators | Associativity |
|------|-----------|---------------|
| 1 | `()` grouping, `.` member access, `[]` indexing, function/method call | n/a |
| 2 | unary `!`, unary `-` | right |
| 3 | `*`, `/`, `%` | left |
| 4 | binary `+`, binary `-` | left |
| 5 | `<`, `<=`, `>`, `>=` | left |
| 6 | `==`, `!=` | left |
| 7 | `&&` | left |
| 8 | `\|\|` | left |

**Not present:** bitwise operators, ternary `? :`, exponentiation (`**` / `^` — use
`math.pow(x, y)`).

**Assignment (`=`) is a statement, not an expression.** `a = b = c` and `x = (y = 1)` are
parse-time errors.

#### Comments

Single-line only (`//`). No block-comment form.

#### Short-circuit evaluation

`&&` and `||` short-circuit; the RHS is not evaluated when the LHS already determines the
result.

#### Indexing operator `[n]`

`[n]` is a **core operator, dispatched by a type capability** (`indexable` / `historyable`,
§4.2). The operator and its syntax are core; a type opts in by declaring the capability and
registering an index rule. Semantics:

- A `historyable` type → `[n]` reads the value `n` activations ago; `[0]` is current.
- An `indexable` type (future core collections: tuple, array) → `[i]` reads the `i`-th
  element (0-based).

No core type is indexable in the current revision (collections postponed, §3.4). Host types
may declare the capability — e.g. a candle stream (`SPEC_SIGNAL_HOST.md`). Out-of-range
access is a runtime error (§6.5).

## 4. Semantics

### 4.1 Execution model — per-event

tascript uses a **per-event** execution model. A program is a dataflow node with a mailbox:
a message arriving on an input port is an **activation**. On each activation the runtime:

1. Reads every declared input from its host binding, established once before `Init`. The
   host guarantees the bound values are coherent — consistent as-of one instant — and
   immutable for the activation's duration.
2. Executes `Run()` top-to-bottom against those values.
3. Delivers each `emit(...)` to that output's host sink as it is evaluated, in program
   order, from inside the tick.
4. Commits state on success; rolls it back on a runtime failure (§6.5). Delivered emits
   are not recalled.

A "candle tick" is simply the special case where a signal block's main input is a candle
stream; the core has no candle concept.

#### Activation policy is a host concern

**When** `Run()` fires — on any input update, only when all inputs have advanced, or some
custom gate — is decided by **host-side sync primitives**, not the DSL. The core exposes
the ports and the activation hook; it never encodes the scheduling policy. tascript offers
no `primary(...)`, no `synchronize(...)`, no way to read or branch on cadence. Programs are
written to be correct under whichever cadence the operator configures — typically by
reading timestamps and using `state.*` to debounce.

### 4.2 History and the `[n]` operator

The `[n]` operator (§3.6) is the core mechanism for reading past values. Any type that
declares the `historyable` capability (§4.4) has a **runtime-owned ring-buffer window** of
its recent values; `x[n]` reads `n` activations ago, `x[0]` is current. The window is a
language-layer concern — the runtime advances it, snapshots pending samples, commits on tick
success, and discards on failure — never the host type's internal business.

**Buffer sizing.** How large each window must be is determined by static analysis of the
maximum lookback across all `[n]` reads (and any host helpers that consume history). In the
core, sizing considers explicit literal indices (`x[5]` contributes 5). Host libraries whose
functions consume history contribute their own lookback — that host-specific sizing analysis
is specified in the relevant host spec (e.g. indicator lookback in `SPEC_SIGNAL_HOST.md`).

**Literal-only constraint.** The `n` in `x[n]` must be a literal `Integer` at parse time.
Dynamic indices (`x[state.i]`) are rejected — this keeps static sizing tractable.

### 4.3 State

A program declares each persistent entry at the top level with a static type and
reads/writes it through the reserved `state` object:

```js
state cooldown: Integer = 0      // const initializer — no Init needed
state last_signal: Time          // no initializer — must be assigned in Init()
```

All `state.*` entries survive between activations; assignment and reads are legal anywhere
inside `Init()` and `Run()`, with no history — an entry is a plain persistent variable.

Static rules (enforced at analysis time; no runtime state errors):

- Access to an undeclared entry — read or write — is rejected (`STATE_UNDECLARED`).
- The initializer must be a constant-domain expression (literals, declared constants,
  module calls — inputs, outputs, and other state entries are not in scope) and must match
  the declared type (`Integer → Float` coercion applies).
- An entry with no initializer must be assigned inside `Init()`, otherwise the program is
  rejected (`STATE_UNINITIALIZED`). No silent zero / null defaults.
- Entry types are scalar in this revision — a record type is rejected at analysis time.

Initializers evaluate once at load, after constants, before `Init()`. The seeding
requirement is **definite assignment**: an entry without an initializer must be assigned on
*every* path through `Init()`. An assignment under a lone `if` does not count; an
`if`/`else` where both branches assign does. The analysis is conservative — it may
over-report `STATE_UNINITIALIZED` but never miss an unseeded path.

`let` bindings inside a function body are scoped to that single invocation and do not
persist.

**Relationship to the general model.** `state` is one instance of the general *persistent
slot* concept (§4.5): a slot whose policy is *assignable* and whose type is a core value
type. A future revision may unify the `state` surface with host-registered persistent
declarations (§4.6) over the same substrate; the reserved-object surface here is the current
revision's form.

### 4.4 Type capabilities

Every type — core or host-registered — declares which operations it supports, from a
**closed** core vocabulary. The set of *capabilities* is fixed by the core; the set of
*types* claiming them is open (§4.6). A host cannot invent an operator; it can only declare
that its type supports a core one.

Capabilities (closed set; grows only by core revision):

| Capability | Meaning |
|------------|---------|
| `comparable` | `==`, `!=` |
| `ordered` | `<`, `>`, `<=`, `>=` |
| `arithmetic` | `+`, `-`, `*`, `/`, `%` |
| `indexable` | random-access `[i]` (future core collections) |
| `historyable` | lookback `[n]` with a runtime-owned window (§4.2) |
| `has-methods` | named member calls, each with effect metadata (§4.6) |
| `replaceable` | a value of this type can be swapped out of a slot (see §4.5) |
| `snapshotable` | can participate in tick rollback; policy: value-copy / host-snapshot / none |

Capabilities are properties of the **type** (intrinsic). Whether a *binding* may be written
is a separate axis — the slot policy (§4.5). Conflating the two is a category error.

### 4.5 Slot policy and assignability

Each binding has a **slot policy** — a property of the *declaration*, independent of its
type:

| Slot policy | Bindings | Writable? |
|-------------|----------|-----------|
| read-only | `const`, `input` | no |
| assignable | `state` entries | yes (whole-value replace) |
| rebindable | `let` (function-local) | yes, within its invocation |
| fixed | persistent host-object slots (§4.6) | no reassignment; mutate via methods |

Assignability is the **intersection** of slot policy and type capability:

```
canWrite(binding) = slotWritable(binding.policy) AND typeReplaceable(binding.T)
```

`input price: Float` is not writable — `Float` is `replaceable` but the `input` slot is
read-only. A fixed host-object slot is not writable — its slot forbids reassignment and its
type is typically not `replaceable`. This is why assignability is not a property of the type
alone. **Safety invariant:** a script can never write *into* a host-owned value's internals
(`obj.field = x`, `arr[i] = x`) — no core type exposes a member/element lvalue target, and
host types must not either; host methods may mutate their own opaque internal state, but must
never expose a script-side lvalue.

**Readability** is the orthogonal axis, and a strictly smaller set: only `const`, `input`,
and `let` bindings carry a readable value. An output is emit-only, and namespaces,
registered type names, `Init`, `Run`, and `emit` occupy the namespace without denoting a
value at all — naming one in value position is `NOT_READABLE`, not a type error.

### 4.6 Standard prelude, and the host extension mechanism

**Irreducibly core** (never a registration): the syntax (literals, operators, `if`, `let`,
`function`, blocks, the declaration-form shape); the type/capability system (§4.4); slot
policies (§4.5); the persistent-slot + snapshot mechanism; the per-event execution model;
the registry itself.

**Standard prelude:** the always-available registrations bundled with the core — the value
types (§3.4), the `math` and `time` modules (§5.3, §3.5), and the `const`/`input`/`output`/
`state` declaration forms. Their code is part of the core, but built so a host may later
**disable** or **partially configure** them (e.g. a `time` clock source). In the current
revision the prelude is always on.

**Host extension.** A host (one per block type) specializes the core by registering:

1. **Types** — with their capability set (§4.4) and, per method, **effect metadata**
   (`pure` / `mutates-receiver` / `emits` / `reads-clock`, allowed phase `load|init|run`,
   receiver ownership `any-value|engine-owned-slot-only`).
2. **Constructors** — phased: *pure value constructors* (usable in `const`/`state`
   initializers) vs *lifetime-bearing constructors* (legal only in a persistent
   declaration, never inside `Run`).
3. **Declaration keywords** — contextual keywords over the **fixed** declaration-form shape
   `<kw> <name> [: Type] [= expr] [modifiers]`. The lexer stays dumb (emits `IDENT`); the
   parser dispatches on a declaration-form registry. Hosts register *words + semantics
   tags* (which slot policy, which phase), never new grammar. Operators stay closed.

The reference host — candles, `ta.*`, `indicator`/`series` declarations — is
`SPEC_SIGNAL_HOST.md`.

## 5. Standard Library (core)

### 5.1 Indicators — host, not core

Technical-analysis indicators are **not** part of the core language. They are the signal
block host's vocabulary — see `SPEC_SIGNAL_HOST.md`.

### 5.2 emit(...) — signal emission

```
emit(OUTPUT, arg [, arg]* [, ident=expr]*)
```

- `OUTPUT` is a declared output identifier, not a string literal.
- `emit(...)` is a built-in runtime action, valid **only inside `Run()`**. Use in `Init()`
  or at top level is `EMIT_OUTSIDE_RUN`.
- The payload obeys the **ordinary call-argument rules** — positional args bind in
  declaration order, keyword args bind by name, each parameter is filled exactly once, and
  positional args may not follow keyword args (`ARG_ORDER_INVALID`, `ARG_DUPLICATE`,
  `ARG_MISSING`, `ARG_UNKNOWN`, `ARG_COUNT_MISMATCH`). `emit` is not a special form here — a
  **structured** output takes one parameter per declared field, a **value** output takes a
  single parameter named `value`.
- `expr` must evaluate to a serialisable value: `Integer`, `Float`, `Bool`, `String`,
  `Null`, `Time`, or `Duration`. Passing a non-serialisable host value (collection, stream,
  opaque object) is a runtime error.

> **Positional payloads bind by field order.** Reordering the fields of a structured
> output's `{ … }` schema silently changes the meaning of every positional `emit` against
> it, with no diagnostic when the reordered types still match. Keyword form is immune;
> prefer it for schemas with more than one field of the same type.

```js
input metric: Float
output logs: String

function Run() {
  emit(logs, "threshold crossed")
}
```

```js
input metric: Float
output alert: { message: String, value: Float }

function Run() {
  emit(alert, message="threshold crossed", value=metric)
  emit(alert, "threshold crossed", metric)  // equivalent, binds by field order
}
```

A value output's single argument is likewise addressable either way — `emit(logs, "hi")`
and `emit(logs, value="hi")` are the same call.

There is no in-language string interpolation in v1.

**Schema enforcement.** A structured output's `{ … }` schema is **strict and closed**:
every declared field must be supplied; no undeclared fields; each value must match the
declared type. Violations are check-time errors when statically detectable, else runtime
errors before delivery.

**Output discovery.** Tooling can enumerate outputs from top-level `output` declarations.
Routing and fan-out are host concerns. Broadcast helpers (`emit(ALL, ...)`) are not v1.

### 5.3 Helpers — the `math` namespace (and `time`, §3.5)

Free-function helpers live in namespaces. Namespaces are **passive syntactic prefixes**,
not first-class values — `math` cannot be assigned, passed, or reflected on.

#### `math` — pure math

| Call | Behaviour |
|------|-----------|
| `math.E` | Euler's number |
| `math.PI` | pi |
| `math.max(a, b)` | larger of two numbers |
| `math.min(a, b)` | smaller of two numbers |
| `math.abs(x)` | absolute value |
| `math.sqrt(x)` | square root; `INVALID_ARGUMENT` if `x < 0` |
| `math.pow(x, y)` | x to the y |
| `math.floor(x)` | round toward −∞ |
| `math.ceil(x)` | round toward +∞ |
| `math.round(x)` | round to nearest integer, half toward +∞ |
| `math.trunc(x)` | integer part, toward zero |
| `math.sign(x)` | sign of number |
| `math.log(x)` | natural logarithm; `INVALID_ARGUMENT` if `x <= 0` |

`math` and `time` are reserved namespace identifiers. Declaring over one is `RESERVED_NAME`;
naming one in value position (`let m = math`) is `NOT_READABLE` — they resolve as prefixes
only, never as values. Host libraries add their own namespaces (e.g. `ta` — `SPEC_SIGNAL_HOST.md`).

### 5.4 String formatting — deferred

The v1 language offers no in-program string composition. Programs emit a string value or
structured fields; rendering is the output sink's responsibility. No template syntax is
reserved.

## 6. Diagnostics

### 6.1 Two phases

- **Parse-time** — before any activation. Examples: unknown identifier, reassignment of a
  reserved name, a port declaration inside a function, `emit(...)` outside `Run()`, `==`
  between mismatched types when statically detectable.
- **Runtime** — during `Init()` or `Run()`. Examples: history index out of range,
  cross-type comparison the analyser could not statically rule out, division by zero. How
  runtime failures abort and roll back is specified in §6.5.

### 6.2 Every error carries

- **Phase** — `parse` or `runtime`. Structured metadata only; not rendered.
- **Category code** — stable machine-readable identifier (§6.4).
- **Source location** — line, column. *Which* script is the host's identity to supply,
  not the diagnostic's.
- **Human-readable message** — may include a hint.

Rendered shape:

```
error[TYPE_MISMATCH] 2:19: expected Float, found String
error[DIVISION_BY_ZERO] 3:9: integer division by zero
```

Type mismatches read `expected X, found Y`. Runtime traps borrow their category code from
the runtime error kind, so both namespaces share the bracket. Two diagnostics render with no
source location: a recovered internal panic (no node is in hand once the stack has unwound)
and an undeclared input port (the offending key is supplied by the host and appears nowhere
in the script).

### 6.3 Parse-time policy

The compiler **collects multiple parse-time errors before aborting** (initial cap: 100).
Recovery uses statement-level resynchronisation.

### 6.4 Stable category codes

Errors carry a stable category code. The core set (host libraries add their own codes in
their specs):

| Category | Phase | When |
|----------|-------|------|
| `TYPE_MISMATCH` | parse / runtime | Operator or function applied to incompatible types. |
| `BOOL_REQUIRED` | parse / runtime | Non-`Bool` used in `if`, `&&`, `\|\|`, `!`. |
| `RESERVED_NAME` | check | Declaration over a prelude name — namespace, registered type, `Init`, `Run`, `emit` (§3.3). |
| `NOT_ASSIGNABLE` | parse | Write to a read-only or fixed slot, or a non-replaceable type (§4.5). |
| `NOT_READABLE` | check | A name that carries no value used in value position — output, namespace, type name, function name (§4.5). |
| `STATE_UNDECLARED` | parse | Read/write of a `state.*` entry with no top-level declaration. |
| `STATE_UNINITIALIZED` | parse | A `state` entry has no initializer and is not definitely assigned in `Init()`. |
| `HISTORY_OUT_OF_RANGE` | runtime | `x[n]` where insufficient history. |
| `INPUT_MISSING` | runtime | A declared input port was not supplied for this activation. |
| `INPUT_UNKNOWN` | runtime | The host supplied a name the script does not declare. Renders with no source location: the name appears nowhere in the script. |
| `INPUT_TYPE_MISMATCH` | runtime | A supplied input value does not match the declared port type and cannot be coerced to it. |
| `OUTPUT_NOT_WIRED` | runtime | A declared output port has no destination block. |
| `PORT_DUPLICATE` | parse | Two top-level ports/bindings declare the same name. |
| `UNKNOWN_OUTPUT` | parse | `emit(...)` targets a non-declared output. |
| `EMIT_OUTSIDE_RUN` | check | `emit(...)` outside `function Run()`. |
| `EMIT_PAYLOAD` | check / runtime | Emitted value/kwargs do not match the output declaration. |
| `EMIT_NOT_EXPRESSION` | parse | `emit(...)` used in expression position — it is a statement. |
| `UNDEFINED_FUNC` | parse | A bare call `foo(...)`; only module methods are callable. |
| `NOT_CALLABLE` | parse | Call on an expression that is not a callable form. |
| `NOT_INDEXABLE` | parse | `a[i]` on a type with no index rule (today: every type). |
| `ARG_DUPLICATE` | parse | One parameter filled twice — positionally and by keyword, or by two keywords. |
| `ARG_UNKNOWN` | parse | Keyword argument that names no parameter of the callee. |
| `TOP_LEVEL_FORM` | parse | A construct not permitted at the top level (e.g. a `state.*` assignment, `if`). |
| `TOP_DECL_MISPLACED` | parse | A top-level declaration used inside a function body. |
| `MISSING_RUN` | parse | Program does not declare `function Run()`. |

Future categories are additive; existing codes never change meaning.

### 6.5 Runtime failure protocol

Runtime errors come in two kinds, distinguished by whether the **program** or the
**interpreter** is at fault.

#### Script traps

A script trap is a failure a correct interpreter hits on otherwise legal input — division
by zero, an invalid `Time.truncate(...)` bucket, history index out of range. A trap carries
a stable machine-readable **kind**, a source position, and a message:

| Kind | When |
|------|------|
| `DIVISION_BY_ZERO` | `/` or `%` with a zero divisor — integer, float, or `Duration`. |
| `INVALID_ARGUMENT` | A rule rejected a well-typed argument (e.g. a bad `Time.truncate` bucket). |
| `OUT_OF_RANGE` | Index outside the valid range (history `x[n]`, future tuple `t[i]`). |
| `UNASSIGNED_STATE` | Read of a `state.*` field that reached `Run()` unset. Unreachable in a program that passes definite-assignment analysis; retained as a fail-loud backstop. |
| `UNKNOWN_FAILURE` | A host-registered rule returned a non-registry error; wrapped verbatim. |

**Tick transactionality.** A script trap aborts the current invocation:

- A trapped `Run()` tick **rolls back `state.*`** to its pre-tick snapshot. The host decides
  whether to continue.
- **Emits are not rolled back.** Each `emit` reaches its sink as it is evaluated, so a trap
  cannot recall what already went out — a tick that traps halfway has emitted its first half.
  Whether that is acceptable is the sink's policy: a `log` output wants every line
  regardless of the outcome, an order router may prefer to buffer and flush only after
  `Run` returns cleanly. The engine takes no position and does not buffer on the host's
  behalf.
- A trapped `Init()` means the program never reached a valid initial state; no partial state
  is retained and no `Run()` follows.

Rollback covers only what the engine owns. Host objects reached through registered rules are
outside it: a host method that mutates its receiver mid-tick — an indicator advancing its
window, say — is not undone by a later trap. Keeping such an object consistent across a
failed tick is the host's business.

#### Internal failures

An internal failure is a recovered panic below the two public entry points (`EvalInit`,
`EvalRun`): core code reaching a state the resolver was supposed to prevent, or a registered
rule that panicked. It surfaces with kind `INTERNAL_FAILURE`, the entry function, and a stack trace,
but **no source position** — it signals an interpreter or host-rule bug, not a program
fault.

## 7. Resource Limits

Conservative limits prevent pathological programs from monopolising resources. The host may
tighten but not relax them above the ceilings.

| Limit | Initial value | Phase | Behaviour on breach |
|-------|---------------|-------|---------------------|
| Max history window length | 5000 | parse | Parse error (`HISTORY_LIMIT`) when static analysis computes a window bound exceeding the cap. |
| Max string literal length | 4096 chars | parse | Parse error. |
| Max string value at runtime | 4096 chars | runtime | Runtime error. |
| Max `emit(...)` kwargs per call | 32 | parse | Parse error. |
| Max identifier length | 128 | parse | Parse error. |
| Max nested expression depth | 64 | parse | Parse error. |
| Max source size | 256 KB | parse | Parse error. |
| Max parse errors collected | 100 | parse | Parser aborts the collection pass (§6.3). |

Host libraries may add their own limits (e.g. indicator-driven window sizing) in their specs.
Wall-clock budgets per `Init()`/`Run()` are **operational** limits enforced by the host, not
the language.

### Reserved category codes for limits

| Category | Phase | When |
|----------|-------|------|
| `HISTORY_LIMIT` | parse | Static analysis computed a per-window bound exceeding the limit. |
| `STRING_LIMIT` | parse / runtime | String literal or runtime string value exceeded the cap. |
| `KWARG_LIMIT` | parse | `emit(...)` with more than the allowed kwargs. |
| `IDENT_LIMIT` | parse | Identifier longer than the cap. |
| `NESTING_TOO_DEEP` | parse | Expression nested deeper than the cap. |
| `SOURCE_SIZE_LIMIT` | parse | Source larger than the cap. |

## 8. Examples (core, domain-free)

These use only core features — no host, no candles — to show the core surface stands on its
own. Realistic technical-analysis programs live in `SPEC_SIGNAL_HOST.md`.

### 8.1 Threshold alert with a cooldown

One numeric input, a persistent counter, a threshold condition.

```js
const THRESHOLD = 100

input metric: Float

output alerts: {
  level: String,
  value: Float
}

state cooldown: Integer = 0

function Run() {
  state.cooldown = math.max(0, state.cooldown - 1)

  if (metric > THRESHOLD && state.cooldown == 0) {
    emit(alerts, level = "high", value = metric)
    state.cooldown = 20
  }
}
```

### 8.2 Time-based debounce

Exercises `Time`/`Duration`, `state.*` holding a `Time`, and `Init()` seeding.

```js
const COOLDOWN = 10 * time.MINUTE

input metric: Float
input clock: Time            // host supplies the current activation time

output alerts: { value: Float }

state last_alert: Time

function Init() {
  state.last_alert = time.from_unix_ms(0)
}

function Run() {
  if (metric > 100 && clock - state.last_alert > COOLDOWN) {
    emit(alerts, value = metric)
    state.last_alert = clock
  }
}
```

### Gaps surfaced by these examples

1. **`return` statement.** No early-return form is locked; the substitute is wrapping body
   code in a filter `if`. If real programs grow nested, `return` should be a small addition.
2. **Multi-line expressions / line continuation.** Bracket-depth suppression is locked: a
   NEWLINE is swallowed inside an open `(` or `[`, permitting multi-line calls and index
   expressions (inline schemas span lines because the parser skips newlines while reading
   them, not because of bracket depth). Broader trailing-token continuation is deferred:
   continuation is decided by the token at the **end** of a line, with one exception —
   keywords that can never begin a statement. `else` is the only one today; adding to that
   set requires the same proof.
