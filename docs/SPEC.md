# tascript — Core Language Specification

> Status: **v0.1**. Normative: this document defines the language. Design rationale,
> rejected alternatives, and planned work live in `DESIGN_TODO.md`.
>
> **Scope: this is the CORE language — it is domain-blind.** It knows nothing about
> candles, indicators, orders, or any trading concept, and it has no vocabulary for
> persistence either. Domain words are supplied by a **host** (one per block type) through
> the registry. See §4.4 for the extension mechanism and the core/host boundary.

## 1. Purpose

tascript is a small language for **per-event block logic**. A program is the logic inside
one *block* on a dataflow graph: it declares typed **input ports**, reacts when the host
activates it, holds values across activations in **slots**, and **emits** to **output
ports** wired to sinks.

Model: **inputs → logic + slots → emitted events.**

The core knows primitives, control flow, ports, a registry of host-supplied types and
rules, and a declaration-form mechanism through which a host adds its own declaration
keywords. It does not know what a candle, an indicator, or an order is — and it does not
know what `state` is either. `state` is an ordinary host registration (§4.2), which is why
every example in §8 declares the host it assumes.

## 2. Event output

Every program's output is a stream of values pushed through declared **output ports**. A
port is bound to a host **sink** before `Init`:

```go
type Sink interface { Emit(Value) }
```

`emit(...)` delivers to that sink as it is evaluated, from inside the tick, in program
order. A program may emit zero or more times per activation and may target different
outputs in the same activation.

**What the sink receives is the value, and only the value.** A value output delivers the
scalar; a structured output delivers a `Record` whose `Fields` map holds the declared
fields. There is no envelope — no port name, no timestamp, no wrapper. The sink already
knows which port it was bound to, and the engine has no clock.

Delivery policy is entirely the sink's: buffer-and-flush, fire-and-forget, or drop during
warmup. This is why a `log` output and an order router can coexist under one program. The
engine takes no position and never buffers on the host's behalf.

Inputs and outputs are declared in the script but wired outside it:

```js
input metric: Float

output alerts: {
  level: String,
  value: Float
}
output logs: String
```

The host maps `metric`, `alerts`, and `logs` to concrete blocks. The language knows nothing
about sources, transports, formats, or credentials.

**Out of scope for the DSL:** delivery destinations, output formatting, secrets, transport
configuration, routing and fan-out. These belong to the layer that consumes the stream.

## 3. Grammar

### 3.1 Surface style

tascript leans on a JavaScript-flavoured surface with deliberate deviations:

- **No statement terminators.** Newlines end statements; `;` is not used. The lexer
  suppresses newlines inside an open `(` or `[`, so long calls may span lines, and before an
  `else`, so the keyword may start its own line. Inline type schemas (`{field: Type, ...}`)
  may also span lines — the parser skips newlines while reading them — but `{` **blocks**
  keep newlines significant, since there they are the statement separators.
- **C-style blocks.** `if (cond) { ... } else { ... }`. `else` may start its own line.
- **C-style logical operators.** `&&`, `||`, `!` (not `and`, `or`, `not`).
- **Trailing comma** in call arguments (`f(a, key=b,)`) and inline type schemas (`{a: Integer,}`).
- **Function-local bindings use `let`.** Inside `Init()` and `Run()`, `let x = a > b` binds
  for the current invocation only. There is no `var`.
- **Top-level constants use `const`.** Valid only at the top level.
- **Everything else that persists is a host-registered declaration** (§4.2), written
  `<keyword> <name> [: Type] [= expr]`.

Keywords are closed: `let`, `const`, `input`, `output`, `function`, `if`, `else`, `true`,
`false`. `emit`, `Init`, and `Run` are reserved identifiers — the lexer emits them as
`IDENT`, and declaring over one is `RESERVED_NAME`.

#### Comments

Single-line only (`//`). No block-comment form.

### 3.2 Program structure

A program is composed of:

1. **Zero or more top-level constants.** `const THRESHOLD = 0.5` is evaluated once at load.
   Reassignment from inside a function is `NOT_ASSIGNABLE`. By convention `UPPER_SNAKE_CASE`.
2. **Zero or more input declarations.** `input name: Type` declares a runtime-wired input
   port, read-only, readable in `Run` only (§3.3).
3. **Zero or more output declarations.** `output name: Type` or `output name: { ... }`.
4. **Zero or more host-kind declarations.** `state cooldown: Integer = 0`,
   `indicator fast = ta.sma(3, source = ta.Close)` — legal only for keywords the host
   registered (§4.2).
