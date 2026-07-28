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
  **While you are in `TypeDef` anyway, consider folding type registration and the reserved
  error-type guard into one mechanism** — e.g. a `Reserved bool` field: register `ErrorTypeID` like
  any other type so the existing occupancy check protects it, have `LookupType` skip reserved
  entries so scripts still cannot name it, and have rule registration reject reserved operands/
  params/`EvalType`. Today those are two separate mechanisms: builtins are protected *by being
  in* `Types`, the error type by deliberately staying *out* of it (see #23 — putting it in
  the map makes `input x: Error` resolve again). Explicitly **not** done with #23: it does not shrink
  the code (the same eight `rejectReserved` call sites remain, only the predicate changes), it
  turns a comparison into a registry lookup with an ordering dependency, and it only pays off
  once a second reserved type exists. If capabilities give `TypeDef` real structure — or if a
  host ever needs internal types it can register but scripts cannot name — revisit it here.
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
| 13 | P2 | both | Engine/host API rework — replaces BOTH temporary APIs: the `Emitted()` emission sink (TODO in evaluator.go) and the shipped `EvalRun(frame map[string]Value)` map stopgap (#7 — migrate to a slot `[]Value` frame) | Includes registry purge-on-reuse: `resolveTypeDecl` writes synthesized inline port types (`input.x`/`output.sig`) into the host-owned registry, but the duplicate guard reads the per-pass env — so a second `Compile()` on one Engine, or two scripts sharing one registry, collides. Scope the synthesized types per compile (or purge on reuse) rather than papering over the collision with a diagnostic. Design the API around the now-runnable end-to-end fixture: struct input → compute → emit. See also "Rethink the public API surface" below — that section now also carries the resolver-contract and registry-encapsulation notes from the review |
| 14 | P2 | diag | Diagnostics quality pass — **format, code set and position model shipped 2026-07-28; what is left is the §6.4 reconciliation** | **Rendered shape** is `error[CODE] line:col: message`, single-sourced through `diag.render`, pinned by `diag/diag_test.go` (the strings had no test at all before — the whole suite passed on struct comparison while the format changed under it). `expected X, found Y` phrasing; `Token.String()` debug leak replaced across 9 types. **Codes** are explicit constants, shortened and grouped by prefix (`ARG_*`, `TOP_DECL_*`, `STATE_*`); misspellings gone. Decision: **a code is a user-facing *category*, not a 1:1 class id** — binary and unary both report `INVALID_OPERATION` (nobody debugging a script cares which arity failed), as rustc's E0308 spans many situations. That rules out any test asserting code==typename. **Message hygiene, second pass (2026-07-28).** `PARSE_FAILED` became `NUMBER_OUT_OF_RANGE` and now carries the literal: its helper took an `error` and **discarded it** (`addParseFailed(pos, target, _ error)`), so an oversized literal reported "failed to parse INTEGER" while `strconv`'s actual "value out of range" was thrown away at the call site. Verified reachable and verified that range is the *only* failure mode — the lexer emits `INTEGER` from pure digit runs and `FLOAT` from digits with at most one dot, so `ErrSyntax` cannot occur. `INVALID_OPERATION` was rephrased from the non-sentence "Integer can't be + with String" / "String can't be -" to "cannot apply + to Integer and String" / "cannot apply - to String"; the shared category meant one phrasing decision covered both structs. **Position model unified.** The old `Pos`-vs-`Token` split was accidental, not principled: 13 diagnostics genuinely interpolated token text, 15 carried `Pos` only, and 8 carried a whole `token.Token` while only ever reading `.Pos`. Now **no diagnostic stores a `token.Token`** — each carries `At token.Pos` plus explicit semantic fields (`Name`, `Op`, `Field`, `Keyword`). `Phase` stopped being a field and became a per-type constant method, which is what unblocked the accessors: the earlier 'blocked on a name collision' finding was self-inflicted by storing constants as state. Interface is now `Error()/Code()/Phase()/Pos()`. `RuntimeFailure.EntryFn` dropped (it has a `Pos`, nothing read it, and the caller just called `EvalRun` or `EvalInit`); `InternalFailure` keeps it as its only locator, having no position at all — its `Pos()` returns the zero `token.Pos`. **Codex review of the finished change (2026-07-28) found five real defects, all fixed:** (i) `bindInputs` only scanned for undeclared frame keys when `len(frame) > len(inputs)`, so a *same-size* wrong key (`input price` + frame `{volume}`) reported `INPUT_MISSING` for `price` and silently swallowed the real mistake — the scan is now unconditional and unknown keys are reported first, since a typo'd key causes both symptoms; (ii) `InputUnknown` dropped the `%q` the old `fmt.Errorf` had, and that name is **host-supplied**, so an empty or newline-containing key could break the one-line format; (iii) `registry.UnknownKind` rendered as bare `UNKNOWN` in the code slot (reachable whenever a host rule returns a non-registry error), violating the error-word rule — now `UNKNOWN_FAILURE`; (iv) `SPEC.md` had been left stale **by this same session** — the §6.2 block still showed `(in Run)` and claimed the internal panic was the only diagnostic without a location, after `RuntimeFailure.EntryFn` was dropped and no-position `InputUnknown` was added; (v) catalogue subtests keyed on `Code()` collided, because codes are deliberately non-unique — now keyed on the Go type. Rejected codex's suggestion to assert `Phase()`/`Pos()` in the catalogue: those were deliberately deleted as tests of the Go compiler. **Left open by decision:** `DuplicateDeclaration.Phase()` returns `PhaseCheck` although the parser also produces it — deemed not worth splitting the type over. **Settled 2026-07-28:** port wiring is a **runtime** concern, not a launch-time one. The spec's `INPUT_NOT_WIRED`/launch-phase framing is gone; the phase vocabulary is now exactly `parse`/`check`/`runtime` with no fourth stage, and §6.4 carries `INPUT_MISSING`/`INPUT_UNKNOWN`/`INPUT_TYPE_MISMATCH`. **Reviewed with codex, which talked me out of the bigger version:** privatizing every field + constructors would have broken `errors.As` at 4 sites (tests use *value* targets, `var f diag.RuntimeFailure`), and an unexported embedded `base` cannot be reached by `cmp.AllowUnexported` from an external test package, forcing `diag` to export a cmp-options helper. Keeping fields exported left both `cmp.Diff` sites untouched. Rejected codex's `Pos() (token.Pos, bool)` — 36 types paying a two-value return for one exception; `go/token`'s `NoPos` sentinel is the precedent. Its warning that a decoded string `Literal` is not the same width as its source text is worth honouring when `Span` lands. **Decision made without you: `DuplicateDeclaration` is now `PhaseCheck`** — the parser previously reported it as `PhaseParse` and the resolver as `PhaseCheck`; a per-type constant forced one, and a duplicate declaration is semantic regardless of which pass noticed. Phase is no longer compared by tests (the field is gone), so nothing pins this. **Left:** (a) §6.4 reconciliation is still wide — spec has ~11 codes with no impl (`BOOL_REQUIRED`, `EMIT_PAYLOAD`, `RESERVED_REASSIGN`, `OUTPUT_NOT_WIRED`, `HISTORY_*`, most of the `*_LIMIT` family — `EMIT_OUTSIDE_RUN` shipped with #12, and `EMIT_RESERVED_KWARG`/`INPUT_NOT_WIRED` were deleted from the spec rather than implemented) and impl has ~24 with no spec entry, plus naming conflicts (`PORT_DUPLICATE` vs `DUPLICATE_DECLARATION`, `UNKNOWN_OUTPUT` vs `INVALID_EMIT_TARGET`, `TOP_LEVEL_FORM` vs `TOP_DECL_UNEXPECTED`, `INTERNAL` vs `INTERNAL_FAILURE`); (b) whether `BOOL_REQUIRED` splits out of `TYPE_MISMATCH` for `if`/`&&`/`||`, which the spec assumes and the impl does not do; (c) whether `UNDEFINED_IDENT`/`UNDEFINED_VAR` and `UNDEFINED_ATTRIBUTE`/`UNDEFINED_METHOD` merge under the same 'nobody cares which' rule; (d) `EMPTY_FUNCTION` — decide whether an empty `Run` is illegal at all, spec never says; (e) **severity does not exist in `diag` at all** — the `Severity` type and its constants were removed 2026-07-28 rather than left as unused vocabulary; `render` writes the literal `error`. Reintroducing it is the *warnings* work, and the blocker is not the field: **counting diagnostics is load-bearing in three places that all assume diagnostic == error.** (i) ~9 `errsBefore := len(r.errs)` dedup guards in the resolver (and `p.errCount` in the parser) mean "did this subtree already report something" — a warning in the same slice makes the guard suppress a real diagnostic; (ii) `resolver.go:449`, the #23 `ErrorGuaranteed` assertion `if len(r.errs) == 0 { panic }`, would be **falsely satisfied** by a warning, silently minting error types with no error behind them — this one does not crash, it un-does the guarantee; (iii) `maxParseErrors` caps `len(p.errors)`, so warnings would eat the error budget. So warnings need their **own slice**, not a severity field filtered at the end, and `Compile()` must start returning an executable *and* diagnostics (today `len(diags) > 0` **is** the failure predicate, in `NewEngine`, `Compile`, `prog.Valid`, and the fuzz assertion) — same seam as #13. First real warning is probably `EMPTY_FUNCTION` (an error today; the spec never says an empty `Run` is illegal) or a #11 shadowing rule. `render` must also take severity from the diagnostic when this lands, or warnings print as `error[...]`; (f) **input-binding failures became diagnostics 2026-07-28** — `bindInputs` used to return bare `fmt.Errorf` for unknown/missing/mistyped ports, so a host wiring the wrong type into a port got an unstructured Go error with no code and no position while every other failure was a `Diagnostic`. Now `INPUT_UNKNOWN` / `INPUT_MISSING` / `INPUT_TYPE_MISMATCH`, all `PhaseRuntime`, pointing at the `input` declaration (`InputUnknown` has no position: the offending key is not in the script at all, so like `InternalFailure` it renders without one). Note this is **the first genuine two-phase category** the impl has — a port type error is the runtime half of what spec §6.4 calls `TYPE_MISMATCH | parse / runtime`; it was given its own code rather than reusing `TYPE_MISMATCH` so `Phase()` could stay a per-type constant. If more categories turn out to span phases, `Phase()` goes back to being data. Also 2026-07-28: the two `default:` branches in `evalStmt`/`evalExpr` now panic instead of returning `fmt.Errorf`, matching the #22 decision in the resolver — an unhandled node means the resolver produced something the evaluator never wired up, which is an interpreter bug, and the recover boundary turns it into `INTERNAL_FAILURE`; (g) inherited from #21, the emit arity diag counts rule params against `call.Args[1:]`, so `emit(sig, "up")` says "expected 2 args, found 1" — right arithmetic, misleading against what was typed |

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
kwarg's own token, unlike the positional diagnostics that were still blocked on #21 at the time.

**#21 closed 2026-07-28, and the "blocked" premise in its own row was wrong.** The row said it
needed `Tok()` on `ast.Expression` and should fold into #14's position work; neither held. Every
`ast` node already carried a `Token`, so the interface method was mechanical, and the three
consumer sites only wanted *a better token to hand an existing diag constructor* — they never
touched how `diag.Diagnostic` carries position, which is the actual #14 question. Doing it first
keeps #14 to interface + code set + message hygiene. Shipped: `Tok() token.Token` on the `Node`
interface (so statements and type decls get it too, not just expressions); `ast.BadStmt` is the
only node without a `Token` field and synthesizes from `From`, which is what `resolveStmt` used
to open-code. Positional args and `CallArgExpr.Token` now use `arg.Tok()`, kwargs use the key
token, and the `if` condition points at the condition instead of the `if` keyword — that last one
was a separate TODO not even listed under the row. Arity and missing-arg diags deliberately keep
the call token: there is no arg to point at. The ~124 concrete-type `.Token` reads in the
resolver were left alone — `Tok()` earns its place only where the static type is an interface;
converting the rest trades a compile-time-checked field for a method returning the same value.

**#12 closed 2026-07-28 — `emit` semantic checks.** Shipped: `EMIT_OUTSIDE_RUN`, using the same
`currFn string` the evaluator already carries (set to `Init`/`Run` around each `resolveFunc`)
rather than a bespoke bool, so a third entry point later is a string and not a second flag; it
reports and keeps resolving, so payload diagnostics still surface.

**Decided: both argument forms stay legal, and the spec bent to the impl.** Every other call in
the language accepts positional *and* kwargs, so `emit` being kwargs-only would have been the
special case, not the consistency — `emit(sig, "down", 0.5)` and `emit(sig, dir="down",
price=0.5)` are the same call, and `TestResolver_ResolveEmit` still blesses both. The field-order
hazard is real (reordering a structured output's `{ … }` fields silently changes every positional
emit whose types still line up) and is a §5.2 callout rather than a diagnostic. This also
promoted `EmitRule`'s synthetic `value` param from accidental leak to **documented surface**:
`emit(logs, "hi")` and `emit(logs, value="hi")` are equivalent.

**`EMIT_RESERVED_KWARG` was built and then removed the same day, along with the whole
reserved-name concept.** Measured against unmodified HEAD, the reservation bought almost nothing:
`output` is a **keyword token**, so `emit(a, output=1)` already died at parse with
`EXPRESSION_EXPECTED` and `output sig: {output: String}` with `UNEXPECTED_TOKEN: expected
IDENTIFIER, found output`; `emit(a, ts=1)` already gave `ARG_UNKNOWN` + `ARG_MISSING`. The single
real gap was `output sig: {ts: String}` being accepted — not worth a code, a diag type and a
two-site check, **because the thing being protected does not exist**: nothing injects
`ts`/`output` today. §5.2's reserved-names paragraph and the §6.4 row are gone. **Revisit when
#13 defines the event envelope**, and prefer nesting it (`event.ts`) over reserving bare names,
which avoids the collision entirely.

**Resolver precondition, decided explicitly.** `parser.go:114` only assigns `prog.RunFn` when the
decl parsed clean, so any parse error inside `Run` (`function Run() {}` → `EMPTY_FUNCTION`,
`function Run() {`, `let a = 1 &&`) leaves it nil and `Resolve()` nil-derefs at `ast.go:181`. A
nil-guard was added and then **rejected: `Resolve` is never called on an invalid program in
reality** — `Compile()` returns on parse diagnostics first (`tascript.go:23`), and #13's API makes
that structural. The contract is *`Resolve` requires a clean parse*, unguarded. Consequence:
resolver diag fixtures must parse cleanly, so the `emit inside Init` case needs a non-empty `Run`
body. A `FuzzResolve` (parse → resolve, skipping inputs with parse diagnostics) was written to
confirm reachability, then **dropped** — of its own 597-entry corpus only **8 entries (1.3%)**
parsed cleanly enough to reach the resolver. Go's fuzzer sees coverage from the whole fuzz
function and the parse runs inside it, so the mutation loop is steered by lexer/parser edges and
drifts away from valid programs; it was a slower `FuzzParse`. Fuzzing the resolver properly needs
a **grammar-based generator of valid programs** — revisit after #13 settles the API. The
resolver's real protection stays the `^`-marker diag tables, which assert exact diagnostics a
fuzzer cannot. Note `FuzzParse` stopping at parse is why nothing caught the nil-`RunFn` deref
despite `function Run() {\n}` sitting in its seed corpus.

**Spec gap noted, not closed:** the general call-argument rules are specified *nowhere* — they
exist only implicitly in the §6.4 error rows — so §5.2 states them inline. Worth a real §5.x when
someone next touches call syntax.

**Rows #26–#30 (added 2026-07-25) are the smalls that were only ever prose bullets** in the
sections below — promoted so the table is the single work queue. Each is mechanical or one
decision wide; none blocks anything else, so they are fill-in work between the design items.
#26/#27 shipped the same day: a trailing comma now closes an inline type schema too, and
`math.sqrt(x < 0)` / `math.log(x <= 0)` trap with `INVALID_ARGUMENT` instead of handing back
a NaN that every later comparison reads as `false`. `math.pow` can still produce a NaN
(`pow(-1.0, 0.5)`); it needs a two-argument domain rule, so it was left for whoever wants it.
#30 followed on 2026-07-26 (Duration div-by-zero rows, `TestEvaluator_LoadPhase`, and the
lexer/parser fuzz targets — details in "Test-coverage gaps worth closing" below), and #28
followed the same day (six unused `Type()` methods + `CallArgExpr.T` deleted; the "a Statement
has no type" decision is recorded under "Style & cleanup backlog"), and #29 closed with it
(see "Reconcile naming between `ast` and `resolved`" — the `ast`/`resolved` type difference
turned out to be the layer boundary and was deliberately kept).

**#22 closed 2026-07-26 without building the walker.** The audit that preceded it: 26 of the 28
`BadExpr`/`BadStmt` construction sites in `resolveStmt`/`resolveExpr` pair the bad node with a
diagnostic — five of them report conditionally behind `if len(r.errs) == errsBefore`, which only
skips when an inner expression already reported, so `len(Diagnostics()) == 0 ⟹ no Bad node`
holds there. The invariant had exactly **two** holes, both `default:` branches returning a bad
node with zero diagnostics under an "unreachable" comment (resolver.go:182 and :591). Reaching
either meant the host saw an empty `Diagnostics()`, ran the program, and got
`unsupported statement %T` out of the *evaluator* instead. Both now `panic` with the offending
`%T`, matching the evaluator's existing unreachable-branch convention. That covers the same bug
class — a new `ast` node the resolver never wired up — for two lines instead of a ~70-line
walker over 23 node types that would itself need a case per new node. The walker only adds value
if the resolver ever grows a path that builds a bad node for a reason *other* than a reported
error; revisit then, not before. Note these panics are not recovered — unlike `EvalRun`/`EvalInit`,
`Resolve()` has no recover boundary, which is consistent with it already nil-dereffing on an
invalid program (see "Rethink the public API surface"). If `FuzzParse` is ever extended to resolve
the programs it accepts, these become fuzz-visible.

**#23 (error-type absorption, aka poisoning) shipped 2026-07-26**, design-reviewed against Codex first. Before:
one undefined identifier produced four diagnostics (`let x = nope` → `INVALID_OPERATION` on `x + 1`,
again on `y * 2`, then `TYPE_MISSMATCH` on `math.sqrt(z)`), three of them naming `Unknown`, a type
that does not exist in the language. Now every one of those collapses to the single real error.
What landed:

- **`ErrorTypeID` is reserved** (`rejectReserved`, registry.go). It was never in `DefaultRegistry`
  and `RegisterType` only checked map occupancy, so a host could do `RegisterType("Error")` and
  get a legitimately-typed `input x: Error` binding with zero diagnostics. Now rejected from
  `RegisterType`/`RegisterModule`/`RegisterScriptType` (name and field types)/`RegisterCoercion`/
  `RegisterBinary`/`RegisterUnary`/`RegisterMemberAccess`/`RegisterCall` (owner, `EvalType`, params).
  This is what makes the assertion below sound: the error type is now unforgeable, so every
  occurrence traces to a resolver-reported error.
- **The predicate is type-based, not structural.** `isErrorType(t) == (t == ErrorTypeID)`, never
  `IsBadExpr`. In `x + 1` the `x` resolves to an ordinary `IdentExpr` carrying `T: Error` — it is
  *not* a `BadExpr`, so a structural check misses the entire cascade. The pre-existing
  `!resolved.IsBadExpr(...)` guards in `resolveIfStmt` and `resolveLogical` were exactly this bug
  and are now type-based.
- **Absorbing sites:** binary, unary, member access, index, method call, if-condition, logical
  operands, both `resolveArgs` type checks, and the name-assign / state-assign / state-init coerce
  checks.
- **The error type does NOT propagate out of a call.** An error-typed argument still yields the rule's declared
  `EvalType`, so `math.sqrt(<errored>)` stays a `Float` and downstream stays quiet. Codex argued the
  call should be error-typed too; that manufactures cascade rather than stopping it, and assuming the
  declared result type is what Go's own checker does. Wrong arg *count* still invalidates — that is
  not a type question.
- **`ErrorGuaranteed`-style assertion.** `Resolver.newErrorExpr()` panics if it fires with
  `len(r.errs) == 0`. Weaker than rustc's provenance token (an unrelated earlier error can still mask
  a fresh resolver bug), but full threading is not worth it at this size, and reserving the type
  closed the hole that actually mattered.
