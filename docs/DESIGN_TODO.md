# Design TODO — work queue & deferred ideas

Things we deliberately decided *not* to build yet, with enough context to pick them
up later — plus, merged in as of 2026-07-05, the findings of the full project review
(formerly `docs/REVIEW_FINDINGS.md`). Behavioral claims marked "verified" were
reproduced by running real scripts through the parse → resolve → eval pipeline.

## Resolver/evaluator gap tracker (actionable work queue)

Unlike the rest of this doc, these are **not** deferred ideas — it's the prioritized
inventory of what the parser already accepts but resolver/evaluator don't handle yet
(re-snapshot: 2026-07-05 post-review; done items are removed, not struck through;
numbering restarted — old #1–#12 are absorbed below). Priorities:
P1 = silent correctness holes / crashes in what already "works", P2 = parser-accepted
features the pipeline drops or handles wrong, P3 = polish/consistency.

Already shipped and no longer tracked here: `if` (resolve + eval), binding kinds
(`Binding{T, Kind}`, `Assignable()`, NOT_ASSIGNABLE), input/output decl resolution
(inline-type synthesis, `Record`, env binding), `emit` end-to-end (statement-position
`emit(target, ...)` → `resolved.EmitStmt` → evaluator with the temporary `Emitted()` sink),
output-read check (outputs are emit-only: `let x = alert` → OUTPUT_NOT_READABLE),
panic recover at the `EvalRun` boundary (stopgap: `5 % 0` no longer crashes the host;
real failure protocol is #10).

| # | Prio | Area | Missing | Details |
|---|------|------|---------|---------|
| 2 | P1 | resolver | Bare `CallExpr` (`foo()`) → silent `BadExpr`, no diagnostic | Breaks "resolution gates evaluation" — verified: compiles clean, dies at eval with `not implemented expression *resolved.BadExpr` (internal type name leaks to the user). Two sub-cases: (a) callee ident is `emit` → precise diag "emit is a statement and cannot be used as a value" (statement-position emit is intercepted upstream in `resolveStmt`, so reaching `resolveExpr` with an emit call *is* expression position, e.g. `let x = emit(...)`); (b) any other bare call → "unknown function"-style diag (no `LookupFunc` table exists; don't build one until bare functions are a real feature). Related decision to record: `emit` is matched by callee name, so `let emit = 3` does not shadow it — keyword-by-convention |
| 3 | P1 | resolver | `IndexExpr` (`a[i]`) — no case in `resolveExpr`, falls to silent-`BadExpr` default | Verified: same eval-time death as #2. **Nothing is indexable today**: registry has no index-rule table (`VectorShape` declared but unused), so the fix is diag-always, not a feature. Plan: resolve `Left` and `Index` first (their errors surface), then `NOT_INDEXABLE` diag (`Integer is not indexable`) with the infix-style dedup guard (only add if no new errs) + `BadExpr`. When built-in indexed state lands: add `LookupIndex(receiverType, indexType)` to the registry and this exact path becomes the real impl — lookup success builds the already-existing `resolved.IndexExpr`, lookup failure keeps the same diag. Do NOT add an empty registry table before then |
| 4 | P1 | resolver | `resolveConst` bails out of the whole loop on first duplicate (`return resolvedConsts` instead of `continue`) | One-line fix. Verified: `const A = 1` / `const A = 2` / `const C = undefined_name` reports only the duplicate — C is neither resolved nor checked, its error hidden |
| 5 | P1 | resolver | `resolveTypeDecl` silently swallows the `RegisterScriptType` error (resolver.go:266-270, `// unreachable` comment) | Reachable on registry reuse: a second `Compile()` against the same registry drops the input/output decl with **no diag**, then every use reports a misleading `UNDEFINED_IDENT` (verified: `emit(sig, …)` → "unknown identifier sig", zero hint at the cause). Even as a defensive branch it must append a diagnostic — silently dropping declarations violates the project's own "fail loudly" principle. Full fix (purge-on-reuse) stays in #13; the diag is a 3-liner now |
| 6 | P2 | all | `state.*` — the spec's entire persistence story (§3.1/§4.3) is absent | **Biggest missing part**; every spec example depends on it (cooldowns, debouncing). `state.cooldown = 0` parses (member-target assign) and dies in the resolver with a generic INVALID_ASSIGN_TARGET; no env entry, no runtime store, no STATE_UNSET diag. Was missing from this tracker despite matching its premise. Needs a design slot: a persistent store with Init/Run lifetime — interacts with #9 (Init semantics) and #13 (host API), and is part of the realistic end-to-end fixture |
| 7 | P2 | evaluator | Runtime input binding stopgap — scripts *declaring* inputs resolve but die at eval ("unknown identifier") because no values ever reach the env | Sketch agreed: `EvalRun(inputs map[string]registry.Value)` — for each `prog.Inputs`: missing → error, `value.TypeID() != decl.T` → error, else bind into the **per-run** env (not top-level; fresh values per bar). Same "temporary until host API" contract as `Emitted()`. Note: changes `EvalRun`'s signature → touches `evalSrc` test helper. These runtime checks are legitimately eval's job (host values arrive at run time; resolver can't vouch for them) |
| 8 | P2 | both | `&&` / `\|\|` — parsed, but no `BinaryRule` registered | Verified: `true && false` → bogus `Bool can't be [&&] -> && with Bool`. Design decision pending: registry rule (eager, zero new machinery) vs dedicated resolved node (short-circuit semantics — spec §3.6 promises short-circuit). Last real parser-accepted feature with no pipeline support; scripts with `if` will trip over this first |
| 9 | P2 | evaluator | `EvalInit` is a stub (`return nil, nil`); consts meanwhile are evaluated inside `EvalRun` — once per bar — and re-`Set` into the persistent env (evaluator.go:40-45) | Init bodies silently do nothing; needs persistent-env semantics (Init writes outlive the call, unlike Run). Const RHS is spec'd "evaluated once at program load" (§3.2) — move const evaluation to the load/init phase when this lands; per-bar re-eval is wasted hot-path work now and semantically wrong once state/indicators exist |
| 10 | P2 | registry/evaluator | `EvalFn` signatures can't fail: `func(left, right Value) Value` — runtime errors are structurally unreportable | Decide the failure protocol (`(Value, error)` returns vs a defined panic/recover boundary in the evaluator) **before** the talive integration: indicators will fail for real (bad period from a runtime expr, insufficient data — spec §3.4 requires runtime errors), and every registry entry added first enlarges the migration. Absorbs the old runtime-errors item: bare `fmt.Errorf`, no position, `PhaseRuntime` diag unused — do positions as part of the same protocol. Also owns the div-by-zero behavior family (all inconsistent today): int `%` 0 panics → recovered to an error by the `EvalRun` stopgap, int `/` 0 → `+Inf` silently, float `%` 0.0 → `NaN` silently |
| 11 | P2 | resolver | No reserved-name / shadowing rules | All verified accepted-without-diag: `let math = 5` shadows the module inside a function (spec: RESERVED_REASSIGN); `let m = math` then `m.sqrt(9.0)` — modules are first-class, aliasable values (spec §5.4: passive syntactic prefixes, not values — a bare module ident in value position should diag); `const Init = 5` coexists with `function Init()` — function names never enter the top-level env (spec §3.3: one namespace). Decision needed alongside: same-scope `let x` redeclaration currently silently rebinds, even with a new type (JS errors on it; spec silent) |
| 12 | P2 | resolver | `emit` semantic checks: placement, payload shape, reserved kwargs | Verified gaps: (a) emit inside `Init()` resolves + evals clean — spec §5.2 wants EMIT_OUTSIDE_RUN (`resolveFunc` is identical for both fns; needs a context flag); (b) positional args fill *structured* outputs — `emit(sig, "down", 0.5)` — spec says kwargs-only, and `TestResolver_ResolveEmit` blesses the behavior: decide, then fix test or spec; (c) `emit(logs, value="hi")` — the synthetic `value` param name from `Registry.EmitRule` leaks into the surface language (kwargs on value outputs should be rejected); (d) reserved kwargs `ts`/`output` (spec: EMIT_RESERVED_KWARG) unenforced — cheap to reject at output-decl resolution before host wiring lands |
| 13 | P2 | both | Engine/host API rework — replaces BOTH temporary APIs: the `Emitted()` emission sink (TODO in evaluator.go) and the `EvalRun(inputs)` stopgap from #7 | Includes registry purge-on-reuse (see #5 for the immediate diag fix). Design the API around the now-runnable end-to-end fixture: struct input → compute → emit. See also "Rethink the public API surface" below — that section now also carries the resolver-contract and registry-encapsulation notes from the review |
| 14 | P2 | diag | Diagnostics quality pass: machine-readable interface, code reconciliation, message hygiene | `diag.Diagnostic` is just `Error() string` — spec §6.2/§6.4 promises tooling phase/code/pos without parsing messages; add `Phase()`/`Code()`/`Pos()` accessors while the type count is small. Fix misspelled external-contract codes **before** tooling matches on them: `TYPE_MISSMATCH`/`ARGS_NUMBER_MISSMATCH` (spec: `TYPE_MISMATCH`), `UNDEFINED_MEETHOD`. Stop leaking `Token.String()` debug format into messages ("unknown identifier [IDENTIFIER] -> sig") — use `.Literal`; affects ~9 diag types. Reconcile the code set with spec §6.4: UNEXPECTED_TOP_DECL vs TOP_LEVEL_FORM, DUPLICATE_DECLARATION vs PORT_DUPLICATE, INVALID_OPERATION shared by two diag types, EMPTY_FUNCTION absent from spec (also decide whether an empty `Run` is even illegal — spec never says so). Phases: spec defines two, impl has three (`PhaseCheck`), and `DuplicateDeclaration` ships with PhaseParse from the parser but PhaseCheck from the resolver (the NOTE at parser.go:100 already doubts it) |
| 15 | P2 | spec/both | Type-system divergence: the Int/Float split already shipped; spec §3.4 still locks a single `Number` (float64) | Decide which is truth. The decision owns: user-facing type names (schemas say `Float`/`Integer` today), `/` semantics (already float-returning), overflow rules (int arithmetic wraps silently — verified `9223372036854775807 + 1`; and `-9223372036854775808` fails to parse since minus is an operator and the bare literal overflows Atoi), and the §3.4 integer-param validation story, which assumes `Number` + metadata |
| 16 | P2 | parser | Forbidden function name → error cascade (parser.go:238-241) | `function Foo() {…}` returns without consuming the body; top-level recovery (`syncToNewLine`) lands inside it → verified: FORBIDDEN_FUNCTION + one UNEXPECTED_TOP_DECL per body line + one for the closing `}` (4 errors for a 2-line body). Parse the body normally (or skip balanced braces) and report once |
| 17 | P2 | parser | `else` on its own line fails with misleading "expected expression got else" (stmt.go:90) | The peek for ELSE doesn't cross NEWLINE; `} // comment` before `else` fails too (both verified). JS — the surface model — allows Allman-else. Either skip newlines before the ELSE check (unambiguous: `else` can't start a statement) or keep the restriction and emit a dedicated "else must follow } on the same line" diag |
| 18 | P2 | evaluator | `evalIf` silently treats a non-Bool condition as false (evaluator.go:95: `condBool, _ :=`) | One-line guard (`ok` check → error). The resolver normally guarantees Bool, but #2/#3 prove the invariant can break; in a signal product, a silently skipped branch (silently *not emitting*) is the worst failure mode — fail loudly |
| 19 | P3 | resolver | Assignment doesn't coerce (`let x = 1.5` then `x = 1` → TypeMissmatch) | Inconsistent with args/infix which coerce Int→Float |
| 20 | P3 | resolver | `resolveArgs` kwarg diagnostics: unknown kwarg silently skipped; positional+kwarg duplicate → actively wrong message | Verified: `math.pow(2.0, base=3.0)` reports "missing exponent arg" — the user passed `base` twice; the count check + silent kwarg-over-positional overwrite convert "duplicate arg" into "missing other arg". Fix: `resolvedIdx[i]` already true → "duplicate argument"; no rule match → "unknown keyword argument". Also: for emit, the count diag says "expected 2 args but got 1" counting only value args, though the user typed the target too — fix wording together with #21 |
| 21 | P3 | resolver | `CallArgExpr.Token` is the call token (existing TODO in `resolveArgs`) | Arg-level errors point at `(` instead of the arg |
| 22 | P3 | both | Bad-node invariant unchecked (the rolled-back `containsBad` walker) | Would have caught #2–#3 mechanically |
| 23 | P3 | resolver | No Unknown-type poisoning — one error cascades | `let x = <bad expr>` binds `x` as `UnknownTypeID`; every later use emits a fresh INVALID_OPERATION/TYPE_MISSMATCH (the `errsBefore` guard only dedups within one expression tree). Standard fix: lookups where an operand is Unknown succeed silently as Unknown |
| 24 | P3 | lexer | Raw newlines are legal inside string literals (`readString`) | Verified: `"line1<LF>line2"` lexes as one STRING. Consequence: an *unterminated* string swallows the file to the next quote — one confusing far-away error, recovery wrecked for everything between. Spec is silent on multi-line strings. Recommend: newline in string → `unterminated string` at the opening quote, terminate the token at line end (standard single-line recovery) |
| 25 | P3 | parser | Spec §7 resource limits: only nesting depth (64) exists | The parse-error cap (spec: 100) is a few lines and bounds worst-case memory — the parser currently collects unbounded diagnostics. String-length, identifier-length, source-size, kwarg-count caps can wait |

Suggested order: #7 first (finishes the
input/output arc; makes a realistic script — struct input, condition, emit — runnable
end-to-end for the first time, which is also the fixture #13 should be designed around).
Then the P1 batch #2–#5 (same disease: silent holes reachable from valid parses, failing
without diagnostics). Then #8, then #9. #6 (state) needs its own design pass and should
land before #13 is shaped; #10 must be settled before the talive/registry surface grows;
#14 before any external tooling consumes diagnostics. #13 last, informed by the stopgaps
being used in anger.

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

- `/` by zero, `math.sqrt(-1)` — the silent `+Inf`/`NaN` family, pin behavior when #10
  decides it (`%` by zero is covered since the recover stopgap landed).
- emit-inside-Init (#12), shadowing cases (#11), Allman-else (#17),
  compile-twice-against-one-registry (#5).
- Evaluator has zero `Init` coverage (stub, but a test pinning "Init writes persist
  into Run" is the executable spec for #9).
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
gone; don't let type re-checking creep back in. (Pairs with tracker #10 — the failure
protocol is what will consume these positions.)

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
  `resolved.IndexExpr` (#3), `diag.PhaseRuntime` (#10).

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