5. **An optional `function Init() { ... }`**, run exactly once before the first activation.
6. **A required `function Run() { ... }`**, run once per activation.

Only `Run` is mandatory; its absence is `MISSING_RUN`. An empty `Run` body is
`EMPTY_FUNCTION`. A function other than `Init`/`Run` is `FORBIDDEN_FUNCTION`. No other
top-level forms are permitted (`TOP_DECL_UNEXPECTED`); a top-level declaration inside a
function body is `TOP_DECL_MISPLACED`. There are no user-defined functions: `Init` and `Run`
are the only two a program may declare, and neither takes parameters or returns a value.
There is no `return` statement; a function body runs to its end, and conditional logic is
expressed by wrapping code in an `if`.

Canonical shape, assuming a host that registers `state`:

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

The DSL names no sources, symbols, timeframes, chats, or URLs. Programs declare named
**ports**; the blocks behind them are configured outside the DSL.

#### Input declarations

```js
input <name>: <Type>
```

`<Type>` is any registered type — core value types (§3.4) or host types. The name becomes a
read-only top-level value.

An input is bound once, before `Init`, via `BindInput(name, value)`, and the binding is
permanent. **Inputs are not readable inside `Init`** (`INPUT_IN_INIT`): `Init` runs the load
phase, where port data is not yet meaningful. Every declared input must be bound before
`Init` or `Init` fails with `INPUT_MISSING`; binding a name the script never declared is
`INPUT_UNKNOWN`; binding a value whose type does not match and cannot be coerced is
`INPUT_TYPE_MISMATCH`.

#### Output declarations

An output is **either** a value output or a structured output. The form after `:` decides:

```js
output <name>: <Type>                   // value output — emit a single value
output <name>: { field: Type, ... }     // structured output — emit keyword fields
```

`<Type>` may be **any registered type**, host types included. The language takes no position
on what may cross an output port: a sink that serialises to a webhook and a sink that hands
the object to the next in-process block are both just sinks, and which one a port has is
the host's wiring decision (§4.5).

An empty schema (`output x: {}`) is `EMPTY_CUSTOM_TYPE`. A structured output's schema is
registered as a script type named `output.<name>`, which is why two outputs cannot share a
name.

Output names are not readable values. They are valid only as the first argument to
`emit(...)`.

Every declared output must be bound to a sink before `Init` via `BindOutput(name, sink)`, or
`Init` fails with `OUTPUT_MISSING`. Binding an undeclared name is `OUTPUT_UNKNOWN`.

A `Sink` reports the type it accepts through `TypeID() TypeID`, the same method every
`Value` carries. `BindOutput` checks it against the port's declared type and rejects a
mismatch with `OUTPUT_TYPE_MISMATCH`. For a structured port the sink names the script type
the port registered, `output.<name>`.

#### Port rules

- Port declarations may appear **only at the top level**.
- Port names are identifiers, not string literals.
- Ports, constants, host-kind declarations, function names, module names, registered type
  names, and reserved names share one top-level namespace.
- A declaration over a name the prelude already binds — a module, a registered type name,
  `Init`, `Run`, `emit` — is `RESERVED_NAME`. A declaration over an earlier *script*
  declaration is `DUPLICATE_DECLARATION`.
- **Shadowing is not permitted.** A `let` may not reuse a name bound in any enclosing scope,
  top level included; the diagnostic is `DUPLICATE_DECLARATION`. Sibling blocks may each
  bind the same name.
- An input binding is read-only; assigning to it is `NOT_ASSIGNABLE`.
- An output name is emit-only; reading it is `NOT_READABLE`, and `emit`ing to anything that
  is not a declared output is `INVALID_EMIT_TARGET`.

**Out of scope for this revision:** user-tunable config inputs with defaults
(`input period: Integer = 14` — a host `setting` kind covers this), and named reusable
custom types (`type Alert { … }`).

### 3.4 Value types

The core scalar value types:

| Type       | Notes |
|------------|-------|
| `Integer`  | Signed 64-bit. `14` is an `Integer`. |
| `Float`    | `float64`. `1.5` is a `Float`. `Integer` coerces to `Float`; `/` always returns `Float`. |
| `Bool`     | `true`, `false`. |
| `String`   | Double-quoted only: `"BTC/USDC"`. |

`Time` and `Duration` are supplied by the `time` prelude module (§3.5).

`Integer → Float` is the single implicit coercion, applied at operator, argument, and
assignment boundaries.

Residual sharp edges, all current behaviour: `/` always returns `Float`, even for two
`Integer`s; `Integer` arithmetic **wraps silently** on overflow; and
`-9223372036854775808` fails to parse with `NUMBER_OUT_OF_RANGE`, since `-` is an operator
and the bare literal overflows on its own.