- **Two cascade sources fixed beyond the row's text**, both found by Codex: a failed state
  *initializer* used to `continue` before appending the field, turning one bad initializer into a
  later `STATE_UNDECLARED`; and a failed `state.x = ...` used to become a `BadStmt`, which
  `definitelyAssignedState` does not count, adding `STATE_UNINITIALIZED` on top. Both now keep the
  field/statement so the follow-on never fires. **Decision: an error-typed RHS counts as an
  initialization attempt** — the program cannot run anyway, so the follow-on is pure noise.

**Follow-up shipped the same day: failed port/state types now bind anyway.** A bad type on an
`input`/`output`/`state` decl used to `continue` without creating the binding, so one
`UNDEFINED_TYPE` was followed by `UNDEFINED_IDENT` / `STATE_UNDECLARED` at every use — the
misleading kind of cascade, since the message tells you to declare a name you *did* declare.
All three now bind with `ErrorTypeID` and report once. Two consequences worth knowing:

- **Comparisons absorb when EITHER side is the error type**, not just the value side.
  `isErrorType` is variadic and every actual-vs-expected check passes both
  (`resolveArgs`, state init, state assign, name assign). Binding with `ErrorTypeID` alone would
  have moved the cascade rather than removed it — `emit(sig, 1)` against an error-typed output
  would have reported `expected Error type got Integer`.
