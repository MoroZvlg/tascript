# Design TODO — deferred ideas

Things we deliberately decided *not* to build yet, with enough context to pick them
up later. None of these block the current resolver/evaluator work.

## Resolver/evaluator gap tracker (actionable work queue)

Unlike the rest of this doc, these are **not** deferred ideas — it's the prioritized
inventory of what the parser already accepts but resolver/evaluator don't handle yet
(snapshot: 2026-07-04). Priorities: P1 = silent correctness holes in what already
"works", P2 = parser-accepted features the pipeline drops, P3 = polish/consistency.

| # | Prio | Area | Missing | Impact |
|---|------|------|---------|--------|
| 1 | P1 | resolver | `CallExpr` with non-member callee (`foo()`, `emit(...)`) → `default:` returns `BadExpr` **without a diagnostic** | Breaks "resolution gates evaluation": zero diags, `Compile` passes, evaluator hits "not implemented expression" |
| 2 | P1 | resolver | `IndexExpr` (`a[i]`) — no case in `resolveExpr`, falls to silent-`BadExpr` default | Same silent hole; parser has `parseIndexExpr` wired |
| 3 | P1 | resolver | `resolveStmt` default returns `nil` (hits on `ast.IfStmt`, `ast.BadStmt`) | `nil` lands in `Stmts`, evaluator: "unsupported statement `<nil>`". Should be diag + `BadStmt` |
| 4 | P1 | resolver | `resolveConst` bails out of the whole loop on first duplicate (`return resolvedConsts`) | Consts after the duplicate are neither resolved nor checked — one error hides the rest |
| 5 | P1 | evaluator | `evalAssignName` uses `Get`+`Set` instead of `Env.Assign` | Shadow-binding bug the moment nested blocks (`if`) exist |
| 6 | P2 | both | `IfStmt` — no resolve case, no eval case (`resolved.IfStmt` node already exists) | Whole feature; resolver side must also require `Bool` condition |
| 7 | P2 | both | `&&` / `||` — parsed, but no `BinaryRule` registered | `true && false` → bogus `InvalidBinaryOperation`. Design decision pending: registry rule (eager) vs dedicated node (short-circuit) |
| 8 | P2 | both | Inputs/Outputs — resolver side DONE (2026-07-04): decl resolution, inline-type synthesis (`RegisterScriptType`, `Record`), env binding, carried into `resolved.Program`. Remaining: runtime input binding (host API) + output delivery to the host. Known lie: outputs sit in the env as readable vars until binding kinds land (TODO in `resolveOutputs`) | Runtime half is prereq for `emit` and for actually reading inputs in Run |
| 9 | P2 | both | `emit(...)` — resolver side DONE (2026-07-05): statement-position `emit(target, ...)` lowers to `resolved.EmitStmt{Output, Args, T}`; target must be `KindOutput` (INVALID_EMIT_TARGET otherwise); args checked via `resolveArgs` against a rule synthesized from the output type (scalar → one `value` param, structural → one param per field, coercion works). Evaluator side landed too (2026-07-05) with a **temporary sink**: emissions collect into `Evaluator.emitted`, read via `Emitted()`, reset per run — replace with real output channels to the host during the Engine/host API rework (TODO in evaluator.go). Known gaps: `emit` in expression position (`let x = emit(...)`) falls into #1's silent hole; `emit` is matched by name, so a `let emit = ...` binding doesn't shadow it | A resolved program using emit runs into "unsupported statement" at eval until the runtime half lands |
| 10 | P2 | resolver | Binding kinds — `Binding{T, Kind}` landed (2026-07-05), all env entries tagged (let/const/input/output/module); `Binding.Assignable()` (only `let`) rejects assignment to const/input/output/module with NOT_ASSIGNABLE. Remaining: reject *reading* an output (emit-only, decided 2026-07-05) — wire together with emit (#9) | `let x = alert` still resolves and dies at runtime |
| 11 | P2 | evaluator | `EvalInit` is a stub (`return nil, nil`) even though resolver resolves `InitFn` | Init bodies silently do nothing; needs persistent-env semantics (Init writes outlive the call, unlike Run) |
| 12 | P3 | resolver | Assignment doesn't coerce (`let x = 1.5` then `x = 1` → TypeMissmatch) | Inconsistent with args/infix which coerce Int→Float |
| 13 | P3 | resolver | `resolveArgs`: unknown kwarg silently skipped (`continue`), positional+kwarg duplicate for same param overwrites without diag | Wrong programs resolve cleanly |
| 14 | P3 | resolver | `CallArgExpr.Token` is the call token (existing TODO in `resolveArgs`) | Arg-level errors point at `(` instead of the arg |
| 15 | P3 | evaluator | Runtime errors are bare `fmt.Errorf`, no position; `PhaseRuntime` diag unused | Matters once runtime can genuinely fail (div by zero, sqrt of negative) |
| 16 | P3 | both | Bad-node invariant unchecked (the rolled-back `containsBad` walker) | Would have caught #1–#3 mechanically |

Suggested order: the P1 block first (items 1–3 are the same disease — "impossible"
default branches reachable from valid-parse programs, failing without diagnostics;
#4 is a one-line decision). Then 7 → 6 (`if` needs booleans) and 8 → 9 (`emit` needs
outputs), which together complete the language surface the parser already accepts.

## Slot-based variable access (perf)

**Status:** deferred. Starting with name → `TypeID` env and string-keyed runtime lookup.

The resolver currently maps names to `TypeID`. Later, assign each `let`/`const`/`input`
a **slot index** and emit `resolved.Ident{Slot, Type}` instead of a name. Evaluator reads
`slots[n]` (array index, ~1–2 ns) instead of a string-keyed map (~15–50 ns).

**Why it matters:** the `run()` body is re-walked **per bar** over thousands of candles.
Index access compounds; it's a real win, not premature. Also turns undefined-variable into
a resolve-time error and lets eval drop the locals map entirely.

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
gone; don't let type re-checking creep back in.

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

## Reconcile naming between `ast` and `resolved`

The two packages mirror each other node-for-node, but small inconsistencies have crept in
during the migration. Do a pass to align them (or deliberately document each intentional
divergence). Known so far:

- `MemberAccessExpr.Method` / `KwargsExpr.Key`: `*ast.IdentExpr` in `ast`, bare `token.Token`
  in `resolved`. (The `resolved` choice is cleaner — a method/kwarg name isn't a variable — so
  consider fixing the AST side instead.)
- `ast.KwargsExpr` implements `Expression`; `resolved.KwargsExpr` is a plain helper. Pick one.
- Node-name suffix conventions (`*Expr` / `*Stmt` / `*Decl`) — verify they match 1:1 and that
  wrappers line up (`ast.Program.InitFn/RunFn` ↔ `resolved.Program.InitFn/RunFn`, etc.).
- Field name `Name` type differs across nodes (`token.Token` vs `string`) — audit for
  consistency now that `ConstDecl.Name` is a token.

## Remove dead `ast.Declaration` interface

`ast.Declaration` (and the four `declarationNode()` markers on `InputDecl`/`OutputDecl`/
`ConstDecl`/`FunctionDecl`) is defined and satisfied but **never used as a type** — `Program`
holds declarations as concrete slices (`Consts []*ConstDecl`, etc.), not `[]Declaration`.
Dead code; drop the interface and the four marker methods. (Not mirrored in `resolved` for
the same reason — `resolved.Program` also uses concrete slices.)

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

