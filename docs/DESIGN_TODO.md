# Design TODO — work queue

What is left to do, and nothing else. This file is not a record of what happened: what shipped
is in git, and the normative surface is `docs/SPEC.md`, which states what the language does and
nothing about what it might do later. Everything unbuilt — language-level or otherwise — lives
here, under "Deferred designs". Rationale and rejected alternatives live in commit messages and
in that section.

**Target: v0.1** — a released core that a real signal host (live feed → strategy → signals)
can be built on. Scope is a **cut**, not a build-out.

## Ground rules any new work must respect

- **The core is domain-blind.** It knows primitives, control flow, typed input/output ports,
  host-registered declaration kinds, host-type dispatch, and a **per-event** execution model
  (activation → `Run` → `emit`; a candle tick is just the signal block's special case).
  *When* `Run` fires is a host sync primitive, not a language feature.
- **The prelude registers no declaration kinds.** `const`/`input`/`output` are builtin forms;
  everything else — `state` included — is a host registration. A program using `state`
  compiles only against a host that registered the word. Do not add one to `DefaultRegistry`.
- **Core packages** (`registry`, `resolver`, `evaluator`, `lexer`, `parser`, `ast`,
  `resolved`, `token`) must never import talive or mention candles. All TA lives in
  `examples/` + tests. **TODO: add a CI import-guard.**
- **Shared-handle invariant** — host-registered types expose **read-only methods only**, and
  the language never grows `obj.field =` or `arr[i] =` for host-owned values (slot mutation
  is fine, slots are engine-owned). Three ways to break it, guard each: general member
  assignment beyond a namespaced slot's `kind.name`; index assignment when `[n]` lands (keep
  it read-only); a host method registered with side effects writing through the shared
  pointer.
- **`Resolve()` requires a clean parse** and is unguarded by contract — `Compile()` returns
  on parse diagnostics first. Resolver diag fixtures must therefore parse cleanly.
- **`Resolver.Resolve` always returns a possibly-partial program**; callers must check
  `Diagnostics()` before using it.
- The **capability model** is a design intent, not built (see "Deferred designs"). Nothing in
  v0.1 may assume it exists.

## v0.1 — what is left

Items 1–4 and 6–8 are done; what remains is item 5 and the API cuts.

**5. Make `Registry`'s maps private.** Exported maps mean every `Register*` guard is
advisory: a host can write `reg.Types[id] = TypeDef{Shape: ModuleShape}` and desync `Types`
from `Modules`, which resolves clean and then panics in `evalIdent` (the resolver reads
`Types`, the evaluator env is seeded from `Modules`). `RegisterType` rejects `ModuleShape`
now, but that only closes the API path. `resolver` and `evaluator/env.go:12` reach into
`reg.Modules` directly and must go through accessors. Also here: `CoerceRule.EvalType`
duplicates its key's `to`, unchecked.

**The emit arity diagnostic counts wrong.** It counts rule params against `call.Args[1:]`, so
`emit(sig, "up")` reports "expected 2 args, found 1" — right arithmetic, misleading against
what was typed.

**API surface, all cheaper before release than after:**

- `NewBuilder` unconditionally registers stdlib — make `math` / `time` opt-in (`WithMath()` /
  `WithTime()`). Note `registry.RegisterStdMath` is a *different thing* despite the name: it
  registers arithmetic/comparison operators on builtin scalars and stays unconditional. This
  is also the trigger for the ~37 discarded stdlib registration errors below.
- `Builder` re-exports registry methods one-by-one as passthroughs, so every new registry
  capability means another forwarder — the example needed five added at once, stopping only
  because it ran out of things to register. Consider exposing the registry directly, or a
  real option pattern, instead of mirroring it.
- **Every host `Value` must carry a `TypeID` field** by hand just to satisfy `TypeID()`, so
  the host threads IDs from registration into every constructed value (the `hostTypes` struct
  in `examples/signal`). Pure boilerplate — see if the interface can carry it instead.
- **`BindOutput` type-checks nothing.** `Sink.Emit` takes an untyped `Value`, so a sink
  written for one output shape and bound to a port of another fails at the sink's own type
  assertion rather than at bind. Output ports have a type contract on the script side and
  none on the host side. Independent of what types outputs accept.

### Closed since the last revision

- **6. The sharing contract** — settled as *host-owned, never copied*, written up in SPEC
  §4.5. No enforcement and no copying: the engine did not create the aliasing, it routes host
  code to host code. Covers all three facts — live pointer vs snapshot value, handle escape
  through `emit`, and the coherence window spanning the tick.
