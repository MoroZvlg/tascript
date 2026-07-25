# Design TODO — work queue & deferred ideas

Things we deliberately decided *not* to build yet, with enough context to pick them
up later — plus, merged in as of 2026-07-05, the findings of the full project review
(formerly `docs/REVIEW_FINDINGS.md`). Behavioral claims marked "verified" were
reproduced by running real scripts through the parse → resolve → eval pipeline.

## Architecture rework (2026-07-25) — target redefined, read first

A long design session redefined what tascript *is*. **Authoritative surface:
`docs/SPEC.md`** (core language) + **`docs/SPEC_SIGNAL_HOST.md`** (signal-block host). The
**why / rejected alternatives / open decisions** are in
**`docs/DESIGN_STREAMING_INDICATORS.md`** (a rationale/decision log, no longer normative).
Read the specs before picking up new work. Summary of what changed and how it reshapes
this queue:

**tascript is NOT a TA-DSL.** It is the shared language inside many *block types* on a
visual whiteboard (signal / filter / trading / SL-TP). Each block type is a **host**
embedding the same core with a different registered vocabulary. Therefore:

- **The core is domain-blind.** It knows primitives, control flow, typed input/output
  ports, `state`, host-type dispatch (+ future effect metadata), snapshot/rollback, and a
  **per-event** execution model (a message on a port → `Run` → `emit`; the candle tick is
  just the signal block's special case). *When* `Run` fires is a host sync primitive, not
  a language feature.
- **Indicators are NOT a language feature.** They are the *signal-block host's* vocabulary:
  a host-registered type + a host-registered declaration keyword. Everything TA
  (`indicator`/`series`/`step`/warmup/clocks) lives in ONE host, built later.
- **All TA/talive code lives in `examples/` + tests ONLY.** Core packages (`registry`,
  `resolver`, `evaluator`, `lexer`, `parser`, `ast`, `resolved`, `token`) must never
  import talive or mention candles. (Add a CI import-guard later.)

**Capability model (the mechanism everything rides on):** two orthogonal axes —
*type capabilities* (per TYPE: comparable/ordered/arithmetic/historyable/snapshotable/
replaceable/has-methods — a **closed** core vocabulary; types are open/host-registered)
and *slot policy* (per BINDING: read-only/assignable/rebindable/fixed). Assignability =
`slotWritable(kind) AND typeReplaceable(T)` — this is why "safety from type alone" was
wrong; binding kinds survive as slot policy. `state` is not hardcoded — it is a **standard
prelude** registration over the persistent-slot mechanism (included by every host).

**New CORE work items (reusable by every block type), in order — do AFTER the in-flight
frame (#7) and the P1 hole batch are stable:**

- **A. Type capabilities** — add capability metadata fields to `registry.TypeDef`
  (`storable`, `replaceable`, `snapshot`: value-copy|host|none, `historyable`); populate
  the builtin scalars. Pure data, no behavior change; unblocks the gates.
- **B. Slot-policy two-gate** — generalize `resolver.Binding.Assignable()` into
  `canWrite = slotWritable(kind) && typeReplaceable(T)`; route `input`/host-object
  assignment rejection through it. **Do NOT refactor `state` yet** — leave `resolved.State`
  working; converge it onto the generic slot in item D, once indicators exist to validate
  the shape.
- **C. Persistent host-object slots + snapshot generalization** — generalize the
  state-snapshot into a slot table that holds host `Value`s across ticks; add a
  `Snapshotable` interface host types implement (or reject-at-load if a host object in a
  persistent slot can't snapshot — see input-wiring residual note #3). **This is the real
  enabler for indicators.**

**Deferred to the signal-block host (build when you start it, NOT before):**

- **D. Host-registerable declaration keywords** — contextual keyword + declaration-form
  registry (parser refactor); this is when `state`/`input`/`output` fold into the generic
  mechanism as prelude registrations. Overlaps tracker #13 (engine/host API rework).
- **E. Effect metadata on `CallRule`** (pure/mutates-receiver/cardinality/phase/ownership)
  — needed for safe stepping. Closes the "registry can't express 'this rule mutates'"
  foot-gun already flagged under input-wiring residuals and State deferred extensions.
- **F. `indicator`/`series`/`step`/`sparse`, `[n]` history windows, warmup (prefeed vs
  replay), clocks** — all signal-host, on top of A–E.

**How this reshapes existing tracker rows:**
- #3 (`IndexExpr`/`[n]`) — the client is now explicit: the *historyable* capability (item
  A) + named `series` slots. Still diag-always until item A/F land; `LookupIndex` arrives
  with historyable.
- #7 (frame) — the per-tick frame is the per-event activation; unchanged, just reframed.
- #13 (engine/host API) — absorbs item D and the `examples/` signal-host packaging.
- #15/coercion/subtyping sections — the `CandleSeries <: Series` and multi-way-projection
  notes are all **host** concerns now; keep the mechanisms (Coerce/Implements) core, the
  concrete types host.

## Resolver/evaluator gap tracker (actionable work queue)

Unlike the rest of this doc, these are **not** deferred ideas — it's the prioritized
inventory of what the parser already accepts but resolver/evaluator don't handle yet
(re-snapshot: **2026-07-22**, reconciled against the actual code after the errors +
time modules landed; done items are removed, not struck through; numbering restarted —
old #1–#12 are absorbed below). Priorities:
P1 = silent correctness holes / crashes in what already "works", P2 = parser-accepted
features the pipeline drops or handles wrong, P3 = polish/consistency.

Already shipped and no longer tracked here: `if` (resolve + eval), binding kinds
(`Binding{T, Kind}`, `Assignable()`, NOT_ASSIGNABLE), input/output decl resolution
(inline-type synthesis, `Record`, env binding), `emit` end-to-end (statement-position
`emit(target, ...)` → `resolved.EmitStmt` → evaluator with the temporary `Emitted()` sink),
output-read check (outputs are emit-only: `let x = alert` → OUTPUT_NOT_READABLE),
panic recover at the `EvalRun`/`EvalInit` boundary (now the real internal-failure net,
not a stopgap — see below), duplicate-const recovery (`resolveConst`
continues past a duplicate instead of aborting the loop), **`state.*` end-to-end**
(2026-07-06, was #6: `state name: Type [= const-expr]` decls, `state.name` access,
STATE_UNDECLARED / STATE_UNINITIALIZED via the `definitelyAssignedState`
definite-assignment walker, TOP_DECL_IN_BODY, persistent `states` store on the
evaluator; design record in spec §3.2 item 4 / §4.3, deferred extensions below),
**real `EvalInit` + load-phase consts** (was #9, absorbed by state: consts → state
initializers → Init body all evaluate once in `EvalInit`; `EvalRun` no longer
re-evaluates consts per bar; host contract: call `EvalInit` once before `EvalRun`),
**runtime failure protocol** (was #10, shipped `066f969` "introduce runtime errors":
all four rule types return `(Value, error)`; `registry.Error{Kind, Message}` →
`diag.RuntimeFailure` (script trap: `Pos` + `EntryFn`) or `diag.InternalFailure`
(recovered panic, no position); uniform division-by-zero trap for `/` and `%` across
Integer/Float/Duration; `EvalRun` snapshots state via `maps.Clone` + clears emits on
any failure = tick rollback; spec §6.5 written to match), **`time` module end-to-end**
(`79f00bc`: `Time`/`Duration` int64-ms UTC values, `time.*` duration + weekday
constants, `from_unix_ms`, `truncate`, all operator rules in `stdlib/time_operators.go`;
spec §3.5 written; `DATETIME_DESIGN.md` folded in and deleted), **`&&` / `||`**
(was #8, 2026-07-22: short-circuit `resolved.LogicalExpr` node — NOT a registry rule,
since spec §3.6 promises short-circuit and the eager `InfixExpr` path evaluates both
operands; resolver special-cases `token.AND`/`token.OR` before `LookupBinary` and
requires Bool operands via the same TYPE_MISMATCH path as the `if`-cond check;
evaluator `evalLogical` skips the RHS when the LHS decides the result, asserts Bool
→ internal panic on violation), **runtime input binding** (was #7, 2026-07-25: the
frozen per-tick frame — `EvalRun(frame map[string]registry.Value)` → `bindInputs` validates
against `prog.Inputs`: unknown key → error, missing → error, type mismatch → error with the
int→float coerce edge tried first; binds fresh into the per-run env; threaded through
`Executable.Run(inputs)` + test helpers pass `nil`; `TestEvaluator_InputBinding` pins all six
paths. Design record + adversarial safety sweep in "Input wiring — design decision" below; the
frame is the per-event activation for the domain-blind core; still the throwaway-map stopgap
until the slot `[]Value` frame + host API #13), **bare `CallExpr` diagnostics**
(was #2, 2026-07-25: the `*ast.CallExpr` default branch in `resolveExpr` no longer returns a
silent `BadExpr`. Three sub-cases, all still `BadExpr` after reporting: `emit(...)` in
expression position → `EMIT_NOT_EXPRESSION` (statement-position emit is intercepted upstream in
`resolveStmt`, so reaching `resolveExpr` with an emit call *is* expression position);
ident callee → `UNDEFINED_FUNC` "unknown function foo" — reported even when the name *is* bound,
since nothing in the value namespace is callable and no `LookupFunc` table exists (don't build
one until bare functions are a real feature); any other callee (`(1 + 2)()`) → `NOT_CALLABLE`.
Args are deliberately not resolved — the call can never be valid, and the call diag is the
actionable one. Recorded decision: `emit` is matched by callee name, so `let emit = 3` does not
shadow it — keyword-by-convention).

| # | Prio | Area | Missing | Details |
|---|------|------|---------|---------|
| 3 | P1 | resolver | `IndexExpr` (`a[i]`) — no case in `resolveExpr`, falls to silent-`BadExpr` default | Verified: same eval-time death as #2. **Nothing is indexable today**: registry has no index-rule table (`VectorShape` declared but unused), so the fix is diag-always, not a feature. Plan: resolve `Left` and `Index` first (their errors surface), then `NOT_INDEXABLE` diag (`Integer is not indexable`) with the infix-style dedup guard (only add if no new errs) + `BadExpr`. Note (2026-07-05): state is NOT the future index client anymore — the locked state design (#6) has no `[n]` access; series history (spec §4.3 item 1) is the client that will eventually justify `LookupIndex(receiverType, indexType)` (lookup success builds the already-existing `resolved.IndexExpr`, failure keeps the same diag). Until then this stays diag-always; do NOT add an empty registry table before then |
| 5 | P1 | resolver | `resolveTypeDecl` silently swallows the `RegisterScriptType` error (resolver.go:319-323, comment now reads `// unreachable: duplicate decl names are caught by the env check first`) | Still open (2026-07-22). Reachable on registry reuse: a second `Compile()` against the same registry drops the input/output decl with **no diag**, then every use reports a misleading `UNDEFINED_IDENT` (verified: `emit(sig, …)` → "unknown identifier sig", zero hint at the cause). Even as a defensive branch it must append a diagnostic — silently dropping declarations violates the project's own "fail loudly" principle. Full fix (purge-on-reuse) stays in #13; the diag is a 3-liner now |
| 11 | P2 | resolver | No reserved-name / shadowing rules | All verified accepted-without-diag: `let math = 5` shadows the module inside a function (spec: RESERVED_REASSIGN); `let m = math` then `m.sqrt(9.0)` — modules are first-class, aliasable values (spec §5.4: passive syntactic prefixes, not values — a bare module ident in value position should diag); `const Init = 5` coexists with `function Init()` — function names never enter the top-level env (spec §3.3: one namespace). Decision needed alongside: same-scope `let x` redeclaration currently silently rebinds, even with a new type (JS errors on it; spec silent) |
| 12 | P2 | resolver | `emit` semantic checks: placement, payload shape, reserved kwargs | Verified gaps: (a) emit inside `Init()` resolves + evals clean — spec §5.2 wants EMIT_OUTSIDE_RUN (`resolveFunc` is identical for both fns; needs a context flag); (b) positional args fill *structured* outputs — `emit(sig, "down", 0.5)` — spec says kwargs-only, and `TestResolver_ResolveEmit` blesses the behavior: decide, then fix test or spec; (c) `emit(logs, value="hi")` — the synthetic `value` param name from `Registry.EmitRule` leaks into the surface language (kwargs on value outputs should be rejected); (d) reserved kwargs `ts`/`output` (spec: EMIT_RESERVED_KWARG) unenforced — cheap to reject at output-decl resolution before host wiring lands |
| 13 | P2 | both | Engine/host API rework — replaces BOTH temporary APIs: the `Emitted()` emission sink (TODO in evaluator.go) and the shipped `EvalRun(frame map[string]Value)` map stopgap (#7 — migrate to a slot `[]Value` frame) | Includes registry purge-on-reuse (see #5 for the immediate diag fix). Design the API around the now-runnable end-to-end fixture: struct input → compute → emit. See also "Rethink the public API surface" below — that section now also carries the resolver-contract and registry-encapsulation notes from the review |
| 14 | P2 | diag | Diagnostics quality pass: machine-readable interface, code reconciliation, message hygiene | `diag.Diagnostic` is just `Error() string` — spec §6.2/§6.4 promises tooling phase/code/pos without parsing messages; add `Phase()`/`Code()`/`Pos()` accessors while the type count is small. Fix misspelled external-contract codes **before** tooling matches on them: `TYPE_MISSMATCH`/`ARGS_NUMBER_MISSMATCH` (spec: `TYPE_MISMATCH`), `UNDEFINED_MEETHOD`. Stop leaking `Token.String()` debug format into messages ("unknown identifier [IDENTIFIER] -> sig") — use `.Literal`; affects ~9 diag types. Reconcile the code set with spec §6.4: UNEXPECTED_TOP_DECL vs TOP_LEVEL_FORM, DUPLICATE_DECLARATION vs PORT_DUPLICATE, INVALID_OPERATION shared by two diag types, EMPTY_FUNCTION absent from spec (also decide whether an empty `Run` is even illegal — spec never says so). Phases: spec defines two, impl has three (`PhaseCheck`), and `DuplicateDeclaration` ships with PhaseParse from the parser but PhaseCheck from the resolver (the NOTE at parser.go:100 already doubts it) |
| 15 | P3 | spec/both | Type-system divergence: the Int/Float split already shipped; spec §3.4's table still headlines a single `Number` (float64) | **Downgraded to P3 (2026-07-22): the split is now the accepted truth** — spec §3.5 explicitly names "the current `Integer`/`Float` runtime split" (line ~443) and time-operator registration is written around it, and §3.4 already documents Int/Float as a permitted future split. Remaining work is doc-consistency, not a fork: reword the §3.4 `Number` table row to acknowledge the shipped split up front (today only §3.5 does), keep user-facing names (`Float`/`Integer`) consistent, and note the residual sharp edges — `/` always float-returning (shipped), int arithmetic wraps silently (verified `9223372036854775807 + 1`), `-9223372036854775808` fails to parse (minus is an operator, bare literal overflows Atoi). No behavior change needed |
| 16 | P2 | parser | Forbidden function name → error cascade (parser.go:238-241) | `function Foo() {…}` returns without consuming the body; top-level recovery (`syncToNewLine`) lands inside it → verified: FORBIDDEN_FUNCTION + one UNEXPECTED_TOP_DECL per body line + one for the closing `}` (4 errors for a 2-line body). Parse the body normally (or skip balanced braces) and report once |
| 17 | P2 | parser | `else` on its own line fails with misleading "expected expression got else" (stmt.go:90) | The peek for ELSE doesn't cross NEWLINE; `} // comment` before `else` fails too (both verified). JS — the surface model — allows Allman-else. Either skip newlines before the ELSE check (unambiguous: `else` can't start a statement) or keep the restriction and emit a dedicated "else must follow } on the same line" diag |
| 18 | P3 | evaluator | `evalIf` silently treats a non-Bool condition as false (evaluator.go:175: `condBool, _ := condition.(registry.Bool)`) | **Downgraded to P3 (2026-07-22): the resolver now guards it** — `resolveIfStmt` rejects a non-Bool `if` condition (resolver.go:224, TYPE_MISMATCH), so the eval-time hole is only reachable via a `BadExpr` leak from #2/#3. Still worth the one-line `ok`-check → internal panic/error, since in a signal product a silently skipped branch (silently *not emitting*) is the worst failure mode; low urgency now that the normal path is covered |
| 19 | P3 | resolver | Assignment doesn't coerce (`let x = 1.5` then `x = 1` → TypeMissmatch) | Inconsistent with args/infix which coerce Int→Float |
| 20 | P3 | resolver | `resolveArgs` kwarg diagnostics: unknown kwarg silently skipped; positional+kwarg duplicate → actively wrong message | Verified: `math.pow(2.0, base=3.0)` reports "missing exponent arg" — the user passed `base` twice; the count check + silent kwarg-over-positional overwrite convert "duplicate arg" into "missing other arg". Fix: `resolvedIdx[i]` already true → "duplicate argument"; no rule match → "unknown keyword argument". Also: for emit, the count diag says "expected 2 args but got 1" counting only value args, though the user typed the target too — fix wording together with #21 |
| 21 | P3 | resolver | `CallArgExpr.Token` is the call token (existing TODO in `resolveArgs`) | Arg-level errors point at `(` instead of the arg |
| 22 | P3 | both | Bad-node invariant unchecked (the rolled-back `containsBad` walker) | Would have caught #2–#3 mechanically |
| 23 | P3 | resolver | No Unknown-type poisoning — one error cascades | `let x = <bad expr>` binds `x` as `UnknownTypeID`; every later use emits a fresh INVALID_OPERATION/TYPE_MISSMATCH (the `errsBefore` guard only dedups within one expression tree). Standard fix: lookups where an operand is Unknown succeed silently as Unknown |
| 24 | P3 | lexer | Raw newlines are legal inside string literals (`readString`) | Verified: `"line1<LF>line2"` lexes as one STRING. Consequence: an *unterminated* string swallows the file to the next quote — one confusing far-away error, recovery wrecked for everything between. Spec is silent on multi-line strings. Recommend: newline in string → `unterminated string` at the opening quote, terminate the token at line end (standard single-line recovery) |
| 25 | P3 | parser | Spec §7 resource limits: only nesting depth (64) exists | The parse-error cap (spec: 100) is a few lines and bounds worst-case memory — the parser currently collects unbounded diagnostics. String-length, identifier-length, source-size, kwarg-count caps can wait |

Suggested order (re-sequenced 2026-07-25 — #2/#6/#7/#8/#9/#10 now shipped):
**P1 hole-plugging batch: #3/#5 remain** (same disease as #2: silent `BadExpr` holes reachable
from valid parses, failing without diagnostics). Then #14 before any external tooling consumes
diagnostics. Then the CORE architecture-rework items A/B/C (top of doc) once the P1 batch is
stable. #13 (Engine/host API rework, absorbing item D) last, informed by the `Emitted()` +
`EvalRun(frame)` stopgaps being used in anger. The strategic milestone beyond the core is the
**signal-block host** (items D–F): indicators, warmup, clocks — built on top of A–E, in
`examples/` + tests only, never in the domain-blind core.

## Input wiring — design decision (2026-07-23 session)

How host data reaches a running strategy. Decided after weighing alternatives and a
cross-check against how other embeddable languages solve it (Starlark, Lua/LuaJIT,
Pine, CEL/Expr, wasm, NumPy/Arrow).

**Decision: the frozen per-tick FRAME.** The host hands the evaluator a snapshot of all
declared inputs, coherent as-of one timestamp, bound into the per-run env before each
`Run()`. This is the CEL/Expr "compile-once, eval-many with a per-call activation" model
and matches the spec's existing host-synchronizer framing (§4.1 "Run() cadence is a
runtime concern").

**Rejected alternatives:**
- **Live refs/pointers the script reads directly** (host registers `*Value`, a goroutine
  updates it). Fatally breaks *per-tick coherence*: a background write between two reads
  inside one `Run()` lets the strategy see `price@T` and `volume@T+1` — a worldview no
  market state ever produced, silently. Also needs atomics/locks and wrecks backtest
  determinism. The moment you add snapshot/epoch semantics to fix coherence, it collapses
  back into the frame. The *update-cadence-decoupling* this idea wanted is legitimate — but
  it belongs on the **host side**: the host keeps a live store (goroutines welcome), and the
  engine snapshots a frozen frame at the tick boundary. The script never reads a live pointer.
- **Actor mailbox / data-source iterator** — good fits for the future streaming runtime, but
  they *produce* a frame; they're #13 host-API shapes, adapters over the same frame contract,
  not a different execution model.

**The coherence boundary is the whole point:** who owns "a tick is one consistent snapshot."
Answer: the engine evaluates against a frozen frame; the host guarantees the frame's backing
data is immutable for the duration of a `Run`. Free in a synchronous backtest driver (update
and eval on one goroutine); a generation-stamp / double-buffer for a live concurrent updater.
**tascript promises nothing about host concurrency** — all sync primitives live host-side, and
that's fine; the host handles it easily.

**Memory — large inputs (e.g. "latest 5000 candles") are already cheap.** `registry.Value` is a
Go *interface* (two-word header), so a `CandleSeries`/window input rides in the frame as a
**handle**, not a copy — per-tick frame cost is O(#inputs), never O(window size). "Pass a ref"
is literally what an interface value is. This is *better* than wasm for our case: wasm copies
into linear memory across a heavy boundary; in-process Go shares the pointer directly. The only
thing faster is LuaJIT-FFI raw-pointer access, which trades away all safety — not worth it.
Optimization ladder (cheapest first): ring buffer (O(1) advance, no realloc) → reuse the handle
across ticks → columnar struct-of-arrays (`[]float64` per field, cache-friendly, likely what
talive wants) → opaque handle + column projections (`candles.close`; only touched columns
materialize; matches spec §3.4 CandleSeries + the "explicit named projections" note) → slot-array
frame (kills map hashing). The 5000-candle case ultimately wants to be an **engine-owned rolling
series the script indexes backward** (Pine's model; spec §4.2 history buffers) rather than a
per-tick input — the frame handle is the correct stopgap until then and costs nothing.

**Safety is a language-surface property, NOT a Go-memory property** (verified 2026-07-23). The
ring buffer is genuinely shared by reference, and it's safe anyway because *the script cannot
phrase a write to it*. tascript has exactly three assignment forms — `let x =`, `x =` (rebind a
`let`), and `state.field =` (state only). Verified rejections at resolve time: `price = 10` →
NOT_ASSIGNABLE (`Binding.Assignable()` is `Kind == KindLet`; inputs/const/output/module all
fail); `p.close = 10` → INVALID_ASSIGN_TARGET (member-assign requires the object be `state`).
Scalars are Go value types anyway (no aliasing even in principle); only compound handles are
shared, and they have no write syntax.

**Invariant to hold (the shared ref stays safe only while this holds):** host-registered types
expose **read-only methods only**, and the language never grows `obj.field =` / `arr[i] =` for
host-owned values (state mutation is fine — state is engine-owned). Three ways to accidentally
break it, guard against each: (1) general member assignment beyond `state.`; (2) index assignment
when `[n]` lands (#3 — make it read-only); (3) a host method registered with side effects (a
setter) writing through the shared pointer — the registry can't currently express "this rule
mutates" (same class as the impure-fn foot-gun already noted under State deferred extensions).

**Adversarial sweep (Codex, 2026-07-23) — no script-level mutation path today**, assuming
registered `EvalFn`s are genuinely read-only. `let x = candles` aliases but only name-rebinding
is available; `obj.field=`, `arr[i]=`, nested `state.a.b=` all fail to resolve to writes. Three
residual **escape/coherence** notes (not holes — no write surface — but real for #13 + state):
- **Handle escape via Run result** — `function Run() { candles }` returns the live input handle
  (`EvalRun` returns the block result as-is; `result` is already flagged temp/debug). If the host
  retains it past the tick it's no longer a snapshot.
- **Handle escape via emit** — `emit(out, candles)` with `output out: CandleSeries` emits the live
  handle; structured emits allocate a fresh outer `Record` but field values are still shared.
- **State is NOT scalar-only** (corrects an earlier claim) — only *inline-record* state is
  rejected; `state x: NamedType` accepts any registered type. Verified: `state m: math = math`
  compiles clean and persists a module handle across ticks. Once a `CandleSeries` seed/constructor
  exists, `state.saved = candles` would store a live host handle across ticks — breaking the
  "state holds copied scalars" assumption and making shallow `maps.Clone` rollback insufficient if
  such a handle's internals ever become mutable. Still no write primitive by itself.

## State — deferred extensions (from the 2026-07-05 design session)

Design locked in tracker #6 / spec §4.3. Explicitly parked:

- **Batch decl form** — `state: { cooldown: Integer, ... }` declaring several
  entries at once (entries without defaults, seeded in Init). New syntax, pure
  sugar over per-entry decls; add when a real script gets decl-heavy.
- **Record-typed entries** — `state pair: {a: Float, b: Float}`. Needs
  field-mutation semantics (`state.pair.a = 1`); v1 rejects at resolve time.
- **Purity flag on registry fns** — const/state-initializer contexts currently
  accept any module call; an impure host-registered fn makes a "const" value
  depend on the load moment. Accepted as the host's foot-gun; add a
  `Pure bool` on rules + context check only if it bites for real.
- **Rejected, do not revisit casually: lookback semantics** (Pine model,
  `state.x[1]`, ring buffer advanced per bar). User wants plain persistent
  variables; series history (§4.3 item 1) covers the lookback need.

## Style & cleanup backlog (small, no design needed)

From the 2026-07-05 review; each is a mechanical fix.

- `addNestingTooDeep` appends a **value** diagnostic; every other helper appends a
  pointer, and a test type-asserts the value form. Unify on pointers.
- `resolveArgs` parameter named `token` shadows the package (resolver.go:394) —
  rename (`callToken`).
- `Resolver.Resolve` ends in two identical branches (resolver.go:46-49) — a lost
  `return nil`? Decide the partial-tree-on-error contract and simplify.
- Two `Env` implementations drifting: resolver keys by `Symbol`, evaluator by
  `string`; resolver's `Get` special-cases `isTopLevel()` while the evaluator walks
  the parent chain plainly; `Symbol` is declared in resolver.go, not env.go. Align
  during the slot-based rework at the latest.
- `parseInputDecl` / `parseOutputDecl` are byte-identical copy-paste (parser.go:139-203),
  same "IDEN" typo comment three times including `parseConstDecl` — one parameterized
  helper.
- `token.Pos` / `token.Token` `String()` on pointer receivers — value receivers are
  idiomatic for tiny immutable structs, and `%s` on a `Pos` value misses the Stringer
  today.
- `resolved.LetStmt`/`AssignNameStmt` expose `Type()` though no interface requires it;
  `CallArgExpr.T` always equals `Value.Type()` after coercion wrapping. Promote
  deliberately or trim.
- Test debt: `extractErrorsPos` + `runDiagCases` duplicated between parser and resolver
  test packages and already forked (registry-setup param) — one shared home;
  parser_test.go:465 prints `len(prog.Consts)` in an inputs assertion; two subtests
  both named "args number missmatch" (the second is a type-mismatch case); a subtest
  named `"???"`; resolver's `runDiagCases` doc comment says "parser's diagnostics".
- Spec §3.1 wording: newline suppression is `(` / `[` only in the lexer; `{…}` schemas
  get parser-side newline skipping; `{` *blocks* keep newlines significant (they are
  statement separators). Fix the sentence.

## Test-coverage gaps worth closing

- `/` and `%` by zero now trap uniformly (DIVISION_BY_ZERO) across Integer/Float/Duration
  since #10 landed — pin with tests if not already covered. `math.sqrt(-1)` → NaN is still
  a silent-`NaN` gap; decide whether it should trap (INVALID_ARGUMENT) or stay NaN.
- emit-inside-Init (#12), shadowing cases (#11), Allman-else (#17),
  compile-twice-against-one-registry (#5).
- `Init` coverage exists now that state seeding runs there; a test pinning "Init writes
  persist into Run" is still the executable spec for the load-phase contract.
- No fuzz target on lexer/parser; the `^`-marker corpus makes seeds cheap.

## Slot-based variable access (perf)

**Status:** deferred. Starting with name → `TypeID` env and string-keyed runtime lookup.

The resolver currently maps names to `TypeID`. Later, assign each `let`/`const`/`input`
a **slot index** and emit `resolved.Ident{Slot, Type}` instead of a name. Evaluator reads
`slots[n]` (array index, ~1–2 ns) instead of a string-keyed map (~15–50 ns).

**Why it matters:** the `run()` body is re-walked **per bar** over thousands of candles.
Index access compounds; it's a real win, not premature. Also turns undefined-variable into
a resolve-time error and lets eval drop the locals map entirely.

Same pass (review addition): `evalMethodCall` builds a `map[string]Value` per call per
bar — the rule already knows param order, so the resolver can bake a param index and
the evaluator can pass positional `[]Value`, dropping the allocation and string hashing.

**When:** after the resolved-tree (bound-tree) migration is correct and stable.
Resolver env entry becomes `Binding{Type, Slot}`; runtime env becomes `[]Value`.

## Public coercion API (`Coerce` / `Implements`)

**Status:** deferred. `int → float` is hardcoded as a single private registry edge.

Keep the *shape* (a `LookupCoerce(from,to) → (rule, ok)` table) so going public is trivial —
do **not** inline `if from==Int && to==Float` in the resolver. Expose `reg.Coerce(...)` and
`reg.Implements(...)` only when a real custom-type need appears.

Design rules already agreed, to honor when it goes public:
- **At most one implicit edge per `(From, To)` pair** — reject a second at registration.
- **No transitive chaining** — single hop only (no auto `int→float→Price`).
- Per-edge `implicit` vs `explicit-only` flag (lossy conversions exist but require a cast).

## Subtyping + dynamic dispatch (`Implements`, e.g. CandleSeries <: Series)

**Status:** deferred. Only the numeric coercion exists today.

`Implements(Sub, Super)` = same value, no transform (vs `Coerce` = value changes). Per-function
granularity comes from *which type each param declares* — no union types.

**Bound-tree impact:** when an expression's static type is an abstract supertype and a
member/method is overridden per concrete type, the resolver **cannot** bake a single `EvalFn`.
Emit a `resolved.DynamicAccess{Recv, Method}` node that looks up by `value.TypeID()` at eval
time. Monomorphic sites stay fully baked; only polymorphic sites pay a runtime lookup.

## Multi-way conversions = explicit named projections

When a type has **more than one** way to become another (e.g. `CandleSeries → Series` via
close / open / hl2 / volume), it is **not** a coercion or subtype — it's a choice. Model as
explicit accessors: `candles.close`, `candles.hl2`. Implicit requires a *unique canonical*
transform.

**Open decision:** do candles get one blessed default (implicit `close` + explicit rest) or
zero default (always explicit, e.g. `sma(candles.close)`)? Leaning **zero default** — no
hidden price selection in a strategy.

## Expression-form defaults

**Status:** deferred. `ParamRule.Default` is an evaluated `Value` (constants only).

Fine for `period: 14`. To allow `offset: length/2` later, `Default` must become an `Expr`
(or a `resolved` node) evaluated at the call site. Crossing that line is the trigger.

Review addition — **the field is currently dead code and a trap**: declared, set `nil`
everywhere, never read, and `resolveArgs` enforces exact arg count, so the first registry
entry that actually sets a Default still hard-fails. Either wire default-folding into
`resolveArgs` or delete the field until this section triggers; at minimum comment it
"not folded yet".

## Lighter position info on resolved nodes (`Span`/`Pos` instead of full `token.Token`)

`resolved` nodes currently carry a full `token.Token` (Literal + Type + Pos). Once resolution
is done, `Literal`/`Type` are syntactic and dead weight — the evaluator only needs *where* to
point when a runtime error fires (div-by-zero, sqrt of negative, index out of range, bad/empty
input series). Replace with a lightweight span:

```go
type Span struct{ From, To token.Pos }   // or a single Pos if point-precision is enough
```

Expose it uniformly on the node interface (`Span() Span`) alongside `Type()`, so error
reporting can ask any node its position without knowing the concrete type. This is a
whole-`resolved`-file change, so do it in one pass (touches every node + wherever the resolver
constructs them). Note: only **value-domain** checks remain in eval — type checks are already
gone; don't let type re-checking creep back in. (Tracker #10 shipped and already consumes
positions — `RuntimeFailure` carries `tok.Pos` threaded through `evalInfix`/`evalPrefix`/
`evalMemberAccess`/`evalMethodCall` from each node's full `token.Token`. This section is now
a pure weight-reduction refactor: swap the full token for a `Span`/`Pos` without changing what
error reporting can point at.)

## Rethink the public API surface (`Engine` / `Executable` / `tascript.go`)

The top-level wiring feels ugly and needs a design pass. Smells so far:
- `Engine` re-exports registry methods one-by-one as passthroughs (`RegisterType`,
  `RegisterBinary`, …) — every new registry capability means another boilerplate forwarder.
  Consider exposing the registry directly, or a dedicated builder, instead of mirroring it.
- The `Engine` → `Compile()` → `Executable` split and lifecycle isn't clearly motivated
  (when do you register types vs compile vs run? what's reusable across runs?).
- Naming: `Engine`/`Executable`/`Compile` — settle on a coherent vocabulary for the phases
  (parse → resolve/compile → execute per-bar).
- Define the intended user flow end-to-end first (register custom types/funcs → compile a
  script → feed bars → read outputs), then shape these types around it.

Review additions:
- **Resolver contract is unchecked**: `Resolve()` nil-derefs on `prog.RunFn` / recovered
  nil idents if handed a program that failed parsing — "resolver only accepts Valid
  programs" is written nowhere and not asserted. Also resolves `RunFn` *before* `InitFn`,
  so diagnostics come out in non-source order. Define the contract here; IDE-style
  best-effort resolution over invalid programs will need nil-tolerance anyway.
- **Registry is half-encapsulated**: exported maps (`Binary`, `Types`, `Modules`, …)
  coexist with Register/Lookup methods, and resolver + evaluator reach into `reg.Modules`
  directly. All `Register*` except `RegisterType` return always-nil errors (existing TODO
  in registry.go) — callers ignore them today, so when dup checks land they'll be silently
  swallowed; make registration failures loud. `CoerceRule.EvalType` duplicates its key's
  `to` — unchecked redundancy.

## Reconcile naming between `ast` and `resolved`

The two packages mirror each other node-for-node, but small inconsistencies have crept in
during the migration. Do a pass to align them (or deliberately document each intentional
divergence). Known so far:

- `MemberAccessExpr.Method` / `KwargsExpr.Key`: `*ast.IdentExpr` in `ast`, bare `token.Token`
  in `resolved`. (The `resolved` choice is cleaner — a method/kwarg name isn't a variable — so
  consider fixing the AST side instead.)
- `ast.KwargsExpr` implements `Expression`; `resolved.KwargsExpr` is a plain helper. Review
  data point: `resolved.KwargsExpr` is **never constructed** (the resolver lowers kwargs to
  `CallArgExpr`) — deleting it resolves this bullet for free.
- Node-name suffix conventions (`*Expr` / `*Stmt` / `*Decl`) — verify they match 1:1 and that
  wrappers line up (`ast.Program.InitFn/RunFn` ↔ `resolved.Program.InitFn/RunFn`, etc.).
- Field name `Name` type differs across nodes (`token.Token` vs `string`) — audit for
  consistency now that `ConstDecl.Name` is a token.

## Dead code to remove

- `ast.Declaration` (and the four `declarationNode()` markers on `InputDecl`/`OutputDecl`/
  `ConstDecl`/`FunctionDecl`) — defined and satisfied but **never used as a type**; `Program`
  holds concrete slices. Drop the interface and markers. (Not mirrored in `resolved` for the
  same reason.)
- `resolved.KwargsExpr` — never constructed (see naming section above).
- `ast.FunctionDecl` commented-out `Parameters` block — delete; git remembers.
- `registry.ParamRule.Default` — see "Expression-form defaults" above.
- Deliberately kept, tied to tracker items (do not delete): `registry.VectorShape` (#3),
  `resolved.IndexExpr` (#3). (`diag.PhaseRuntime` is no longer dead — #10 shipped and both
  `RuntimeFailure`/`InternalFailure` now carry it.)

## Parser: trailing comma in call args (bug — double error)

`math.sqrt(number_foo=9.0, number=5, )` produces **2** errors:

```
parse [ARGS_ORDER] 1:49: args after kwargs not allowed
parse [UNEXPECTED_TOKEN] 1:50: expected ) got NEWLINE
```

Should be **0** — a trailing comma before `)` is probably fine, allow it. Even if we
decide trailing commas are illegal, it must be **one** error (the trailing comma itself),
not a cascade: the parser currently misreads the `,` `)` sequence as a positional arg
after kwargs and then trips again on the close paren.
