# Design TODO — work queue

What is left to do, and nothing else. This file is not a record of what happened:
what shipped is in git, and the normative surface is `docs/SPEC.md` (core language).
Rationale and rejected alternatives live in commit messages and the "Deferred designs"
section below — `SPEC_SIGNAL_HOST.md` and `DESIGN_STREAMING_INDICATORS.md` were deleted
in `9d1de8b`, so the signal-block host has no spec yet.

**Target: v0.1** — a released core that a real signal host (live feed → strategy → signals)
can be built on. Scope is a **cut**, not a build-out: the capability/slot/snapshot queue
that used to head this file is deferred wholesale, because none of it changes what a script
can do. See "Not in v0.1" for what moved and why.

## Ground rules any new work must respect

- **The core is domain-blind.** It knows primitives, control flow, typed input/output
  ports, `state`, host-type dispatch, snapshot/rollback, and a **per-event** execution
  model (message on a port → `Run` → `emit`; a candle tick is just the signal block's
  special case). *When* `Run` fires is a host sync primitive, not a language feature.
- **Core packages** (`registry`, `resolver`, `evaluator`, `lexer`, `parser`, `ast`,
  `resolved`, `token`) must never import talive or mention candles. All TA lives in
  `examples/` + tests. **TODO: add a CI import-guard.**
- **Shared-handle invariant** — host-registered types expose **read-only methods only**,
  and the language never grows `obj.field =` or `arr[i] =` for host-owned values (`state`
  mutation is fine, state is engine-owned). Three ways to break it, guard each: general
  member assignment beyond `state.`; index assignment when `[n]` lands (keep it
  read-only); a host method registered with side effects writing through the shared
  pointer.
- **`Resolve()` requires a clean parse** and is unguarded by contract — `Compile()`
  returns on parse diagnostics first. Resolver diag fixtures must therefore parse cleanly.
- **`Resolver.Resolve` always returns a possibly-partial program**; callers must check
  `Diagnostics()` before using it.
- The **capability model** (per-type capabilities × per-binding slot policy) is a design
  intent, not built — see "Not in v0.1". Nothing in v0.1 may assume it exists.

## v0.1 — what ships

In order. 1–2 are the release; 3–4 are cheap and unblock the error seam; 5–7 are decisions
that get expensive to change once hosts exist; 8 is the doc.

**1. Output path — ~~replace `Emitted()`~~ done.** Outputs bind like inputs: the host calls
`BindOutput(name, sink)` before `Init` (every declared output needs one, or `Init` fails
with `OUTPUT_MISSING`), and `evalEmit` calls `Sink.Emit(Value)` as it evaluates. `Emitted()`
and the emit buffer are gone; `Run() error` no longer returns a debug value. Delivery policy
— buffer-and-flush, fire-and-forget, drop during warmup — is entirely the sink's, which is
why a `log` output and an order router can coexist. Emits are therefore **outside tick
rollback** (§6.5 rewritten). What is left here:

- **The emit event envelope.** Prefer nesting (`event.ts`) over reserving bare names, which
  avoids the collision entirely.
- The emit arity diag counts rule params against `call.Args[1:]`, so `emit(sig, "up")` says
  "expected 2 args, found 1" — right arithmetic, misleading against what was typed.
- Sinks take no error return: the host owns the transport and already knows when it failed.
  Revisit only if a sink failure needs to reach the script.
- A sink runs inside the tick, so it must not call back into the executable — unenforced,
  documented on `registry.Sink`.

**2. Host-registerable declaration keywords** (was item D). Contextual keyword +
declaration-form registry; a parser refactor. This is where `indicator rsi: Scalar =
ta.rsi(14)` comes from, and where `state` / `input` / `output` fold into one generic
mechanism as prelude registrations.

- Measured first: `const fast = ta.sma(3, source = ta.Close)` already compiles, runs, keeps
  the host object across ticks, and takes method calls mid-`Run`. So this buys **vocabulary,
  not capability** — size it accordingly.
- The one semantic difference worth deciding while here: a `const`-held host object lives in
  `env`, so a trapped tick rolls back `state` and emits but leaves the indicator advanced.
  Decided: host's problem — but say so in the spec rather than leaving it silent.

**3. Guard the single-use builder.** `resolveTypeDecl` writes synthesized inline port types
(`input.x` / `output.sig`) into the registry, but the duplicate guard reads the per-pass env
— so a second `Compile()` on one Builder collides and reports it as a *misleading script
error* (`ARG_COUNT_MISMATCH: expected 1 args, found 2`, because the structural type resolves
with no fields). Decided: **one builder = one script = one executable**, so this is a guard
on the `Compile` **error** return, not a design fix.