- **`EmitRule`'s discarded `LookupTypeDef` ok is now genuinely reachable** (it was dead before,
  since `resolveOutputs` skipped the binding). An error-typed output falls through to the scalar
  one-`value`-param shape, and `resolveArgs` absorbs it. That fall-through is now load-bearing,
  not an oversight — pinned by `TestRegistry_EmitRule`.

The motivating question — whether two diagnostics are *better* here — was considered and rejected:
the follow-on names a symbol the user did declare, so it misdirects. The stronger reason to bind is
independent of diagnostic count: a skipped binding leaves a hole in the symbol table, and #14's
tooling goals (go-to-definition, completion, hover) need the symbol to exist even when its type
does not. Go, Rust and TypeScript all bind-and-report-once here.

Residual, deliberately not fixed: a *genuine* type mismatch (not an error type) in a state initializer still
`continue`s and drops the field, and in a state assignment still returns `BadStmt` — so those two
cascades remain. Appending the field anyway would put a value in the tree whose type contradicts the
field's declared `T`, which the error type does not (it is transparently "an error happened here"). Fix it only
alongside a decision about whether `resolved` may hold deliberately ill-typed recovery nodes.

Pinned by 7 cases folded into the existing `TestResolver_ResolveRunErrors` (5) and
`TestResolver_ResolveStateErrors` (2) tables — deliberately not a separate test func, since that
file is organized by construct, not by feature — plus `TestRegistry_ErrorTypeIsReserved` (13).
When items A/B land, short-circuit the error type *before* capability lookups so `typeReplaceable(ErrorTypeID)`
never reports while `slotWritable(kind)` still can.