- **7. `state x: HostType`** — dissolved by `state` being host-registered. The core supplies
  the mechanism (`DeclKind.AllowedTypes`, empty = accept everything) and the policy is the
  host's to set. Documented as host-owned in SPEC §4.5.
- **Output port types** — any registered type, host types included. The language takes no
  position on what crosses an output port; §5.2's old serialisability rule (never
  implemented) is gone.
- **8. `SPEC.md` cut to what ships** — capabilities, slot policy, `[n]` history, snapshot /
  rollback, and the unenforced limits came out entirely. §6.4 reconciled against the 50
  codes `diag` actually defines. Added §5.1 for the general call-argument rules, which were
  specified nowhere. Removed `Null` (never implemented) and the event envelope (sinks receive
  a bare value).

## Smaller open items

- **`math.pow` can still return NaN** (`pow(-1.0, 0.5)`); it needs a two-argument domain rule,
  unlike the single-argument `sqrt`/`log` traps.
- **Two remaining recovery cascades.** A *genuine* type mismatch (not the error type) in a
  slot initializer still drops the whole `SlotDecl`, so later references report as undeclared;
  in a slot assignment it still returns `BadStmt`. Appending anyway would put a value in the
  tree whose type contradicts the slot's declared `T` — fix only alongside a decision on
  whether `resolved` may hold deliberately ill-typed recovery nodes.
- **Two `Env` implementations drifting** — resolver keys by `Symbol`, evaluator by `string`;
  resolver's `Get` special-cases `isTopLevel()` while the evaluator walks the parent chain
  plainly; `Symbol` is declared in `resolver.go`, not `env.go`. Align during the slot-based
  rework at the latest.
- **Resolver diagnostic order** — `RunFn` resolves before `InitFn`, so diagnostics come out in
  non-source order. Noted in SPEC §8 known gaps.
- **Resolver fuzzing needs a grammar-based generator of valid programs.** A `FuzzResolve` over
  the parser corpus is a slower `FuzzParse`: coverage from the whole fuzz function steers
  mutation toward lexer/parser edges, so almost nothing reaches the resolver. Revisit once
  v0.1 settles the API. The resolver's real protection is the `^`-marker diag tables, which
  assert exact diagnostics a fuzzer cannot.
- **`stdlib` discards ~37 registration errors.** Left deliberately: stdlib registers into a
  fresh registry before any host type, duplicates are rejected rather than overwritten, and an
  internal duplicate would fail the pipeline tests. **Revisit with `WithMath()` / `WithTime()`.**
- **Registered values are shared, not copied.** `RegisterCall` stores `rule.Args` as-is and
  `RegisterScriptType` stores `fields` as-is, so a host that mutates the slice it passed in
  mutates the registry. Harmless while a registry serves one executable; defensive copies at
  registration are the fix if that stops being true.
- **Do not delete `registry.VectorShape` or `resolved.IndexExpr`.** Both are unconstructed and
  both wait for indexing / the *historyable* capability.
- **Adjacent wart, no owner:** `ast.IdentExpr` implements `expressionNode()`, so all 10
  name-holding fields claim to hold an expression — but a decl's identifier, a kwarg key and
  the name after a dot are none of them evaluable. If worth fixing it is a dedicated
  `ast.Name{Token}` applied to all 10 sites, not a one-off.
- IDE-style best-effort resolution over invalid programs would need nil-tolerance in the
  resolver, against the clean-parse contract above. Decide if that is ever a goal.

## Deferred designs

None of this is implemented, so none of it belongs in `SPEC.md` — the spec states only what
the language does. Recorded here so the design is not re-derived.

### Language-level, cut from v0.1

**Type capabilities.** A closed vocabulary (`comparable`, `ordered`, `arithmetic`,
`indexable`, `historyable`, `has-methods`, `replaceable`, `snapshotable`) declared per type,
with assignability computed as `slotWritable(kind) && typeReplaceable(T)`. Half of it shipped:
`DeclKind.Assignable` is the kind gate. The type gate needs the capability data, which does
not exist. `RegisterType` also hardcodes `ScalarShape`, so `VectorShape` is unreachable from
outside. Its main payoff was rejecting non-snapshotable host objects in persistent slots, and
host-state consistency is now explicitly the host's problem, which removes the motivation.

**History and the `[n]` operator.** A runtime-owned ring-buffer window per `historyable`
value, sized by static analysis of the maximum literal lookback, with `x[0]` the current
value. Needs `HISTORY_OUT_OF_RANGE` and `HISTORY_LIMIT`. Dynamic indices (`x[state.i]`) stay
rejected to keep sizing tractable.