- The same guard closes a second hole: builder and executable share one registry, so any
  `Register*` after `Compile` mutates the **live** executable's vocabulary. The host never
  gets a `*Registry` (nothing exported returns one), but it keeps the builder.
- This is also the **first producer of `Compile`'s error return**. The seam exists —
  `(*Executable, []diag.Diagnostic, error)`, script problems vs host/API misuse — but
  `len(diags) > 0` is still the failure predicate in `prog.Valid` and the fuzz assertion.
- A registry `Clone()` per compile was implemented and reverted: it worked, but `Clone` must
  track every new `Registry` field with no compiler help, and it defended against a mutation
  that should not happen. If reusable builders are ever wanted, the fix is a resolver-local
  type table so the registry is never written during resolve (the evaluator needs script
  types for one thing only: `len(def.Fields) == 0` at `evalEmit`, which the resolver already
  knows and could bake into `resolved.EmitStmt`).

**4. `resolveTypeDecl` swallows the `RegisterScriptType` error** (`resolver.go:347`) and
returns `ErrorTypeID` with no diagnostic. Its comment claims the branch is unreachable
because duplicate decl names are caught by the env check — true within one compile, false
across two, which is how #3's collision stays silent. Either emit a real diagnostic or use
internal type IDs that cannot collide.

**5. Make `Registry`'s maps private.** Exported maps mean every `Register*` guard is
advisory: a host can write `reg.Types[id] = TypeDef{Shape: ModuleShape}` and desync `Types`
from `Modules`, which resolves clean and then panics in `evalIdent` (the resolver reads
`Types`, the evaluator env is seeded from `Modules`). `RegisterType` rejects `ModuleShape`
now, but that only closes the API path. Resolver and evaluator reach into `reg.Modules`
directly and must go through accessors. Also here: `CoerceRule.EvalType` duplicates its
key's `to`, unchecked.

**6. Decide the sharing contract, then write it down.** Three facts that are currently
invisible at the API:

- **Handle escape.** `emit(out, candles)` hands the live input handle to the sink
  (structured emits allocate a fresh outer `Record`, but field values are still shared).
  The sink now sees it mid-tick, when it is still coherent — but a sink that *retains* it
  past the tick is reading data the feed keeps mutating. Forbid, copy, or document.
- **A bound pointer is live, a bound value is a snapshot**, decided by the host's Go type
  and invisible in `BindInput`'s signature.
- The coherence window widened with binding: a bound value must hold still from bind until
  `Run` returns, not merely for one call.

**7. Decide `state x: HostType`.** Accepted today for any registered type, and rollback is a
shallow `maps.Clone`, so `state.saved = candles` persists a live host handle whose internals
never roll back. Reject at load, or document as host-owned. No snapshot machinery — just
stop it being silent.