**No smalls remain: the table is now #11, #13, #14 — all design work.** (#12 and #14's
implementable half both shipped 2026-07-28; #14's remainder is blocked on #11/#12/#13
because its unwritten codes name behaviours those rows have not designed yet.)

Next: #11 (reserved-name / shadowing rules), then the CORE architecture-rework items A/B/C
(top of doc). #13 (Engine/host API rework, absorbing item D) last, informed by the
`Emitted()` + `EvalRun(frame)` stopgaps being used in anger — it also owns the emit event
envelope that #12 declined to reserve names for. The strategic milestone beyond the core is the
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
- Unused node metadata (was row **#28**) — cleared 2026-07-26. Six `Type()` methods deleted
  from `resolved`: `LetStmt`, `AssignNameStmt`, `AssignStateStmt`, `InputDecl`, `OutputDecl`,
  `ConstDecl`. **Decision: a `Statement` has no type.** `Expression.Type()` is a contract the
  resolver typechecks against; the `T` on a statement answers a different question each time
  (`LetStmt.T` = the type of the binding it creates, `AssignStateStmt.T` = the type of the
  target field, `EmitStmt.T` = the type of the output port), so exposing them all through the
  method name that means "the static type of the value this evaluates to" invites treating
  statements and expressions uniformly — which is exactly what isn't true. The `T` **fields
  stay** (`EmitStmt.T` is load-bearing: the evaluator builds the emitted `Record` from it);
  only the methods went, and the two resolver test dumpers now read `.T` directly. There is no
  `resolved.Declaration` interface, so the three `*Decl` methods had no callers at all.
  `CallArgExpr.T` was deleted outright — written twice in `resolveArgs`, never read, and equal
  to `Value.Type()` once coercion wrapping is applied. Revisit only if tascript ever goes
  expression-oriented (`let x = if c { 1 } else { 2 }`), which would make `IfStmt` an `IfExpr`
  and give blocks real types.

## Test-coverage gaps worth closing

Row **#30 closed 2026-07-26.** What shipped:

- `/` by zero on Duration — the three `SLASH` rules in `time_operators.go` (`/Integer`,
  `/Float`, `/Duration`) joined `TestStdlib_Traps_Pipeline` as `DIVISION_BY_ZERO` rows;
  Integer and Float were already pinned by `TestEvaluator_TrapDivisionByZero`.
- `TestEvaluator_LoadPhase` is the executable spec for the load-phase contract: Init state
  writes persist into Run and across ticks, state initializers evaluate at load, and — via a
  registered counter member that ticks on every evaluation — consts and the `Init` body each
  evaluate exactly once no matter how many `EvalRun`s follow.
- `FuzzLex`/`FuzzParse` in `parser/fuzz_test.go`, seeded with one sample per top-level form
  plus the error shapes that needed recovery work. `FuzzLex` asserts EOF arrives within
  `len(src)+2` tokens (every token consumes at least one byte, so a scanner that stops
  consuming shows up as a hang, not a timeout). `FuzzParse` asserts the parser contract
  `prog.Valid == (len(Diagnostics()) == 0)`, that a valid program always has a `RunFn`, and
  that the diagnostic slice respects `maxParseErrors`. ~16.5M execs clean at time of writing.

Still open, but owned by feature rows rather than by test debt: emit-inside-Init (#12),
shadowing cases (#11).

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
  directly. `CoerceRule.EvalType` duplicates its key's `to` — unchecked redundancy.
- **Registration errors are real now, but `stdlib` still discards them.** The always-nil returns
  are gone (2026-07-26): every `Register*` rejects a duplicate and the reserved error type, and
  `TestRegistry_DuplicateRegistrationRejected` covers all eight entry points. But ~37 call sites
  ignore the returned error, all of them inside `stdlib`/`registry`. **Decision: leave them, do
  not add a `must()` helper.** stdlib registers into a fresh registry before any host type, so its
  only possible failure is a duplicate we authored; and since duplicates are now *rejected* rather
  than overwritten, the first registration wins — an internal duplicate would silently drop a rule
  and fail the pipeline tests. Verified clean at the time: the checks were temporarily converted to
  panics and the whole suite run, since every test builds a registry. **Revisit if that ordering
  guarantee breaks** — if hosts can register before stdlib, or stdlib registration becomes
  conditional, these must be checked. That ordering is exactly what #13's API design should pin
  down, which is why the note lives here rather than in `stdlib.go`.

## Reconcile naming between `ast` and `resolved`

Row **#29 closed 2026-07-26.** The audit found the packages were *more* consistent than the
row claimed, and the row's own premise was wrong on two counts.

**The `ast` ↔ `resolved` type difference is the layer boundary, not drift — do not "fix" it.**
Counted across every name-holding field: `ast` is `*IdentExpr` 10/10, `resolved` is `string`
10/11. Syntax nodes carry a position because resolve-time diagnostics need it; resolved nodes
are plain data because positions are spent by then. The row proposed flattening
`ast.MemberAccessExpr.Method` to a bare token to "match" `resolved` — that would have made it
the only outlier in `ast`. Dropped. (The row also mis-stated the `resolved` side as
`token.Token`; it was already `string`.)

What actually shipped:

- **`resolved.ConstDecl.Name`: `*IdentExpr` → `string`** — the lone outlier in a package of
  `string`s. `String()` lost a dead `== nil` guard (the resolver already dereferences
  `c.Identifier` unguarded upstream). Note the const *identifier's* position is no longer on
  the resolved node — only `ConstDecl.Token` (the `const` keyword). Nothing needs it: duplicate
  -const diags fire at resolve time off the ast node, and runtime failures inside a const's
  value get their position from the value expression. Add a `NameToken` if that ever changes.
- **`Method` renamed where it holds no method.** It named three different things.
  `resolved.MemberAccessExpr.Method` → `Attribute` (`math.PI` invokes nothing — and the
  resolver already reported `addUndefinedAttribute` three lines below the lookup);
  `resolved.StateAccessExpr.Method` → `Field` (`state.cooldown` is a state field);
  `MethodCallExpr.Method` kept, it genuinely holds a method name. On the `ast` side the same
  field became **`Member`**, not `Attribute` — the parser cannot yet know whether the name
  after the dot is an attribute, a method, or a state field; the resolver decides.
- **`resolved.Function` → `FunctionDecl`** — `InputDecl`/`OutputDecl`/`ConstDecl` all keep the
  suffix and the `ast` counterpart is `FunctionDecl`.

Deliberately left divergent: `ast.CallExpr` ↔ `resolved.MethodCallExpr` is a real **narrowing**
(the parser accepts any `Callee Expression`; only a member-access callee resolves, everything
else diags), so the more specific name is correct. `resolved.State`/`StateField` are not a
mirror of `ast.StateFieldDecl` — `State` is a container of `[]*StateField` — so renaming them
`*Decl` would fake a 1:1 correspondence that does not exist. `resolved.KwargsExpr` is gone
entirely (deleted 2026-07-25; the resolver lowers kwargs straight to `CallArgExpr`), so kwargs
exist only in `ast` by design.

Adjacent wart, not fixed, no row yet: `ast.IdentExpr` implements `expressionNode()`, so all 10
of those fields claim to hold an *expression* — but a decl's identifier, a kwarg key, and the
name after a dot are none of them evaluable. If it's worth fixing it's a dedicated
`ast.Name{Token}` applied to all 10 sites, not a one-off; nothing traverses those fields as
expressions today.

## Dead code to remove

Cleared 2026-07-25: `ast.Declaration` + its five `declarationNode()` markers (the doc said
four — `StateFieldDecl` implemented it too), `resolved.KwargsExpr`, the `ast.FunctionDecl`
commented-out `Parameters` block, and `registry.ParamRule.Default`. All were unreferenced
outside their own definitions.

Cleared 2026-07-26: `resolved.IsBadExpr`, orphaned by #23 — its four callers all became
`isErrorType`. Deleting it is deliberate, not just tidying: `IsBadExpr ⊂ isErrorType` (a `BadExpr`
always types as `ErrorTypeID`, but a propagated `IdentExpr{T: ErrorTypeID}` carries the error type
without being one), so leaving the narrower predicate in reach invites exactly the bug #23 fixed.
`ast.IsBadExpr` is a different function, still used three times in `parser/expr.go` — keep it.

- Deliberately kept, still unconstructed after #3 shipped diag-always (do not delete): both
  `registry.VectorShape` and `resolved.IndexExpr` wait for the *historyable* capability
  (rework item A) — that's when `LookupIndex` starts building the node. (`diag.PhaseRuntime`
  is no longer dead — #10 shipped and both `RuntimeFailure`/`InternalFailure` now carry it.)

## Parser: trailing comma — shipped

Fixed 2026-07-25, both sites: `math.sqrt(number=9.0, )` parsed as a positional arg after
kwargs and then tripped again on `)` (2 errors); `input x: {a: Integer,}` reopened the field
loop and demanded another field. A `,` directly before the closer now ends the list in both,
0 errors. The schema form also skips a newline run first, so `{\na: Integer,\n}` closes.
