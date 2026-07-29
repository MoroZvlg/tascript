# Design TODO — work queue

What is left to do, and nothing else. This file is not a record of what happened:
what shipped is in git, the normative surface is `docs/SPEC.md` (core) +
`docs/SPEC_SIGNAL_HOST.md` (signal-block host), and rationale / rejected alternatives
live in `docs/DESIGN_STREAMING_INDICATORS.md`.

## Ground rules any new work must respect

- **The core is domain-blind.** It knows primitives, control flow, typed input/output
  ports, `state`, host-type dispatch, snapshot/rollback, and a **per-event** execution
  model (message on a port → `Run` → `emit`; a candle tick is just the signal block's
  special case). *When* `Run` fires is a host sync primitive, not a language feature.
- **Core packages** (`registry`, `resolver`, `evaluator`, `lexer`, `parser`, `ast`,
  `resolved`, `token`) must never import talive or mention candles. All TA lives in
  `examples/` + tests. **TODO: add a CI import-guard.**
- **Capability model** — two orthogonal axes: *type capabilities* (per TYPE:
  comparable / ordered / arithmetic / historyable / snapshotable / replaceable /
  has-methods — a **closed** core vocabulary, while types themselves are host-registered)
  and *slot policy* (per BINDING: read-only / assignable / rebindable / fixed).
  Assignability = `slotWritable(kind) && typeReplaceable(T)`. `state` is not hardcoded —
  it is a standard prelude registration over the persistent-slot mechanism.
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

## Core work items — do these first, in order

Reusable by every block type.

**A. Type capabilities.** Add capability metadata to `registry.TypeDef` (`storable`,
`replaceable`, `snapshot`: value-copy|host|none, `historyable`) and populate the builtin
scalars. Pure data, no behavior change; unblocks the gates below.

- While in `TypeDef` anyway, consider folding type registration and the reserved
  error-type guard into one mechanism — a `Reserved bool` field: register `ErrorTypeID`
  like any other type so the occupancy check protects it, have `LookupType` skip reserved
  entries so scripts still cannot name it, and have rule registration reject reserved
  operands/params/`EvalType`. Today these are two mechanisms (builtins protected *by being
  in* `Types`, the error type by deliberately staying *out*). Only worth it if capabilities
  give `TypeDef` real structure, or a host needs internal types scripts cannot name.
- Short-circuit the error type **before** capability lookups, so
  `typeReplaceable(ErrorTypeID)` never reports while `slotWritable(kind)` still can.
- `LookupIndex(receiverType, indexType)` slots in where the `NOT_INDEXABLE` diag is today:
  success builds the already-existing `resolved.IndexExpr`, failure keeps the diag.

**B. Slot-policy two-gate.** Generalize `resolver.Binding.Assignable()` into
`canWrite = slotWritable(kind) && typeReplaceable(T)`; route `input` / host-object
assignment rejection through it. **Do not refactor `state` yet** — leave `resolved.State`
working and converge it onto the generic slot in item D, once indicators exist to validate
the shape.

**C. Persistent host-object slots + snapshot generalization.** Generalize the state
snapshot into a slot table holding host `Value`s across ticks; add a `Snapshotable`
interface host types implement, or reject-at-load when a host object in a persistent slot
cannot snapshot. **This is the real enabler for indicators.**

## Signal-block host — build when you start the host, not before

**D. Host-registerable declaration keywords.** Contextual keyword + declaration-form
registry (parser refactor). This is when `state` / `input` / `output` fold into the
generic mechanism as prelude registrations. Overlaps #13.

**E. Effect metadata on `CallRule`** — pure / mutates-receiver / cardinality / phase /
ownership. Needed for safe stepping, and closes the "registry cannot express *this rule
mutates*" foot-gun (same class as the impure-fn one under deferred state extensions).

**F. `indicator` / `series` / `step` / `sparse`, `[n]` history windows, warmup (prefeed vs
replay), clocks** — all on top of A–E.

## Open tracker rows