**There is no `Null` type, and there are no collection types.** No type is indexable: `a[i]`
parses, and always resolves to `NOT_INDEXABLE`.

Domain value types are host-supplied and opaque to the core: the engine knows a host value
only by its `TypeID` and the rules registered against it.

### 3.5 Time and Duration

`Time` and `Duration` are prelude types from the `time` module. Their code is part of the
core distribution.

#### `Time`

A point in time. Sources: `time.from_unix_ms(ms)`, or a host-provided time value. There is
no time-zone support; all component access is UTC. There is no `Time` literal syntax.

**Properties (UTC):** `.unix_ms`, `.year`, `.month`, `.day`, `.weekday` (0 = Sunday),
`.hour`, `.minute`, `.second`, `.millisecond` — all `Integer`.

**Methods (UTC):**

| Method | Type | Notes |
|--------|------|-------|
| `.truncate(bucket: Duration)` | `Time` | Floors to the start of the fixed UTC bucket. `bucket` must be positive, no larger than `time.DAY`, and must evenly divide `time.DAY`; otherwise `INVALID_ARGUMENT`. Flooring is toward negative infinity. |

`90 * time.MINUTE`, `4 * time.HOUR`, and `time.DAY` are valid buckets; `7 * time.HOUR` is
not, because `time.DAY` is not divisible by it.

#### `Duration`

A signed length of time in milliseconds. Produced by `Time - Time`, by a numeric times a
`time.*` constant, or by arithmetic on durations.

| Constant | Value | | Constant | Value |
|----------|-------|-|----------|-------|
| `time.MILLISECOND` | 1 ms | | `time.SUNDAY` | 0 |
| `time.SECOND` | 1000 ms | | `time.MONDAY` | 1 |
| `time.MINUTE` | 60 × SECOND | | `time.TUESDAY` | 2 |
| `time.HOUR` | 60 × MINUTE | | `time.WEDNESDAY` | 3 |
| `time.DAY` | 24 × HOUR | | `time.THURSDAY` | 4 |
| `time.WEEK` | 7 × DAY | | `time.FRIDAY` | 5 |
| | | | `time.SATURDAY` | 6 |

No `time.MONTH` or `time.YEAR` — those are calendar-dependent, not fixed durations. Although
`time.WEEK` is a valid duration, `t.truncate(time.WEEK)` is rejected (epoch-modulo week
truncation would anchor to Thursday 00:00 UTC).

**Properties:** `.milliseconds` (`Integer`), `.seconds`, `.minutes`, `.hours`, `.days`,
`.weeks` (all `Float`), `.abs` (`Duration`).

#### Operator semantics

| Op | Result | | Op | Result |
|----|--------|-|----|--------|
| `Time - Time` | `Duration` | | `Duration ± Duration` | `Duration` |
| `Time + Duration` | `Time` | | `<numeric> * Duration` | `Duration` |
| `Time - Duration` | `Time` | | `Duration * <numeric>` | `Duration` |
| `Time` `<` `<=` `>` `>=` `Time` | `Bool` | | `Duration / <numeric>` | `Duration` |
| `Time == Time` | `Bool` | | `Duration / Duration` | `Float` (ratio) |
| `Time + Time` | `INVALID_OPERATION` | | `Duration` comparisons | `Bool` |
| `Time ± <numeric>` | `INVALID_OPERATION` | | unary `-Duration` | `Duration` |
| | | | `Duration + <numeric>` | `INVALID_OPERATION` |

A zero divisor in `Duration / <numeric>` or `Duration / Duration` is `DIVISION_BY_ZERO`.
Multiplication and division round the resulting milliseconds to the nearest integer, half
away from zero.

`Time` and `Duration` are first-class: storable in slots, passable in `emit(...)`,
assignable to locals. Duration arithmetic with both numeric types is registered explicitly;
infix resolution uses exact binary-rule lookup, then the `Integer → Float` coercion.

### 3.6 Operators, equality, and conditions

tascript rejects JavaScript's loose-equality and truthy-falsy semantics.

#### Equality (`==`, `!=`)

Operands must have the same type, except that `Integer` and `Float` compare numerically.
Any other cross-type pair is `INVALID_OPERATION`. No silent `false`, no implicit conversion.

#### Ordering (`<`, `<=`, `>`, `>=`)

Defined on numeric × numeric, and on `Time`/`Duration` per §3.5. Anything else is
`INVALID_OPERATION`. `String` and `Bool` do not order.

#### Boolean position