**Indexing and collections.** Tuples and arrays as core types, and `LookupIndex(receiver,
index)` slotting in where the `NOT_INDEXABLE` diagnostic is today (`resolver.go:589`) — success
builds the already-existing `resolved.IndexExpr`, failure keeps the diag. Deferred because it
is sugar over a registered call: `candles.at(0)` works today and forecloses nothing. Its one
real coupling is that `TypeID` is flat — a builtin collection would force parameterised type
IDs, at which point an index rule's result type stops being a static `EvalType`. That pressure
hits `RegisterCall`/`RegisterMemberAccess` identically, so indexing is not what exposes us to
it. Index assignment and slicing are additive on top of a read-only rule.

**Iteration.** `for x in candles` needs a loop form plus a host-registered iteration rule. The
expensive part is not the collection type but whether a per-tick script may loop at all, since
unbounded loops let a script hang the host (bounds or a step budget). Independent of indexing.

**Snapshot and rollback.** Tick transactionality for slots, plus a `Snapshotable` interface for
host objects. Killed rather than deferred: the engine has no rollback and the spec says so.
Revive only if a host turns out to want engine-side rollback after all.

**The emit envelope.** Sinks receive a bare value. An envelope carrying the activation
timestamp and port name should nest under a field (`event.ts`) rather than reserve bare names,
which avoids colliding with a payload schema's own field names entirely.

**Expression-form defaults.** `registry.ParamRule.Default` was deleted; re-add it **together
with** default-folding in `resolveArgs`, not before — `resolveArgs` enforces an exact arg
count, so a rule setting a default would hard-fail. A `Value` is enough for `period: 14`;
allowing `offset: length/2` means `Default` must be an `Expr` evaluated at the call site, and
crossing that line is the trigger.

**Warnings.** `diag` has no severity at all; `render` writes the literal `error`. The blocker
is not the field — counting diagnostics is load-bearing in three places that all assume
diagnostic == error: (i) the `errsBefore := len(r.errs)` dedup guards in the resolver (plus
`p.errCount` in the parser) mean "did this subtree already report something", so a warning in
the same slice suppresses a real diagnostic; (ii) the `newErrorExpr` guarantee assertion
`if len(r.errs) == 0 { panic }` would be **falsely satisfied** by a warning, silently minting
error types with no error behind them; (iii) `maxParseErrors` caps `len(p.errors)`, so warnings
would eat the error budget. Warnings need their **own slice**, not a severity field filtered at
the end — plus `render` taking severity from the diagnostic. First real warning is probably
`EMPTY_FUNCTION`.

**Unenforced limits.** Max string literal and runtime string length (4096), max identifier
length (128), max source size (256 KB), max arguments per call (32) — with codes
`STRING_LIMIT`, `IDENT_LIMIT`, `SOURCE_SIZE_LIMIT`, `KWARG_LIMIT`.

**`return`, and broader line continuation.** No early-return form; the substitute is wrapping
body code in a filter `if`. If real programs grow nested, `return` is a small addition.
Continuation is bracket-only today; broader trailing-token continuation would be decided by the
token at the **end** of a line, with one exception — keywords that can never begin a statement.
`else` is the only one, and adding to that set requires the same proof.

**Public coercion and subtyping.** `LookupCoerce(from, to)` is private; keep the *shape* so
going public is trivial — do **not** inline `if from == Int && to == Float` in the resolver.
Rules to honor when it goes public: at most **one** implicit edge per `(From, To)` pair,
rejected at registration; **no transitive chaining** (no auto `int→float→Price`); a per-edge
implicit vs explicit-only flag, since lossy conversions require a cast. `Implements(Sub, Super)`
is the sibling — same value, no transform. Per-function granularity comes from which type each
param declares; no union types. Where an expression's static type is an abstract supertype and
a member is overridden per concrete type, the resolver cannot bake a single `EvalFn`: emit a
`resolved.DynamicAccess{Recv, Method}` that looks up by `value.TypeID()` at eval time.
Monomorphic sites stay fully baked; only polymorphic sites pay. Note interface-typed indicators
need **none** of this — registering `Indicator` as an ordinary TypeID, having `ta.sma(...)`
declare `EvalType: Indicator`, and having the concrete struct report that ID resolves by plain
equality; verified by probe.

**Multi-way conversions = explicit named projections.** When a type has **more than one** way to
become another (`CandleSeries → Series` via close / open / hl2 / volume), it is neither a
coercion nor a subtype — it is a choice. Model as explicit accessors: `candles.close`,
`candles.hl2`. Implicit requires a *unique canonical* transform. **Open:** do candles get one
blessed default (implicit `close`) or zero default (always explicit)? Leaning **zero default**
— no hidden price selection in a strategy.