| # | Prio | Area | Work |
|---|------|------|------|
| 11 | P2 | resolver | Reserved-name / shadowing rules |
| 13 | P2 | both | Engine/host API rework |
| 14 | P2 | diag | §6.4 spec/impl reconciliation + warnings |

**#11 — reserved-name / shadowing rules.** All of these are currently accepted with no
diagnostic and each needs a decision:

- `let math = 5` shadows the module inside a function (spec says `RESERVED_REASSIGN`).
- `let m = math` then `m.sqrt(9.0)` — modules are first-class aliasable values, but spec
  §5.4 calls them passive syntactic prefixes, so a bare module ident in value position
  should diag.
- `const Init = 5` coexists with `function Init()` — function names never enter the
  top-level env, against spec §3.3's one namespace.
- Same-scope `let x` redeclaration silently rebinds, even with a new type. JS errors on
  this; the spec is silent.

**#13 — Engine/host API rework.** Replaces both temporary APIs: the `Emitted()` emission
sink and the `EvalRun(frame map[string]Value)` map stopgap (→ slot `[]Value` frame).
Design it around the end-to-end fixture: struct input → compute → emit. Also owns:

- **Registry purge-on-reuse.** `resolveTypeDecl` writes synthesized inline port types
  (`input.x` / `output.sig`) into the host-owned registry, but the duplicate guard reads
  the per-pass env — so a second `Compile()` on one Engine, or two scripts sharing one
  registry, collides. Scope synthesized types per compile (or purge on reuse) rather than
  papering over it with a diagnostic.
- **The emit event envelope.** Prefer nesting (`event.ts`) over reserving bare names,
  which avoids the collision entirely.
- **`Compile()` returning an executable *and* diagnostics.** Today `len(diags) > 0` *is*
  the failure predicate, in `NewEngine`, `Compile`, `prog.Valid` and the fuzz assertion.
  Same seam the warnings work needs.
- **Handle-escape decisions.** `function Run() { candles }` returns the live input handle,
  and `emit(out, candles)` emits it (structured emits allocate a fresh outer `Record`, but
  field values are still shared). If the host retains either past the tick it is no longer
  a snapshot. Decide whether the API forbids, copies, or documents this.
- **State is not scalar-only.** `state x: NamedType` accepts any registered type, so once
  a `CandleSeries` constructor exists, `state.saved = candles` persists a live host handle
  across ticks and shallow `maps.Clone` rollback would not cover its internals. Ties to
  item C.
- **Frame optimization ladder**, cheapest first: ring buffer → reuse the handle across
  ticks → columnar struct-of-arrays (`[]float64` per field, likely what talive wants) →
  opaque handle + column projections (`candles.close`, only touched columns materialize) →
  slot-array frame (kills map hashing). The large-window case ultimately wants to be an
  engine-owned rolling series the script indexes backward (spec §4.2), not a per-tick input.
- Absorbs item D and the `examples/` signal-host packaging. See also "Public API surface".

**#14 — diagnostics: §6.4 reconciliation.** Blocked on #11/#12/#13, whose behaviours the
unwritten codes name.

- **Reconcile spec against impl.** ~11 spec codes have no impl (`BOOL_REQUIRED`,
  `EMIT_PAYLOAD`, `RESERVED_REASSIGN`, `OUTPUT_NOT_WIRED`, `HISTORY_*`, most of the
  `*_LIMIT` family); ~24 impl codes have no spec entry. Naming conflicts to settle:
  `PORT_DUPLICATE` vs `DUPLICATE_DECLARATION`, `UNKNOWN_OUTPUT` vs `INVALID_EMIT_TARGET`,
  `TOP_LEVEL_FORM` vs `TOP_DECL_UNEXPECTED`, `INTERNAL` vs `INTERNAL_FAILURE`.
- Does `BOOL_REQUIRED` split out of `TYPE_MISMATCH` for `if` / `&&` / `||`? The spec
  assumes it does; the impl does not.