`if (cond)`, `&&`, `||`, and unary `!` all require `Bool`. A numeric (including `0`), a
`String`, or any non-`Bool` in boolean position is `TYPE_MISMATCH` for `if`, `&&`, and `||`,
which check the operand against `Bool` directly, and `INVALID_OPERATION` for unary `!`, which
resolves through unary-rule lookup and finds none. No truthy / falsy values.

```js
if (state.cooldown)     { … }   // error — cooldown is numeric
if (state.cooldown > 0) { … }   // ok
```

#### Precedence and associativity

Standard C/JS precedence, tightest to loosest:

| Tier | Operators | Associativity |
|------|-----------|---------------|
| 1 | `()` grouping, `.` member access, call | n/a |
| 2 | unary `!`, unary `-` | right |
| 3 | `*`, `/`, `%` | left |
| 4 | binary `+`, binary `-` | left |
| 5 | `<`, `<=`, `>`, `>=` | left |
| 6 | `==`, `!=` | left |
| 7 | `&&` | left |
| 8 | `\|\|` | left |

**Not present:** bitwise operators, ternary `? :`, exponentiation (use `math.pow(x, y)`).
`[i]` parses but no type carries an index rule, so it never resolves (§3.4).

**Assignment (`=`) is a statement, not an expression.** `a = b = c` and `x = (y = 1)` are
parse errors.

#### Short-circuit evaluation

`&&` and `||` short-circuit; the RHS is not evaluated when the LHS decides the result. A
trap in an unevaluated RHS therefore never fires.

## 4. Semantics

### 4.1 Execution model — per-event

A program is a dataflow node with a mailbox. The host drives it through a three-stage
lifecycle, and `Executable.Stage` (`created → initialized | failed`) answers what is allowed
right now:

1. **created** — `BindInput`, `BindOutput`, and `Slot.Set` are legal. Binding after this
   stage is `ErrBindTooLate`.
2. **`Init()`** — runs the load phase exactly once: slot initializers in source order, then
   the `Init` body. Every declared input and output must be bound first. A failure is
   terminal; the executable moves to **failed** and never runs.
3. **`Run()`** — one activation. Reads inputs from their bindings, executes the body
   top-to-bottom, delivers each `emit` to its sink as it is evaluated.

**There is no tick rollback.** A trapped `Run` is *unfinished*, not undone: slot writes made
before the trap persist, delivered emits are not recalled, and host objects mutated by
registered rules keep their new state. `Run` may be called again after a trap; whether that
is sensible is the host's judgement. This is a deliberate simplification — see §6.5.

**Activation policy is a host concern.** *When* `Run` fires — on any input update, only when
all inputs have advanced, or some custom gate — is decided by host-side sync primitives. The
core exposes ports and the activation hook; it never encodes scheduling. There is no
`primary(...)`, no `synchronize(...)`, and no way to read or branch on cadence. Programs are
written to be correct under whichever cadence the operator configures — typically by reading
timestamps and debouncing through a slot.

A "candle tick" is simply the case where a signal block's main input is a candle stream; the
core has no candle concept.

### 4.2 Declaration kinds and slots

Beyond `const`, `input`, and `output`, every top-level declaration form is **registered by
the host**. A host registers a word and the parser accepts it over the fixed shape:

```
<keyword> <name> [: Type] [= expr]
```

The registration carries five fields:

| Field | Effect |
|-------|--------|
| `Word` | The keyword. Must be a valid identifier, and not a language keyword, reserved identifier, registered type, or module name. |
| `Initializer` | `Required` / `Optional` / `Forbidden`. Violations are `INITIALIZER_REQUIRED` / `INITIALIZER_FORBIDDEN`. |
| `Assignable` | Whether the script may assign to it after declaration. If not, assignment is `NOT_ASSIGNABLE`. |
| `Namespaced` | If set, the entry is read and written as `kind.name`; if not, as a bare `name`. |
| `AllowedTypes` | The types this kind accepts. **Empty accepts every registered type.** A violation is `DECL_TYPE_NOT_ALLOWED`. |

An unregistered word in declaration position is `UNKNOWN_DECL_KEYWORD`. A declaration with
neither a type annotation nor an initializer is `TYPE_REQUIRED`, since neither the declared
nor the inferred type is available. Reading or writing `kind.name` where no such declaration
exists is `SLOT_UNDECLARED`.

The reference host registers three kinds, none of which is core:

```js
setting   fastPeriod: Integer = 3     // optional initializer, namespaced
indicator fast = ta.sma(3, ta.Close)  // required initializer, bare name, type-restricted
state     cooldown: Integer = 0       // optional initializer, assignable, namespaced
```