**8. Cut `SPEC.md` to what ships.** Finishing the design doc means *removing* what v0.1 does
not implement, not implementing it: the capability model, slot policy, and `[n]` history
windows move to a deferred appendix. Includes the **§6.4 diagnostics reconciliation** (was
tracker row #14), which is unavoidable here because the spec names codes the impl does not
have:

- ~10 spec codes have no impl (`BOOL_REQUIRED`, `EMIT_PAYLOAD`, `OUTPUT_NOT_WIRED`,
  `HISTORY_*`, most of the `*_LIMIT` family); ~24 impl codes have no spec entry. Naming
  conflicts to settle: `PORT_DUPLICATE` vs `DUPLICATE_DECLARATION`, `UNKNOWN_OUTPUT` vs
  `INVALID_EMIT_TARGET`, `TOP_LEVEL_FORM` vs `TOP_DECL_UNEXPECTED`, `INTERNAL` vs
  `INTERNAL_FAILURE`.
- Does `BOOL_REQUIRED` split out of `TYPE_MISMATCH` for `if` / `&&` / `||`? The spec assumes
  it does; the impl does not.
- Do `UNDEFINED_IDENT`/`UNDEFINED_VAR` and `UNDEFINED_ATTRIBUTE`/`UNDEFINED_METHOD` merge
  under the "a code is a user-facing category, not a 1:1 class id" rule?
- `EMPTY_FUNCTION` — decide whether an empty `Run` is illegal at all. The spec never says.
- The **spec gap** on call arguments: the general call-argument rules are specified nowhere,
  existing only implicitly in the §6.4 error rows with §5.2 stating them inline. Worth a real
  §5.x, and v0.1 is when someone reads it.

**Also worth settling before release** (exported signatures get expensive to change after):
`NewBuilder` unconditionally registers stdlib — make `math` / `time` opt-in (`WithMath()` /
`WithTime()`); note `registry.RegisterStdMath` is a *different thing* despite the name, it
registers arithmetic/comparison operators on builtin scalars and stays unconditional. Plus
the two API warts under "Public API surface": the passthrough mirror, and every host `Value`
carrying a `TypeID` field by hand.

## Not in v0.1

Moved out because none of it changes what a script can express. Recorded so the reasoning
is not re-derived.

**Type capabilities (was A).** Capability metadata on `registry.TypeDef` (`storable`,
`replaceable`, `snapshot`: value-copy|host|none, `historyable`). Pure data, no behavior
change. Its main payoff was rejecting non-snapshotable host objects in persistent slots —
and rollback of host state is now the host's problem (v0.1 item 7), which removes the
motivation. Revive it with F, or if a gate below turns out to be needed.

- While in `TypeDef` anyway: fold type registration and the reserved error-type guard into
  one mechanism — a `Reserved bool` field: register `ErrorTypeID` like any other type so the
  occupancy check protects it, have `LookupType` skip reserved entries so scripts still
  cannot name it, and have rule registration reject reserved operands/params/`EvalType`.
  Today these are two mechanisms (builtins protected *by being in* `Types`, the error type by
  deliberately staying *out*).
- Short-circuit the error type **before** capability lookups, so
  `typeReplaceable(ErrorTypeID)` never reports while `slotWritable(kind)` still can.
- **`RegisterType` hardcodes `ScalarShape`**, so a host cannot register any other shape and
  `VectorShape` is unreachable from outside. Blocks indexing and these capabilities.

**Slot-policy two-gate (was B).** Generalize `resolver.Binding.Assignable()` into
`canWrite = slotWritable(kind) && typeReplaceable(T)`. Needs A.

**Persistent host-object slots + snapshot generalization (was C).** A slot table holding
host `Value`s across ticks plus a `Snapshotable` interface. Was billed as "the real enabler
for indicators", but indicators need slots that *hold* host values across ticks — which
`const` already does — not slots that snapshot them. Mostly rollback work; see item 7.

**Effect metadata on `CallRule` (was E)** — pure / mutates-receiver / cardinality / phase /
ownership. Needed for safe stepping; closes the "registry cannot express *this rule mutates*"
foot-gun (same class as the impure-fn one under deferred state extensions).

**`indicator` / `series` / `step` / `sparse`, `[n]` history windows, warmup (prefeed vs
replay), clocks (was F)** — the real TA surface, on top of A–E. Note v0.1 item 2 delivers the
`indicator` *keyword* without any of this.

**Indexing (`candles[0]`).** `LookupIndex(receiverType, indexType)` slots in where the
`NOT_INDEXABLE` diag is today (`resolver.go:589`): success builds the already-existing
`resolved.IndexExpr`, failure keeps the diag. Deferred because it is **sugar over a
registered call** — `candles.at(0)` works today and forecloses nothing. Its one real coupling
is `TypeID` being flat: a builtin collection would force parameterized type IDs, at which
point an index rule's result type stops being a static `EvalType`. That pressure hits
`RegisterCall`/`RegisterMemberAccess` identically, so indexing is not what exposes us to it.
Index *assignment* and slicing are additive on top of a read-only rule.

**Iteration (`for x in candles`).** Needs a loop form plus a host-registered iteration rule.
The expensive part is not the collection type — it is whether a per-tick script may loop at
all, since unbounded loops let a script hang the host (bounds or a step budget). Independent
of indexing.

**Freshness / activation policy.** A binding is permanent, so the evaluator cannot tell which
inputs advanced since the last tick — "fire when all inputs are fresh", "2 of 3", "why was
`Run` called" have nowhere to read from. Adding it means a host-set dirty flag, or replacing
the bare bound value with a slot handle.

**Input optimization ladder**, cheapest first: ring buffer → columnar struct-of-arrays
(`[]float64` per field, likely what talive wants) → opaque handle + column projections
(`candles.close`, only touched columns materialize). Binding already killed the per-tick map
build; a slot array would kill the remaining per-tick map lookup. The large-window case
ultimately wants to be an engine-owned rolling series the script indexes backward (spec §4.2),
not a per-tick input.

**Warnings.** `diag` has no severity at all; `render` writes the literal `error`. The blocker
is not the field — counting diagnostics is load-bearing in three places that all assume
diagnostic == error: (i) the `errsBefore := len(r.errs)` dedup guards in the resolver (plus
`p.errCount` in the parser) mean "did this subtree already report something", so a warning in
the same slice suppresses a real diagnostic; (ii) the `newErrorExpr` guarantee assertion
`if len(r.errs) == 0 { panic }` would be **falsely satisfied** by a warning, silently minting
error types with no error behind them; (iii) `maxParseErrors` caps `len(p.errors)`, so
warnings would eat the error budget. So warnings need their **own slice**, not a severity
field filtered at the end — plus `render` taking severity from the diagnostic. First real
warning is probably `EMPTY_FUNCTION`.

## Smaller open items

- **`math.pow` can still return NaN** (`pow(-1.0, 0.5)`); it needs a two-argument domain
  rule, unlike the single-argument `sqrt`/`log` traps.
- **Two remaining recovery cascades.** A *genuine* type mismatch (not the error type) in a
  state initializer still `continue`s and drops the field; in a state assignment it still
  returns `BadStmt`, which `definitelyAssignedState` does not count. Appending anyway would
  put a value in the tree whose type contradicts the field's declared `T` — fix only
  alongside a decision on whether `resolved` may hold deliberately ill-typed recovery nodes.
- **Two `Env` implementations drifting** — resolver keys by `Symbol`, evaluator by `string`;
  resolver's `Get` special-cases `isTopLevel()` while the evaluator walks the parent chain
  plainly; `Symbol` is declared in `resolver.go`, not `env.go`. Align during the slot-based
  rework at the latest.
- **Resolver diagnostic order** — `RunFn` resolves before `InitFn`, so diagnostics come out
  in non-source order.
- **Resolver fuzzing needs a grammar-based generator of valid programs.** A `FuzzResolve`
  over the parser corpus is a slower `FuzzParse`: coverage from the whole fuzz function
  steers mutation toward lexer/parser edges, so almost nothing reaches the resolver.
  Revisit once v0.1 settles the API. Note the resolver's real protection is the
  `^`-marker diag tables, which assert exact diagnostics a fuzzer cannot.
- **`stdlib` discards ~37 registration errors.** Left deliberately: stdlib registers into a
  fresh registry before any host type, duplicates are rejected rather than overwritten, and
  an internal duplicate would fail the pipeline tests. **Revisit when stdlib registration
  becomes conditional** — i.e. with `WithMath()` / `WithTime()` above.
- **Registered values are shared, not copied.** `RegisterCall` stores `rule.Args` as-is and
  `RegisterScriptType` stores `fields` as-is, so a host that mutates the slice it passed in
  mutates the registry. Harmless while a registry serves one executable; defensive copies at
  registration are the fix if that ever stops being true.
- **Do not delete `registry.VectorShape` or `resolved.IndexExpr`.** Both are unconstructed
  and both wait for indexing / the *historyable* capability.
- **Adjacent wart, no owner:** `ast.IdentExpr` implements `expressionNode()`, so all 10
  name-holding fields claim to hold an expression — but a decl's identifier, a kwarg key
  and the name after a dot are none of them evaluable. If worth fixing it is a dedicated
  `ast.Name{Token}` applied to all 10 sites, not a one-off.
- IDE-style best-effort resolution over invalid programs would need nil-tolerance in the
  resolver, against the clean-parse contract above. Decide if that is ever a goal.

## Deferred designs

### State extensions

- **Batch decl form** — `state: { cooldown: Integer, … }`. Pure sugar over per-entry
  decls; add when a real script gets decl-heavy.
- **Record-typed entries** — `state pair: {a: Float, b: Float}`. Needs field-mutation
  semantics (`state.pair.a = 1`); v1 rejects at resolve time.
- **Purity flag on registry fns** — const and state-initializer contexts accept any module
  call, so an impure host fn makes a "const" depend on the load moment. Accepted as the
  host's foot-gun; add `Pure bool` + a context check only if it bites. Subsumed by item E.
- **Rejected, do not revisit casually: lookback semantics** (`state.x[1]`, ring buffer per
  bar). Plain persistent variables are wanted; series history covers the lookback need.

### Slot-based variable access (perf)

Assign each `let`/`const`/`input` a **slot index** and emit `resolved.Ident{Slot, Type}`
instead of a name; the evaluator reads `slots[n]` (~1–2 ns) instead of a string-keyed map
(~15–50 ns). The `Run` body is re-walked per event over thousands of candles, so this
compounds. Also turns undefined-variable into a resolve-time error and lets eval drop the
locals map. Resolver env entry becomes `Binding{Type, Slot}`; runtime env becomes
`[]Value`. Same pass: `evalMethodCall` builds a `map[string]Value` per call per bar — the
rule already knows param order, so bake a param index and pass positional `[]Value`.

**When:** after the resolved-tree migration is correct and stable.

### Public coercion API (`Coerce` / `Implements`)

`int → float` is a single private registry edge today. Keep the *shape*
(`LookupCoerce(from, to) → (rule, ok)`) so going public is trivial — do **not** inline
`if from == Int && to == Float` in the resolver. Expose `reg.Coerce` / `reg.Implements`
only when a real custom-type need appears. Rules to honor then:

- At most **one** implicit edge per `(From, To)` pair — reject a second at registration.
- **No transitive chaining** — single hop only (no auto `int→float→Price`).
- Per-edge `implicit` vs `explicit-only` flag (lossy conversions require a cast).

### Subtyping + dynamic dispatch

`Implements(Sub, Super)` = same value, no transform (vs `Coerce` = the value changes).
Per-function granularity comes from which type each param declares — no union types.
When an expression's static type is an abstract supertype and a member is overridden per
concrete type, the resolver cannot bake a single `EvalFn`: emit a
`resolved.DynamicAccess{Recv, Method}` that looks up by `value.TypeID()` at eval time.
Monomorphic sites stay fully baked; only polymorphic sites pay the lookup. The concrete
types (`CandleSeries <: Series`) are host concerns — keep the mechanism core.

Note: interface-typed indicators need **none** of this. Registering `Indicator` as an
ordinary TypeID, having `ta.sma(...)` declare `EvalType: Indicator`, and having the concrete
struct report that ID resolves by plain equality — verified by probe.

### Multi-way conversions = explicit named projections

When a type has **more than one** way to become another (`CandleSeries → Series` via
close / open / hl2 / volume), it is neither a coercion nor a subtype — it is a choice.
Model as explicit accessors: `candles.close`, `candles.hl2`. Implicit requires a *unique
canonical* transform.

**Open decision:** do candles get one blessed default (implicit `close`, explicit rest) or
zero default (always explicit, `sma(candles.close)`)? Leaning **zero default** — no hidden
price selection in a strategy.

### Expression-form defaults

`registry.ParamRule.Default` was deleted; re-add it **together with** default-folding in
`resolveArgs`, not before (`resolveArgs` enforces an exact arg count, so a rule setting a
default would hard-fail). A `Value` is enough for `period: 14`; allowing
`offset: length/2` means `Default` must be an `Expr` evaluated at the call site — crossing
that line is the trigger.

### Lighter position info on resolved nodes

`resolved` nodes carry a full `token.Token`; after resolution `Literal`/`Type` are dead
weight and only *where to point* matters. Replace with a span:

```go
type Span struct{ From, To token.Pos }   // or a single Pos if point-precision is enough
```

Expose it uniformly on the node interface (`Span() Span`) so error reporting can ask any
node its position without knowing the concrete type. Whole-`resolved`-file change — do it
in one pass, touching every node and every resolver construction site. Pure weight
reduction: it must not change what error reporting can point at, and value-domain checks
stay the only checks in eval — do not let type re-checking creep back in. When this lands,
honor that a decoded string `Literal` is not the same width as its source text.

### Public API surface (`Builder` / `Executable` / `tascript.go`)

Lifecycle is settled: `NewBuilder()` → `Register*` → `Compile(src)` → `BindInput(...)` →
`Init()` → `Run()` per activation, with an `Executable.Stage` machine
(`created → initialized | failed`) making "what is allowed right now" answerable. One
builder, one script, one executable; the package is single-threaded by contract. What is
left, all of it cheaper before release than after:

- `Builder` re-exports registry methods one-by-one as passthroughs, so every new registry
  capability means another forwarder — and the example needed five added at once, stopping
  only because it ran out of things to register. Consider exposing the registry directly, or
  a real option/builder pattern, instead of mirroring it.
- **Every host `Value` must carry a `TypeID` field** just to satisfy `TypeID()`, so the host
  threads IDs from registration into every constructed value (the `hostTypes` struct in
  `examples/signal`). Pure boilerplate — see if the interface can carry it instead.
- **Stdlib modules are always registered** — see the v0.1 "also worth settling" note.