- Do `UNDEFINED_IDENT`/`UNDEFINED_VAR` and `UNDEFINED_ATTRIBUTE`/`UNDEFINED_METHOD` merge
  under the "a code is a user-facing category, not a 1:1 class id" rule?
- `EMPTY_FUNCTION` — decide whether an empty `Run` is illegal at all. The spec never says.
- **Warnings.** `diag` has no severity at all; `render` writes the literal `error`. The
  blocker is not the field — counting diagnostics is load-bearing in three places that all
  assume diagnostic == error: (i) the `errsBefore := len(r.errs)` dedup guards in the
  resolver (plus `p.errCount` in the parser) mean "did this subtree already report
  something", so a warning in the same slice suppresses a real diagnostic; (ii) the
  `newErrorExpr` guarantee assertion `if len(r.errs) == 0 { panic }` would be
  **falsely satisfied** by a warning, silently minting error types with no error behind
  them; (iii) `maxParseErrors` caps `len(p.errors)`, so warnings would eat the error
  budget. So warnings need their **own slice**, not a severity field filtered at the end —
  plus the `Compile()` seam above, and `render` taking severity from the diagnostic.
  First real warning is probably `EMPTY_FUNCTION` or a #11 shadowing rule.
- The emit arity diag counts rule params against `call.Args[1:]`, so `emit(sig, "up")`
  says "expected 2 args, found 1" — right arithmetic, misleading against what was typed.

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
- **Spec gap:** the general call-argument rules are specified nowhere — they exist only
  implicitly in the §6.4 error rows, with §5.2 stating them inline. Worth a real §5.x when
  someone next touches call syntax.
- **Resolver fuzzing needs a grammar-based generator of valid programs.** A `FuzzResolve`
  over the parser corpus is a slower `FuzzParse`: coverage from the whole fuzz function
  steers mutation toward lexer/parser edges, so almost nothing reaches the resolver.
  Revisit after #13 settles the API. Note the resolver's real protection is the
  `^`-marker diag tables, which assert exact diagnostics a fuzzer cannot.
- **`stdlib` discards ~37 registration errors.** Left deliberately: stdlib registers into a
  fresh registry before any host type, duplicates are rejected rather than overwritten, and
  an internal duplicate would fail the pipeline tests. **Revisit if hosts can register
  before stdlib, or stdlib registration becomes conditional** — which is exactly what #13
  should pin down.
- **Do not delete `registry.VectorShape` or `resolved.IndexExpr`.** Both are unconstructed
  and both wait for the *historyable* capability (item A), when `LookupIndex` starts
  building the node.
- **Adjacent wart, no owner:** `ast.IdentExpr` implements `expressionNode()`, so all 10
  name-holding fields claim to hold an expression — but a decl's identifier, a kwarg key
  and the name after a dot are none of them evaluable. If worth fixing it is a dedicated
  `ast.Name{Token}` applied to all 10 sites, not a one-off.

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

### Public API surface (`Engine` / `Executable` / `tascript.go`)

The top-level wiring needs a design pass:

- `Engine` re-exports registry methods one-by-one as passthroughs, so every new registry
  capability means another forwarder. Consider exposing the registry directly, or a
  dedicated builder, instead of mirroring it.
- The `Engine` → `Compile()` → `Executable` split and lifecycle is unmotivated: when do you
  register types vs compile vs run, and what is reusable across runs?
- Settle a coherent vocabulary for the phases (parse → resolve/compile → execute per event).
- Define the intended user flow end-to-end first (register custom types/funcs → compile →
  feed events → read outputs), then shape the types around it.
- **Registry is half-encapsulated:** exported maps (`Binary`, `Types`, `Modules`, …) coexist
  with Register/Lookup methods, and resolver + evaluator reach into `reg.Modules` directly.
  `CoerceRule.EvalType` duplicates its key's `to`, unchecked.
- IDE-style best-effort resolution over invalid programs would need nil-tolerance in the
  resolver, against the clean-parse contract above. Decide if that is ever a goal.