#### Source-order declaration

All top-level declarations — `const` included — are resolved in **one source-order walk**,
and that walk *is* the scoping rule: a declaration may reference anything declared above it
and nothing below. Referencing a later declaration is `USE_BEFORE_DECLARATION`.

```js
setting period: Integer = 14
const window = period * 2        // ok
const bad = missing * 2          // USE_BEFORE_DECLARATION
setting missing: Integer = 1
```

#### Slots

Each host-kind declaration owns a **slot** — one engine-owned cell that survives across
activations. The host reaches slots by handle, resolved once at wire time:

```go
slot, ok := program.Slot("setting", "slowPeriod")
slot.Set(registry.Integer(7))
value, err := slot.Get()
```

Two rules govern filling:

- **Rule A — a host fill wins.** If the host calls `Slot.Set` before `Init`, the script's
  initializer for that slot does not run. This is how a host overrides a script's declared
  default without rewriting the source.
- **Rule B — nothing may be empty when `Init` ends.** A slot with no initializer that the
  host never filled and `Init` never assigned traps with `UNINITIALIZED_SLOT`. This replaces
  static definite-assignment analysis: it is a runtime check at the end of the load phase,
  not a proof over `Init`'s control flow.

Reading a slot the host never filled through `Slot.Get` returns `ErrSlotEmpty` rather than a
zero value. There are no silent defaults anywhere.

`Slot.Set` is legal during the host's turn only — before `Init`, or between activations.
Calling it mid-tick is `ErrMidActivation`.

### 4.3 Assignability and readability

Assignability is a property of the **declaration**, not the type:

| Form | Writable? |
|------|-----------|
| `const`, `input` | no — `NOT_ASSIGNABLE` |
| host kind with `Assignable: true` | yes, whole-value replace |
| host kind with `Assignable: false` | no — mutate through registered methods instead |
| `let` | yes, within its invocation |

Assignment targets are limited to a bare name or a namespaced `kind.name`. Anything else —
`obj.field = x`, `arr[i] = x` — is `INVALID_ASSIGN_TARGET`.

**Safety invariant:** a script can never write *into* a host-owned value's internals. No core
form produces a member or element lvalue, and host types must not expose one. A host method
may mutate its own receiver's internal state, but must never hand the script an lvalue
pointing inside a host object.

**Readability** is orthogonal, and a strictly smaller set. Only `const`, `input`, `let`, and
host-kind slots carry a readable value. An output is emit-only; modules, registered type
names, `Init`, `Run`, and `emit` occupy the namespace without denoting a value. Naming one in
value position is `NOT_READABLE`, not a type error.

### 4.4 Prelude and the host extension mechanism

**Irreducibly core** (never a registration): the syntax — literals, operators, `if`, `let`,
`function`, blocks, and the declaration-form shape; the slot mechanism; the per-event
execution model; the registry itself.

**Standard prelude** — registered by `NewBuilder`:

- the value types `Integer`, `Float`, `String`, `Bool`
- arithmetic, comparison, and logical rules over them, plus `Integer → Float` coercion
- the `const`, `input`, and `output` declaration forms

The `math` and `time` modules (§5.3, §3.5), and with them the `Time` and `Duration` types,
are **opt-in**: a host registers them, and a program naming `math` or `time` against a host
that did not is `UNDEFINED_IDENT`.

**The prelude registers no declaration kinds.** A fresh registry has none: a program using
`state` compiles only against a host that registered the word.

**Host extension.** A host specialises the core by registering, all before `Compile`:

1. **Types** — an opaque `TypeID` per host type. The engine never inspects a host value; it
   knows only the ID and the rules attached to it.
2. **Rules** — member access, method calls, binary and unary operators, and coercions, each
   keyed by the types it applies to and carrying a Go function the evaluator calls.
3. **Modules** — namespace values like `ta`, holding member and call rules.
4. **Declaration keywords** — §4.2. Hosts register *words plus semantics*, never new grammar.
   The lexer stays dumb (it emits `IDENT`); the parser dispatches on the registry. Operators
   stay closed: a host cannot invent syntax.

The builder is **single-use**. `Compile` spends it whether or not the script compiled, and
every `Register*` afterwards returns `ErrBuilderSpent`. A host retrying a script the user
just fixed must build a fresh builder and re-register. The package is single-threaded by
contract: one builder, one script, one executable.

### 4.5 Host-owned values

Values registered by the host are **host-owned, and the engine never copies them**. A
host-owned value crossing any boundary — bound to an input, stored in a slot, passed to a
sink through `emit` — is the same object the host holds. Three consequences the host owns:

