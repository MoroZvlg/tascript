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
- #3 (`IndexExpr`/`[n]`) — shipped as diag-always (2026-07-25); the client is now explicit: the
  *historyable* capability (item A) + named `series` slots. `LookupIndex` arrives with
  historyable and replaces the diag at that one site.
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
shadow it — keyword-by-convention), **`IndexExpr` diagnostics** (was #3, 2026-07-25:
`resolveExpr` resolves `Left` and `Index` first so their errors surface, then reports
`NOT_INDEXABLE` (`Integer is not indexable`, pointing at `[`) behind the infix-style dedup guard
and returns `BadExpr`. **Diag-always, not a feature** — nothing is indexable today and no index-rule
table exists; do NOT add an empty one. The future client is series history (spec §4.3 item 1) +
the *historyable* capability (rework item A): `LookupIndex(receiverType, indexType)` slots in where
the diag is today — success builds the already-existing `resolved.IndexExpr`, failure keeps this
diag. Keep it **read-only** when it lands — index assignment is one of the three ways to break the
shared-handle invariant, see the input-wiring section).

| # | Prio | Area | Missing | Details |
|---|------|------|---------|---------|
| 11 | P2 | resolver | No reserved-name / shadowing rules | All verified accepted-without-diag: `let math = 5` shadows the module inside a function (spec: RESERVED_REASSIGN); `let m = math` then `m.sqrt(9.0)` — modules are first-class, aliasable values (spec §5.4: passive syntactic prefixes, not values — a bare module ident in value position should diag); `const Init = 5` coexists with `function Init()` — function names never enter the top-level env (spec §3.3: one namespace). Decision needed alongside: same-scope `let x` redeclaration currently silently rebinds, even with a new type (JS errors on it; spec silent) |
| 12 | P2 | resolver | `emit` semantic checks: placement, payload shape, reserved kwargs | Verified gaps: (a) emit inside `Init()` resolves + evals clean — spec §5.2 wants EMIT_OUTSIDE_RUN (`resolveFunc` is identical for both fns; needs a context flag); (b) positional args fill *structured* outputs — `emit(sig, "down", 0.5)` — spec says kwargs-only, and `TestResolver_ResolveEmit` blesses the behavior: decide, then fix test or spec; (c) `emit(logs, value="hi")` — the synthetic `value` param name from `Registry.EmitRule` leaks into the surface language (kwargs on value outputs should be rejected); (d) reserved kwargs `ts`/`output` (spec: EMIT_RESERVED_KWARG) unenforced — cheap to reject at output-decl resolution before host wiring lands |
| 13 | P2 | both | Engine/host API rework — replaces BOTH temporary APIs: the `Emitted()` emission sink (TODO in evaluator.go) and the shipped `EvalRun(frame map[string]Value)` map stopgap (#7 — migrate to a slot `[]Value` frame) | Includes registry purge-on-reuse: `resolveTypeDecl` writes synthesized inline port types (`input.x`/`output.sig`) into the host-owned registry, but the duplicate guard reads the per-pass env — so a second `Compile()` on one Engine, or two scripts sharing one registry, collides. Scope the synthesized types per compile (or purge on reuse) rather than papering over the collision with a diagnostic. Design the API around the now-runnable end-to-end fixture: struct input → compute → emit. See also "Rethink the public API surface" below — that section now also carries the resolver-contract and registry-encapsulation notes from the review |
| 14 | P2 | diag | Diagnostics quality pass: machine-readable interface, code reconciliation, message hygiene | `diag.Diagnostic` is just `Error() string` — spec §6.2/§6.4 promises tooling phase/code/pos without parsing messages; add `Phase()`/`Code()`/`Pos()` accessors while the type count is small. Fix misspelled external-contract codes **before** tooling matches on them: `TYPE_MISSMATCH`/`ARGS_NUMBER_MISSMATCH` (spec: `TYPE_MISMATCH`), `UNDEFINED_MEETHOD`. Stop leaking `Token.String()` debug format into messages ("unknown identifier [IDENTIFIER] -> sig") — use `.Literal`; affects ~9 diag types. Reconcile the code set with spec §6.4: UNEXPECTED_TOP_DECL vs TOP_LEVEL_FORM, DUPLICATE_DECLARATION vs PORT_DUPLICATE, INVALID_OPERATION shared by two diag types, EMPTY_FUNCTION absent from spec (also decide whether an empty `Run` is even illegal — spec never says so). Phases: spec defines two, impl has three (`PhaseCheck`), and `DuplicateDeclaration` ships with PhaseParse from the parser but PhaseCheck from the resolver (the NOTE at parser.go:100 already doubts it) |
| 21 | P3 | resolver | `CallArgExpr.Token` is the call token (TODO at both `resolveArgs` callers) | Arg-level errors point at `(` instead of the arg. **Blocked on `Tok()` for `ast.Expression`** — same blocker as the `if`-condition TODO at resolver.go:225; fold into #14's position work |
| 22 | P3 | both | Bad-node invariant unchecked (the rolled-back `containsBad` walker) | Would have caught #2–#3 mechanically |
| 23 | P3 | resolver | No Unknown-type poisoning — one error cascades | `let x = <bad expr>` binds `x` as `UnknownTypeID`; every later use emits a fresh INVALID_OPERATION/TYPE_MISSMATCH (the `errsBefore` guard only dedups within one expression tree). Standard fix: lookups where an operand is Unknown succeed silently as Unknown |
| 26 | P2 | stdlib | `math.sqrt(-1)` returns NaN silently | Decision, then a one-line rule change: trap with `INVALID_ARGUMENT` (the kind already exists, spec §6.4) or keep NaN and say so in the spec. A NaN then flows through comparisons as `false` everywhere, which is exactly the silent-wrong-answer shape #15 was fixed to avoid |
| 27 | P3 | parser | Trailing comma allowed in call args, rejected in inline type schemas | Shipped 2026-07-25 for calls; `input x: {a: Integer,}` still errors, and `TestParser_Input`/`TestParser_Output` bless that. Fix mirrors the call-arg one in `parseInlineTypeExpr` (3 lines + 2 test expectations) — it was reverted only because those tests pin the current rule. Details in "Parser: trailing comma" below |
| 28 | P3 | resolved | Node metadata that is unused or always derivable | `LetStmt`/`AssignNameStmt` expose `Type()` though no interface requires it; `CallArgExpr.T` always equals `Value.Type()` after coercion wrapping. Promote deliberately (state the interface) or trim |
| 29 | P3 | both | `ast` ↔ `resolved` naming divergence | `MemberAccessExpr.Method` is `*ast.IdentExpr` in `ast` but a bare `token.Token` in `resolved` (the `resolved` shape is the better one); `Name` field types differ across nodes (`token.Token` vs `string`); `*Expr`/`*Stmt`/`*Decl` suffix conventions and wrapper pairs unverified. Full list in "Reconcile naming between `ast` and `resolved`" below |
| 30 | P3 | tests | Test debt with no design attached | Pin `/` and `%` by zero across Integer/Float/Duration (may already be covered — check, don't assume); pin "Init writes persist into Run", the executable spec for the load-phase contract; add a fuzz target for lexer/parser, seeded from the `^`-marker corpus. Full list in "Test-coverage gaps worth closing" below |

Suggested order (re-sequenced 2026-07-25 — #2/#3/#6/#7/#8/#9/#10 now shipped):
**No P1 rows remain**, and the D-batch smalls (#15/#18/#19/#24/#25) shipped 2026-07-25:
non-Bool `if` condition → `InternalFailure` instead of a silently-skipped branch; parse-error cap
of 100 (`maxParseErrors`, with `errCount` counting past the cap so the `errsBefore` guards that
gate AST nodes keep working); a newline inside a string literal ends the token at the line break
with `unterminated string` at the opening quote, leaving the NEWLINE intact so recovery survives;
`x = 1` on a `Float` binding coerces like args/infix/state-assign already did; spec §3.4 reworded
and every remaining `Number` in the spec replaced (it never resolved as a type name).
The E-batch parser smalls (#16/#17 + trailing comma) shipped 2026-07-25: a forbidden function
name no longer skips the body (recovery used to land inside it — 4 errors for a 2-line body,
now 1); `else` may start its own line — the lexer suppresses the NEWLINE run before an `else`
(`atElse`), the same way it already suppresses newlines inside `(`/`[`, so the parser keeps its
single token source and never has to look past a separator it may still need; a trailing comma
before `)` in a call closes the arg list instead of reporting 2 errors. #20 followed: an unknown
kwarg is now `ARG_UNKNOWN_KEYWORD` instead of being silently dropped, and a parameter filled
twice is `ARG_DUPLICATE` instead of surfacing as "missing <the other> arg"; both point at the
kwarg's own token, unlike the positional diagnostics still blocked on #21. Still open from that
row: the emit count diag says "expected 2 args but got 1" without counting the target — wording
belongs with #21.

**Rows #26–#30 (added 2026-07-25) are the smalls that were only ever prose bullets** in the
sections below — promoted so the table is the single work queue. Each is mechanical or one
decision wide; none blocks anything else, so they are fill-in work between the design items.

Next: #14 before any external tooling consumes
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

Cleared 2026-07-25: `addNestingTooDeep` now appends a pointer like every other helper (test
assertion updated); `resolveArgs`'s `token` parameter renamed to `argToken` (it is correct as written — the TODOs moved to the two callers, which are the ones passing a call token); `Resolver.Resolve`
collapsed to one `return` — **contract decided: it always returns the (possibly partial)
resolved program, and callers must check `Diagnostics()` before using it**; `parseInputDecl`/
`parseOutputDecl` now share `parsePortDecl` (they differed only in the node they build);
`token.Pos`/`token.Token` `String()` moved to value receivers; spec §3.1 reworded; the test debt
(the `len(prog.Consts)`-in-an-inputs-test assertion, the duplicated and the `"???"` subtest names,
the stale `runDiagCases` doc comment). **`extractErrorsPos`/`runDiagCases` stay duplicated in the
parser and resolver test packages — deliberate, do not re-file as debt.** A shared home was built
and rejected: it can only live in a normal package (Go cannot import a `_test` package), which
means either `internal/` or a public `diag/diagtest`; `diag` itself is out, since it is imported by
every package and a host, and `Assert` would drag `testing` + `go-cmp` into all of them. Revisit
only if a host embedding tascript needs to assert diagnostics in its own tests — then it becomes a
public `diag/diagtest` (the `httptest` pattern), not an internal one.

- Two `Env` implementations drifting: resolver keys by `Symbol`, evaluator by
  `string`; resolver's `Get` special-cases `isTopLevel()` while the evaluator walks
  the parent chain plainly; `Symbol` is declared in resolver.go, not env.go. Align
  during the slot-based rework at the latest.
- The unused-node-metadata bullet is now tracker row **#28**.

## Test-coverage gaps worth closing

Tracked as row **#30** (except the ones that belong to a feature row).

- `/` and `%` by zero now trap uniformly (DIVISION_BY_ZERO) across Integer/Float/Duration
  since #10 landed — pin with tests if not already covered. (`math.sqrt(-1)` → NaN is a
  language decision, not test debt: row **#26**.)
- emit-inside-Init (#12), shadowing cases (#11).
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

**Status:** deferred, and the `ParamRule.Default` field was deleted 2026-07-25 — it was
declared, never set, never read, and `resolveArgs` enforces an exact arg count, so the first
rule that set a default would have hard-failed anyway. Re-add it together with the
default-folding in `resolveArgs`, not before.

When it comes back: a `Value` (evaluated constant) is enough for `period: 14`. To allow
`offset: length/2`, `Default` must be an `Expr` (or a `resolved` node) evaluated at the call
site. Crossing that line is the trigger.

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

Tracked as row **#29**.

The two packages mirror each other node-for-node, but small inconsistencies have crept in
during the migration. Do a pass to align them (or deliberately document each intentional
divergence). Known so far:

- `MemberAccessExpr.Method`: `*ast.IdentExpr` in `ast`, bare `token.Token` in `resolved`.
  (The `resolved` choice is cleaner — a method name isn't a variable — so consider fixing the
  AST side instead.) The `KwargsExpr.Key` half of this bullet is gone: `resolved.KwargsExpr`
  was deleted 2026-07-25 (the resolver lowers kwargs straight to `CallArgExpr`), so kwargs now
  exist only in `ast`.
- Node-name suffix conventions (`*Expr` / `*Stmt` / `*Decl`) — verify they match 1:1 and that
  wrappers line up (`ast.Program.InitFn/RunFn` ↔ `resolved.Program.InitFn/RunFn`, etc.).
- Field name `Name` type differs across nodes (`token.Token` vs `string`) — audit for
  consistency now that `ConstDecl.Name` is a token.

## Dead code to remove

Cleared 2026-07-25: `ast.Declaration` + its five `declarationNode()` markers (the doc said
four — `StateFieldDecl` implemented it too), `resolved.KwargsExpr`, the `ast.FunctionDecl`
commented-out `Parameters` block, and `registry.ParamRule.Default`. All were unreferenced
outside their own definitions.

- Deliberately kept, still unconstructed after #3 shipped diag-always (do not delete): both
  `registry.VectorShape` and `resolved.IndexExpr` wait for the *historyable* capability
  (rework item A) — that's when `LookupIndex` starts building the node. (`diag.PhaseRuntime`
  is no longer dead — #10 shipped and both `RuntimeFailure`/`InternalFailure` now carry it.)

## Parser: trailing comma — call args allow it, inline type schemas don't

Tracked as row **#27**.

Fixed 2026-07-25 for call args: `math.sqrt(number=9.0, )` parsed as a positional arg after
kwargs and then tripped again on `)` (2 errors); a `,` directly before `)` now closes the
list, 0 errors.

**Left inconsistent on purpose, decide later:** `input x: {a: Integer,}` is still an error
(`expected IDENTIFIER got }`), and two tests bless it (`TestParser_Input` /
`TestParser_Output`, "trailing comma in custom type"). The one-line fix mirrors the call-arg
one in `parseInlineTypeExpr`; it was reverted only because those tests pin the current rule.