### State extensions

- **Batch decl form** — `state: { cooldown: Integer, … }`. Pure sugar over per-entry decls;
  add when a real script gets decl-heavy.
- **Record-typed entries** — `state pair: {a: Float, b: Float}`. Needs field-mutation
  semantics (`state.pair.a = 1`), which the shared-handle invariant currently forbids.
- **Purity flag on registry fns** — `const` and slot-initializer contexts accept any module
  call, so an impure host fn makes a "const" depend on the load moment. Accepted as the host's
  foot-gun; add `Pure bool` + a context check only if it bites. Subsumed by effect metadata.
- **Rejected, do not revisit casually: lookback semantics** (`state.x[1]`, ring buffer per
  bar). Plain persistent variables are wanted; series history covers the lookback need.

### Effect metadata on `CallRule`

Pure / mutates-receiver / cardinality / phase / ownership. Needed for safe stepping; closes
the "registry cannot express *this rule mutates*" foot-gun (same class as the impure-fn one
above). Prerequisite for the real TA surface.

### The TA surface

`indicator` / `series` / `step` / `sparse` declarations with real streaming semantics, `[n]`
history windows, warmup (prefeed vs replay), and clocks — on top of capabilities and effect
metadata. Note the `indicator` *keyword* already works as an ordinary host decl kind; none of
the rest of this is needed for that.

### Freshness / activation policy

A binding is permanent, so the evaluator cannot tell which inputs advanced since the last
tick — "fire when all inputs are fresh", "2 of 3", "why was `Run` called" have nowhere to read
from. Adding it means a host-set dirty flag, or replacing the bare bound value with a slot
handle.

### Input optimization ladder

Cheapest first: ring buffer → columnar struct-of-arrays (`[]float64` per field, likely what
talive wants) → opaque handle + column projections (`candles.close`, only touched columns
materialize). Binding already killed the per-tick map build; a slot array would kill the
remaining per-tick map lookup. The large-window case ultimately wants to be an engine-owned
rolling series the script indexes backward, not a per-tick input.

### Slot-based variable access (perf)

Assign each `let`/`const`/`input` a **slot index** and emit `resolved.Ident{Slot, Type}`
instead of a name; the evaluator reads `slots[n]` (~1–2 ns) instead of a string-keyed map
(~15–50 ns). The `Run` body is re-walked per event over thousands of candles, so this
compounds. Also turns undefined-variable into a resolve-time error and lets eval drop the
locals map. Resolver env entry becomes `Binding{Type, Slot}`; runtime env becomes `[]Value`.
Same pass: `evalMethodCall` builds a `map[string]Value` per call per bar — the rule already
knows param order, so bake a param index and pass positional `[]Value`.

**When:** after the resolved-tree migration is correct and stable.

### Lighter position info on resolved nodes

`resolved` nodes carry a full `token.Token`; after resolution `Literal`/`Type` are dead weight
and only *where to point* matters. Replace with a span:

```go
type Span struct{ From, To token.Pos }   // or a single Pos if point-precision is enough
```

Expose it uniformly on the node interface (`Span() Span`) so error reporting can ask any node
its position without knowing the concrete type. Whole-`resolved`-file change — do it in one
pass, touching every node and every resolver construction site. Pure weight reduction: it must
not change what error reporting can point at, and value-domain checks stay the only checks in
eval — do not let type re-checking creep back in. When this lands, honor that a decoded string
`Literal` is not the same width as its source text.

### Reserved-type unification

Fold type registration and the reserved error-type guard into one mechanism — a `Reserved bool`
field on `TypeDef`: register `ErrorTypeID` like any other type so the occupancy check protects
it, have `LookupType` skip reserved entries so scripts still cannot name it, and have rule
registration reject reserved operands/params/`EvalType`. Today these are two mechanisms
(builtins protected *by being in* `Types`, the error type by deliberately staying *out*). If
capabilities ever land, short-circuit the error type **before** capability lookups.

### Reusable builders

`Compile` spends the builder, so a host retrying a fixed script must rebuild and re-register.
A registry `Clone()` per compile was implemented and reverted: it worked, but `Clone` must
track every new `Registry` field with no compiler help, and it defended against a mutation
that should not happen. If reusable builders are ever wanted, the fix is a resolver-local type
table so the registry is never written during resolve — the evaluator needs script types for
one thing only (`len(def.Fields) == 0` at `evalEmit`), which the resolver already knows and
could bake into `resolved.EmitStmt`.