- **A bound pointer is live; a bound value is a snapshot.** `BindInput` takes a `Value`
  interface, so a pointer type tracks the host's later mutations while a value type is frozen
  at bind. Anything the engine coerces on the way in (`Integer → Float`, or a host-registered
  coercion) is likewise the snapshot taken at bind, not the host's original.
- **Emitted handles escape.** A structured emit allocates a fresh `Record`, but its field
  values are the same references. A sink that *retains* a host value past the tick is reading
  an object the host may still be mutating.
- **The coherence window spans the tick.** A bound object must hold still from the moment
  `Run` is entered until it returns. Mutating it between activations is the intended pattern
  — that is how a feed advances — but mutating it during one is a host bug the engine cannot
  detect.

None of this is enforced, and enforcing it would mean copying, which defeats the purpose of
passing a series by handle in the first place.

## 5. Standard library (core)

### 5.1 Call arguments

All calls — module calls, method calls, and `emit` — obey one set of rules. A rule declares
an ordered parameter list, each with a name and a type.

- Positional arguments bind in declaration order.
- Keyword arguments (`name = expr`) bind by name.
- A positional argument may not follow a keyword argument (`ARG_ORDER_INVALID`).
- Every parameter must be filled exactly once. Unfilled is `ARG_MISSING`; filled twice —
  positionally and by keyword, or by two keywords — is `ARG_DUPLICATE`.
- A keyword naming no parameter is `ARG_UNKNOWN`; too many positional arguments is
  `ARG_COUNT_MISMATCH`.
- Each argument's type must match its parameter, with `Integer → Float` applied.

Parameters have no default values, so the argument count is always exact.

### 5.2 `emit(...)` — event emission

```
emit(OUTPUT, arg [, arg]* [, ident=expr]*)
```

- `OUTPUT` is a declared output identifier, not a string literal. Anything else is
  `INVALID_EMIT_TARGET`.
- `emit(...)` is a statement, valid **only inside `Run()`**. Use in `Init()` or at the top
  level is `EMIT_OUTSIDE_RUN`; use in expression position is `EMIT_NOT_EXPRESSION`.
- The payload obeys §5.1 exactly — `emit` is not a special form. A **structured** output
  takes one parameter per declared field, in schema order. A **value** output takes a single
  parameter named `value`.

```js
input metric: Float
output logs: String

function Run() {
  emit(logs, "threshold crossed")
  emit(logs, value = "threshold crossed")   // the same call
}
```

```js
output alert: { message: String, value: Float }

function Run() {
  emit(alert, message = "crossed", value = metric)
  emit(alert, "crossed", metric)            // equivalent, binds by field order
}
```

> **Positional payloads bind by field order.** Reordering a structured output's `{ … }`
> schema silently changes the meaning of every positional `emit` against it, with no
> diagnostic when the reordered types still match. Keyword form is immune; prefer it for
> schemas with more than one field of the same type.

**Schema enforcement.** A structured output's schema is strict and closed: every field
supplied, no undeclared fields, each value matching its declared type. All of it is resolved
statically, so there is no runtime payload check.

There is no in-language string interpolation.

### 5.3 The `math` namespace

Free-function helpers live in modules. Modules are **passive prefixes**, not first-class
values — `math` cannot be assigned, passed, or reflected on. Naming one in value position
(`let m = math`) is `NOT_READABLE`.

| Call | Behaviour |
|------|-----------|
| `math.E`, `math.PI` | Euler's number, pi |
| `math.max(a, b)`, `math.min(a, b)` | larger / smaller of two numbers |
| `math.abs(x)` | absolute value |
| `math.sqrt(x)` | square root; `INVALID_ARGUMENT` if `x < 0` |
| `math.pow(base, exponent)` | base to the exponent |
| `math.floor(x)`, `math.ceil(x)` | round toward −∞ / +∞ |
| `math.round(x)` | round to nearest, half toward +∞ |
| `math.trunc(x)` | integer part, toward zero |
| `math.sign(x)` | sign of number |
| `math.log(x)` | natural logarithm; `INVALID_ARGUMENT` if `x <= 0` |

`math` and `time` are reserved module names; declaring over one is `RESERVED_NAME`. Hosts add
their own modules (e.g. `ta`).

> `math.pow` can still return NaN — `math.pow(-1.0, 0.5)` — because the domain trap is
> single-argument. A two-argument domain rule is outstanding.

There is no string composition in the language: programs emit a string value or structured
fields, and rendering is the sink's responsibility.

## 6. Diagnostics

### 6.1 Two phases

- **Check-time** — parse and resolve, before any activation. `Compile` returns them as a
  slice; a non-empty slice means the program is rejected.
- **Runtime** — during `Init()` or `Run()`, returned as an `error` from those calls.

`Compile` also returns a separate `error` for host/API misuse, distinct from script
diagnostics. Today its sole producer is `ErrBuilderSpent`.

### 6.2 Every diagnostic carries

- **Phase** — structured metadata, not rendered.
- **Category code** — stable machine-readable identifier (§6.4).
- **Source location** — line, column. *Which* script is the host's identity to supply.
- **Human-readable message.**

```
error[TYPE_MISMATCH] 2:19: expected Float, found String
error[DIVISION_BY_ZERO] 3:9: integer division by zero
```

Type mismatches read `expected X, found Y`. Runtime traps borrow their category code from
the runtime error kind, so both namespaces share the bracket. Two diagnostics render with no
source location: a recovered internal panic (no node is in hand once the stack has unwound)
and an unknown input port (the offending key comes from the host and appears nowhere in the
script).

### 6.3 Check-time policy

The parser **collects multiple errors before aborting**, capped at 100. Recovery uses
statement-level resynchronisation. `Resolve()` requires a clean parse and is unguarded by
contract — `Compile` returns on parse diagnostics before resolving. The resolver always
returns a possibly-partial program, so a caller driving it directly must check its
diagnostics before using the result.

Diagnostics have no severity: every diagnostic is an error, and any diagnostic rejects the
program. There are no warnings.

### 6.4 Stable category codes

Codes are a user-facing category, not a one-to-one class identifier — several classes share
`INVALID_OPERATION`. Future codes are additive; existing codes never change meaning.

**Lexing and parsing**

| Code | When |
|------|------|
| `UNEXPECTED_TOKEN` | Token where the grammar allows another. |
| `EXPRESSION_EXPECTED` | Expression position holds something that cannot start one. |
| `NUMBER_OUT_OF_RANGE` | Numeric literal outside its type's range. |
| `TYPE_EXPECTED` | Type position holds neither a type name nor an inline schema. |
| `EMPTY_CUSTOM_TYPE` | An inline schema `{}` declares no fields. |
| `NESTING_TOO_DEEP` | Expression or block nested deeper than 64. |

**Program shape**

| Code | When |
|------|------|
| `MISSING_RUN` | No `function Run()`. |
| `FORBIDDEN_FUNCTION` | A function other than `Init` / `Run`. |
| `EMPTY_FUNCTION` | `Run` has an empty body. |
| `TOP_DECL_UNEXPECTED` | A construct not permitted at the top level. |
| `TOP_DECL_MISPLACED` | A top-level declaration inside a function body. |

**Declarations**

| Code | When |
|------|------|
| `DUPLICATE_DECLARATION` | Two declarations of one name, including a shadowing `let`. |
| `RESERVED_NAME` | Declaration over a prelude name — module, registered type, `Init`, `Run`, `emit`. |
| `UNKNOWN_DECL_KEYWORD` | Declaration keyword no host registered. |
| `INITIALIZER_REQUIRED` | Kind requires an initializer; none given. |
| `INITIALIZER_FORBIDDEN` | Kind forbids an initializer; one given. |
| `TYPE_REQUIRED` | Declaration has neither type annotation nor initializer. |
| `DECL_TYPE_NOT_ALLOWED` | Type outside the kind's `AllowedTypes`. |
| `USE_BEFORE_DECLARATION` | Reference to a declaration further down the source. |
| `UNDEFINED_TYPE` | Type name no host registered. |
| `TYPE_REGISTRATION_FAILED` | An inline schema could not be registered (name collision). |

**Names and access**

| Code | When |
|------|------|
| `UNDEFINED_IDENT` | Unknown identifier. |
| `UNDEFINED_VAR` | Unknown variable in assignment position. |
| `UNDEFINED_ATTRIBUTE` | Member access with no registered rule. |
| `UNDEFINED_METHOD` | Method call with no registered rule. |
| `UNDEFINED_FUNC` | A bare call `foo(...)`; only module and method calls exist. |
| `SLOT_UNDECLARED` | `kind.name` with no matching declaration. |
| `NOT_READABLE` | A name carrying no value used in value position. |
| `NOT_ASSIGNABLE` | Write to a non-assignable declaration. |
| `INVALID_ASSIGN_TARGET` | Assignment to anything but a name or `kind.name`. |
| `NOT_CALLABLE` | Call on a non-callable expression. |
| `NOT_INDEXABLE` | `a[i]` — no type has an index rule. |
| `INPUT_IN_INIT` | An input read inside `Init`. |

**Types, operators, calls**

| Code | When |
|------|------|
| `TYPE_MISMATCH` | Value's type is not the one required, and does not coerce. |
| `INVALID_OPERATION` | Operator with no rule for its operand types. |
| `ARG_COUNT_MISMATCH` | More arguments than parameters. |
| `ARG_MISSING` | A parameter left unfilled. |
| `ARG_DUPLICATE` | A parameter filled twice. |
| `ARG_UNKNOWN` | Keyword naming no parameter. |
| `ARG_ORDER_INVALID` | Positional argument after a keyword argument. |

**Emit**

| Code | When |
|------|------|
| `EMIT_OUTSIDE_RUN` | `emit(...)` outside `function Run()`. |
| `EMIT_NOT_EXPRESSION` | `emit(...)` in expression position. |
| `INVALID_EMIT_TARGET` | `emit` target is not a declared output. |

**Wiring (runtime)**

| Code | When |
|------|------|
| `INPUT_MISSING` | A declared input was never bound. |
| `INPUT_UNKNOWN` | Bind of a name the script does not declare. Renders with no source location. |
| `INPUT_TYPE_MISMATCH` | Bound value's type does not match and does not coerce. |
| `OUTPUT_MISSING` | A declared output was never bound to a sink. |
| `OUTPUT_UNKNOWN` | Bind of an output the script does not declare. |
| `OUTPUT_TYPE_MISMATCH` | A sink was bound to a port of another type. |
| `SLOT_TYPE_MISMATCH` | `Slot.Set` with a value of the wrong type. |

### 6.5 Runtime failure protocol

Runtime errors come in two kinds, split by whether the **program** or the **interpreter** is
at fault.

#### Script traps

A trap is a failure a correct interpreter hits on otherwise legal input. It carries a stable
kind, a source position, and a message:

| Kind | When |
|------|------|
| `DIVISION_BY_ZERO` | `/` or `%` with a zero divisor — integer, float, or `Duration`. |
| `INVALID_ARGUMENT` | A rule rejected a well-typed argument (a bad `truncate` bucket, `sqrt` of a negative). |
| `OUT_OF_RANGE` | A host rule rejected an index. |
| `UNINITIALIZED_SLOT` | A slot reached the end of `Init`, or was read, with nothing in it (§4.2 Rule B). |
| `UNKNOWN_FAILURE` | A host-registered rule returned a non-registry error; wrapped verbatim. |

**A trap aborts the current invocation and nothing else.** There is no rollback:

- A trapped `Run` leaves every slot write made before the trap in place, and every emit
  already delivered stays delivered. A tick that traps halfway has emitted its first half.
  Whether that is acceptable is the sink's policy — a `log` output wants every line
  regardless, an order router may prefer to buffer and flush only after `Run` returns
  cleanly. The engine takes no position.
- Host objects mutated by registered rules mid-tick — an indicator advancing its window —
  keep their new state. Keeping such an object consistent across a failed tick is the host's
  business (§4.5).
- A trapped `Init` is terminal: the program never reached a valid initial state, the
  executable moves to **failed**, and no `Run` follows.

#### Internal failures

An internal failure is a recovered panic below `EvalInit` / `EvalRun`: core code reaching a
state the resolver was supposed to prevent, or a registered host rule that panicked. It
surfaces as `INTERNAL_FAILURE` with the entry function and a stack trace, but **no source
position** — it signals an interpreter or host bug, not a program fault.

## 7. Resource limits

The language imposes two limits:

| Limit | Value | Phase | On breach |
|-------|-------|-------|-----------|
| Max nested expression depth | 64 | parse | `NESTING_TOO_DEEP` |
| Max diagnostics collected | 100 | parse | Parser aborts the collection pass (§6.3) |

There are no limits on string length, identifier length, source size, or argument count.
Wall-clock budgets per `Init` / `Run` are **operational** limits enforced by the host, not
the language.

## 8. Examples

The core alone has no way to keep a value across activations, so both examples below assume
a host that registers a `state` kind (optional initializer, assignable, namespaced) — the
minimum a real host provides, and a demonstration of the §4.2 extension mechanism rather
than of core syntax alone.

### 8.1 Threshold alert with a cooldown

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

Exercises `Time`/`Duration`, a slot holding a `Time`, and `Init` seeding. Note that `clock`
is an input, so it cannot be read in `Init` (§3.3) — the seed is a fixed anchor.

```js
const COOLDOWN = 10 * time.MINUTE

input metric: Float
input clock: Time            // host supplies the activation time

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
