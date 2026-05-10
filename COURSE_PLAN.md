# Trading Language Interpreter — Course Plan

> Based on "Writing An Interpreter In Go" by Thorsten Ball
> Language theme: a DSL for computing **indicators** and emitting **signals** from price data.
>
> **Scope (locked):** indicators + signals + output only. No `buy`/`sell`/`hold`, no order
> execution, no portfolio state, no backtest engine. Indicator math comes from the `talive`
> library — we do NOT reimplement indicators. Input is a generic candle stream (CSV/JSON,
> format TBD, not a focus).
>
> **Syntax flavor:** TypeScript-style — braces, `let`/`const`, `function` keyword, `;`
> optional, C-style operators. No arrow functions. No significant whitespace (so no
> INDENT/DEDENT tokens). No static type annotations — dynamic like untyped JS.

## How This Works

- Each lesson has: **Concept** → **Task** → **Review** → **Challenge**
- Mark lessons `[x]` when completed
- At the start of each session, tell Claude: "Let's continue the trading-lang course" and it will read this file

---

## Current Status

**Current Lesson:** 5.5 complete
**Last Session Date:** 2026-05-10
**Notes:** Best-effort memory accounting shipped. `MaxLiveBytes` (64 KB default) lives in `object.Limits` next to per-object caps; `currentLiveBytes` package var in evaluator resets alongside `currentOpCount` in the `*ast.Program` arm of `Eval`. Single `accountFor(obj) Object` helper sums sizes for tracked types (`*String`=len, `*Series`=len*8, `*CandleSeries`=len*5*8) and returns `*Error` over limit. Wired at four allocation sites: string literal, string concat, `extractColumn`, `runIndicator`. `extractColumn`'s signature widened from `*object.Series` to `object.Object` so the error path can flow — callers already returned `Object` so no other changes needed. Two bugs surfaced and fixed mid-lesson: (1) initial draft did `accountFor(x); return x` at every site, throwing away the error — fix was `return accountFor(...)`; (2) `runIndicator` (the most allocation-heavy path) was unaccounted entirely — added the wrap. Five tests cover within-limit sanity, cumulative concat, cumulative series via `sma()`, `extractColumn` overflow, reset-between-programs. All green; full suite green.

---

## Module 1: Lexing (Chapters 1–2 of the book)

The lexer turns raw source code into tokens — the smallest meaningful pieces of the language.

- [x] **1.1 — Hello Tokens**
  - What tokens are, why we need them
  - Define the token types for our language
  - Task: Define the `Token` struct and `TokenType` constants
  - Keywords (TS-flavored): `let`, `const`, `function`, `return`, `if`, `else`, `true`, `false`
  - Operators: `=`, `==`, `!=`, `<`, `>`, `<=`, `>=`, `+`, `-`, `*`, `/`, `!`, `&&`, `||`
  - Punctuation: `(`, `)`, `{`, `}`, `[`, `]`, `,`, `;`, `:`
  - Domain identifiers (plain idents, not keywords): `signal`, `sma`, `ema`, `rsi`, `close`, `open`, `high`, `low`, `volume`

- [x] **1.2 — The Lexer**
  - Build a lexer that reads source code character by character
  - Task: Implement `Lexer` struct with `NextToken()` method
  - Lex simple expressions: `sma(close, 14)`

- [x] **1.3 — Extending the Lexer**
  - Support numbers (integers and floats), strings, comparison operators
  - Task: Lex full expressions like `sma(close, 14) > 50.0`
  - Challenge: Lex a multi-line strategy definition

- [x] **1.4 — The REPL (Read-Eval-Print Loop), Part 1**
  - Build a simple REPL that tokenizes input and prints tokens
  - Task: Create `main.go` with a working token REPL

---

## Module 2: Parsing (Chapters 2–3 of the book)

The parser turns a flat list of tokens into a tree (AST) that represents the structure of the code.

- [x] **2.1 — AST Foundations**
  - What an AST is, node types (expressions vs statements)
  - Task: Define AST node interfaces and basic node types
  - Trading twist: `let signal = sma(close, 14) > ema(close, 21)`

- [x] **2.2 — The Parser, Part 1: Statements**
  - Recursive descent parsing, `let`/`const` and `return` statements
  - Task: Parse `let`/`const` statements and `return` statements
  - Example: `const entry = rsi(close, 14) < 30;`

- [x] **2.3 — The Parser, Part 2: Expressions (Pratt Parsing)**
  - Pratt parser for operator precedence
  - Prefix and infix expressions
  - Task: Parse arithmetic and comparison expressions
  - Example: `sma(close, 14) + atr(14) * 2 > 100`

- [x] **2.4 — The Parser, Part 3: Grouped, If, and Functions**
  - Grouped expressions, if/else, `function` literals
  - Task: Parse conditionals that produce signal values
  - Example: `if (rsi(close, 14) < 30) { "oversold" } else { "neutral" }`

- [x] **2.5 — Function Calls & Parameters**
  - Parse call expressions with arguments
  - Task: Parse `sma(close, 14)`, `signal("rsi_low", rsi(close, 14) < 30)`
  - Challenge: Parse a complete signal-definition block

---

## Module 3: Evaluation (Chapters 3–4 of the book)

The evaluator walks the AST and actually executes the code.

- [x] **3.1 — The Object System**
  - Internal representation of values (integers, floats, booleans, null)
  - Task: Define the object system
  - Trading twist: add a `Series` type for price data

- [x] **3.2 — Evaluating Expressions**
  - Tree-walking evaluation of arithmetic, booleans, prefix/infix ops
  - Task: Evaluate `(50 + 10) * 2 > 100` → `true`

- [x] **3.3 — Conditionals and Environments**
  - If/else evaluation, variable bindings, environment (scope)
  - Task: Evaluate `let x = 10; if x > 5 { x * 2 } else { x }`

- [x] **3.4 — Functions and Closures**
  - Function evaluation, closures, call stack
  - Task: Define and call indicator-helper functions
  - Example: `const doubleAtr = function(period) { atr(period) * 2 }; doubleAtr(14);`

- [x] **3.5 — The REPL, Part 2**
  - Upgrade REPL to evaluate expressions and show results
  - Task: Full working REPL that can evaluate indicator/signal expressions

- [x] **3.6 — Member Access**
  - Add a generic `obj.prop` expression: lexer `DOT` token, `MemberExpression` AST node,
    infix parser at CALL-level precedence, evaluator dispatch on receiver type.
  - Lexer care: don't break float literals — `3.14` stays a float, but `foo.bar` lexes as
    IDENT DOT IDENT.
  - Canary use: expose `.length` on `String` (e.g. `"hi".length` → `2`) so 3.6 has a real
    end-to-end test without needing the `Candles` type yet.
  - Errors: unknown property → runtime error; receiver type with no properties → runtime error.
  - Tests: lexer (no float regression, ident.ident sequence), parser (`a.b`, `a.b.c`,
    `f().x`, precedence vs `+`), eval (string `.length`, unknown prop, bad receiver).

---

## Module 4: Indicators & Signal Output (beyond the book)

Make the language useful for computing indicators and emitting signals from a candle stream.
**Out of scope:** orders, positions, PnL, backtesting.

- [x] **4.1 — Candle Input (AoS)**
  - Thin host-side glue, NOT part of the DSL surface — will be replaced when the DSL gets
    a real way to ingest data. No tests for the loader itself; it's just plumbing.
  - **Storage shape: array of structs.** A `Candle` is one bar with float fields
    `open / high / low / close / volume`. A `CandleSeries` holds `[]Candle`.
  - Member access surface (relies on 3.6):
    - `candle.open / .high / .low / .close / .volume` → `*Float`
    - `candleseries.opens / .highs / .lows / .closes / .volumes` → `*Series` built on
      the fly by walking the slice. Recompute each call (no caching — premature).
  - CSV-only loader (header `open,high,low,close,volume`) lives next to the REPL.
  - REPL auto-loads `./data.csv` if present and seeds env binding `candles`. No flag.
    Missing file = REPL still starts normally; malformed file prints an error but launches.
  - Tests (evaluator side only): `candles.closes` returns Series, `candles.closes.length`
    chains through 3.6, single Candle scalar accessors, unknown-prop errors on both types.

- [x] **4.2 — Built-in Indicators via talive**
  - Add a builtin-function mechanism (new object kind, separate from user `Function`,
    backed by a Go closure over `[]object.Object`).
  - Wire `sma`, `ema`, `rsi` to the `talive` library — at least 3 indicators.
  - Builtins consume `*Series` and return `*Series`. talive owns the math; we never
    reimplement indicators.

- [x] **4.3 — Indexing**
  - Add `IndexExpression` end to end: `[` `]` tokens already exist (1.1), needs a Pratt
    entry (CALL-level precedence or higher), an AST node, and evaluator dispatch.
  - Index into `*Series` → `*Float`. Index into `*CandleSeries` → `*Candle`.
  - Decide indexing convention now: `[0]` = oldest or newest? Negative-index semantics
    (`[-1]`)? Lock the choice in the lesson notes when shipped.
  - Out-of-bounds → runtime error.
  - Tests: positive/negative indices on both series types, oob errors, chained
    `candles[-1].close`.
  - **Locked:** 0-based (oldest = `[0]`), negative indices rejected with explicit error,
    out-of-bounds = error.

- [x] **4.4 — Signals & Output**
  - **Resolved:** per-bar host loop wins over broadcasting. Language stays scalar; no
    broadcasting infix, no `bar` keyword. The host re-evaluates the program each tick
    with `candles` regrown. Newest-first indexing (`[0]` = latest bar) makes scripts
    read naturally without any DSL-level loop construct.
  - `signal(text)` is a one-arg builtin: writes `received signal: %s\n` to
    `evaluator.SignalOutput` (default stdout, redirectable for tests), returns NULL.
    Per user: real signal interface will be different in production; this is just
    enough for the lesson.
  - Per-bar host driver is OUT OF SCOPE — user explicitly deferred ("real execution
    environment will be different"). Indexing reversal is the one architectural change
    that actually persists.

- [x] **4.5 — Final Project**
  - Write a signal-only program using several indicators (e.g. RSI cross + SMA filter).
  - Review: clean up, add tests, reflect on architecture decisions made in 4.1–4.4.
  - **Shipped:** `examples/demo.tas` runs end-to-end via `tascript examples/demo.tas`,
    using `rsi`/`sma`, indexing, member access, and 5 signal emissions. 4 of 5 fire on
    the bundled `data.csv`. Lexer regression fix (digits in identifiers) caught by
    failing parser tests — added `TestLexer_IdentifierWithDigits` regression test.

---

## Module 5: Sandbox & Limits (beyond the book)

Make the interpreter safe to run untrusted scripts. Lessons 5.1–5.5 cover **soft,
in-process** limits the interpreter enforces itself; 5.6 swaps those for **hard,
structural** limits by running the interpreter inside a WASM sandbox. Real
per-process RSS / CPU caps via cgroups or `setrlimit` are OS-level and stay out
of scope — but 5.6 is the architectural answer to "what does proper sandboxing
look like in Go," and pays off the soft-vs-hard story the rest of the module sets up.

- [x] **5.1 — String & Collection Size Caps**
  - Hardcoded global `object.DefaultLimits` (`MaxStringLength`, `MaxSeriesLength`,
    both 1024). User decided against per-env config / chain inheritance — global is
    enough for now. `Environment.Limits()` just returns the package var.
  - Enforced at: string literal eval, string concat (`+` on strings), and the
    builtin indicator path. Builtin check is on **input** `len(candles.Value)` via
    `runIndicator`, not output — works for talive (same-length output) but is the
    spot to revisit if a future builtin grows the series.
  - **Skipped:** `extractColumn` (`candles.closes` etc.) — host owns the CSV, no
    DSL-side path to grow `candles`. Revisit if a slicing builtin lands.
  - Violation = `*object.Error` carrying the offending size + the limit name.
  - Builtin signature changed to `func(env *object.Environment, args ...) Object` —
    threaded through `evalBuiltin`, `runIndicator`, and the four exported builtins.

- [x] **5.2 — Operation Budget**
  - Package-level `const opLimit = 10_000` + `var currentOpCount` in evaluator
    (no Limits-struct field — kept consistent with 5.1's hardcoded-global stance).
  - `Eval` increments + checks at the top for every node; `*ast.Program` is
    special-cased to reset the counter and bypass the check (entry-node-only path).
  - Lesson gotcha: reset MUST happen before the increment+check, or the leftover
    counter from a previous run trips immediately. Verified with
    `TestOperationsCounterResetsBetweenPrograms` (runaway test followed by a
    trivial `1 + 1`).
  - Non-reentrant by design — comment in `evaluator.go` notes that two scripts
    cannot run concurrently in one process. Acceptable for the teaching scope.

- [x] **5.3 — Wall-Clock Deadline**
  - `Eval` signature: `Eval(ctx context.Context, node ast.Node, env *object.Environment)`.
    No shim — direct change, every call site updated. `ctx` threaded into all helpers
    that recurse (`evalProgram`/`evalBuiltin`/`evalFunc`/`evalBlock`).
  - Check is `ctx.Err() != nil` near the top of `Eval`, right after the op-budget check.
    Returns `newError("deadline exceeded")`. Op-budget wins when both would trigger
    on the same call; flagged for future flip if "stop now" deadline semantics are wanted.
  - Builtins do NOT yet observe `ctx` — talive math is CPU-bound and won't be cancelled
    mid-flight. Documented as a future concern (e.g. when network-touching builtins land).

- [x] **5.4 — (Stretch) Static Validation Pass**
  - `evaluator/validator.go`: `Validator` struct + `Validate(node, scope) ObjectType`.
    Recursive walk mirroring `Eval`'s shape but tracking `ObjectType` per name in a
    new `object.Scope` (sibling to `Environment`, `outer` chain). Pre-seeded with
    `candles → CandleSeriesKind`; let/const propagate RHS kind; identifier lookup
    resolves transitively.
  - Concrete rule shipped: indicator builtins (`sma`/`ema`/`rsi`) require first arg
    to be `CandleSeriesKind`. `checkIndicatorCall` enforces; runs before `Eval`.
  - Function literal bodies walked at definition time — finds violations in code
    that may never run. Params get `KindAny` (no caller info statically).
  - Known false positives pinned in tests: function-param case, scoped binding
    leaking out of an if-block. Known minor bug: nested indicator calls produce
    duplicate errors (`checkIndicatorCall` and the arg-loop both walk arg[0]).

- [x] **5.5 — (Stretch) Best-Effort Memory Accounting**
  - Track the sum of `len()` of live strings + series + candle series via
    wrapper allocations. Reset counter between program runs (same place as
    `currentOpCount`); pre-existing host objects (e.g. `candles` from CSV) are
    intentionally uncounted because they're allocated before `Eval` starts.
  - `MaxLiveBytes` lives in `object.Limits` next to the per-object caps; default
    64 KB. Sizes: string = `len(Value)`, series = `len(Value)*8`, candle series
    = `len(Value)*40`.
  - Single `accountFor(obj) Object` helper bumps counter for tracked types and
    returns `*Error` on overflow. Wrap **every** allocation site for the three
    types: string literal, string concat, `extractColumn`, `runIndicator`.
    `extractColumn` returns `Object` (not `*Series`) so the error path can flow.
  - Document the gap in a comment near the helper: cumulative-only (no
    decrement), intermediate values count even though Go GC reclaims them, host
    objects don't count, real RSS limits live in WASM/cgroups (see 5.6).

- [ ] **5.6 — (Stretch) WASM Sandbox Runner**
  - Pay off Module 5 by replacing the soft, in-process limits from 5.1–5.5 with
    **hard** limits enforced by a WASM runtime. Same tascript surface; same
    syntax; same `examples/demo.tas` runs; the change is structural — the
    interpreter now lives inside a sandbox the host process controls.
  - Architecture (Stage 2 from the WASM evolution discussion): compile the
    existing Go interpreter for `GOOS=wasip1 GOARCH=wasm`, producing a single
    `tascript-interp.wasm`. New host program (`cmd/sandbox/main.go`) embeds
    [wazero](https://github.com/tetratelabs/wazero) (pure-Go, no CGO) and
    instantiates a fresh sandbox per script run. Each instance gets its own
    bounded linear memory and its own fuel budget — a runaway script can't
    escape the byte slice the host allocated.
  - Host ABI design: minimal at first. The host writes the script source into
    the module's memory and calls `run()`; the module calls back through
    imports to read candles (`candle_count()`, `candle_close(i)`, etc.) and
    to emit signals (`emit_signal(ptr, len)`). Numbers and `(ptr, len)` pairs
    are the only things that cross the boundary — WASM has no string type.
  - Limits comparison (the teaching payoff): show the soft 5.1–5.5 caps still
    work at the language level, then show that even with those disabled the
    WASM memory cap and fuel limit catch a runaway script. The same script
    that returns `*Error` in soft mode causes a wazero **trap** in WASM mode.
    Trap → host receives an error, sandbox is torn down cleanly, host stays
    healthy. That's the difference between best-effort and structural.
  - Out of scope (deferred): multi-language host (Stage 4 — running arbitrary
    user-supplied `.wasm` modules conforming to the ABI); compiling tascript
    itself to WASM (Stage 3 — replacing the interpreter with a compiler); WASI
    filesystem/networking. Stays a Stage-2 lesson: same language, hard sandbox.
  - Likely to split into 5.6a (WASI build of the interpreter + minimal host)
    and 5.6b (host ABI for candles/signals + comparison tests). Decide when
    we get there.

---

## Session Log

| Session | Date       | Lessons Covered | Notes |
|---------|------------|-----------------|-------|
| 1       | 2026-04-14 | 1.1, 1.2        | Token types defined; lexer handles single-char tokens, identifiers, keywords, ints. |
| 2       | 2026-04-15 | 1.3, 1.4        | Extended lexer (floats, strings, `==`/`!=`/`<=`/`>=`/`&&`/`||`, `//` comments). REPL built with I/O decoupling + tests. |
| 3       | 2026-04-17 | 2.1, 2.2        | AST foundations + parser for `let`/`const`/`return` statements. Expression parsing stubbed. |
| 4       | 2026-04-20 | 2.3             | Pratt parser: prefix/infix fn maps, precedence table, `PrefixExpression` and `InfixExpression` AST nodes, full precedence-string tests. |
| 5       | 2026-04-21 | 2.4, 2.5        | Grouped/if/function literals/call expressions/string literals. Arrow functions removed from scope. Shape + table-driven tests. Module 2 complete. |
| 6       | 2026-05-02 | 3.1             | Object system: `Object` interface + Integer/Float/String/Boolean/Null/Series. `int64` for Integer. `Kind`-suffixed Go consts, lowercase type strings. |
| 7       | 2026-05-03 | 3.2             | Tree-walking evaluator for literals + prefix/infix ops. Singletons, int↔float promotion, error objects, div-by-zero, no-mutation regression test. |
| 8       | 2026-05-03 | 3.3             | Environment with `outer` chain. `let`/`const`, identifier lookup, `if`/`else` as expressions, block statements, truthiness rule. Tests for nested-scope reads + error propagation. |
| 9       | 2026-05-05 | 3.4             | Functions + closures. `object.Function{Params, Body, Env}`. Args eval'd in caller env, bound in fresh enclosed env (defn-env never mutated). Tests: identity, closure, closure isolation, recursion, non-fn call, arity. |
| 10      | 2026-05-05 | 3.5             | REPL Part 2: persistent env, parser-error path, eval+print. `let`/`const` return bound value. Module 3 complete. |
| 11      | 2026-05-06 | 3.6             | Member access: DOT token, `MemberExpression` AST, infix at CALL precedence, evaluator dispatch (`String.length`, `Series.length`). Pre-existing fixes: `return` statements via `*object.Return` wrapper; `&&`/`||` eval; FunctionCall error propagation + `not a function` message. |
| 12      | 2026-05-06 | 4.1             | Candle input (AoS): `Candle` + `CandleSeries{Value []Candle}`, evaluator member access for scalar accessors and column extraction (via `extractColumn` helper, recompute each call). CSV loader + REPL auto-seed of `candles` from `./data.csv`. Module 4 plan reworked: 4.3 now also adds `IndexExpression`. |
| 13      | 2026-05-07 | 4.2             | Built-in indicators via talive. New `*object.Builtin` kind; FunctionCall arm type-switches `*Function`/`*Builtin`/else. Lowercase `sma`/`ema`/`rsi` share a `runIndicator(name, args, factory)` helper; talive `OHLCV` satisfied by a private `ohlcvAdapter` in `builtin.go` — `object.Candle` stays a plain struct with long field names. Builtins take `*CandleSeries` + `*Integer` → `*Series`. `RegisterBuiltins(env)` lives in evaluator package, called from REPL. Tests cover dispatch mechanism + real talive math. |
| 14      | 2026-05-07 | 4.3             | Indexing end-to-end. `LBRACKET` Pratt entry → `parseIndexExpression`; new `INDEX` precedence between `PREFIX` and `CALL`; `parseIndex` broadened to a full expression. Evaluator dispatch: `*Series` → `*Float`, `*CandleSeries` → `*Candle`. Convention locked: 0-based, no negatives (explicit error), out-of-bounds = error. TDD-driven: parser tests written first (shape + precedence-string), then evaluator tests surfaced two bugs (Go panic on negative index, `%d` against `Object` wrapper) that user fixed. |
| 15      | 2026-05-07 | 4.4             | Signals + indexing reversal. Newest-first lookup: `Value[len-i-1]` in evaluator (4.3 tests flipped). `signal(text)` builtin writes `received signal: %s\n` to swappable `SignalOutput` writer, returns NULL. Per-bar host loop deferred — user said real runtime will be different. Scalar-only DSL confirmed: no broadcasting, no bar keyword. Module 4 wraps with one lesson left (4.5 final project). |
| 16      | 2026-05-08 | 4.5             | Final project. Script-runner mode in `main.go` (one-shot: load candles, register builtins, eval script). `repl.LoadCandlesCSV` exported. `examples/demo.tas` fires 4/5 signals on 30-bar `data.csv`. Lexer bug found via demo: digits in identifiers (`s14`) weren't allowed; fix in `readIdentifier`, regression test added. Module 4 complete. |
| 17      | 2026-05-09 | 5.1             | Sandbox lesson 1: string + series size caps. Global hardcoded `object.DefaultLimits` (1024/1024); `Environment.Limits()` returns it directly. Three enforcement points (string literal, string concat, builtin input). `extractColumn` skipped intentionally. Builtin signature gained `env`. New `withLimits` test helper + 6 enforcement tests. Three pre-existing tests updated for new builtin signature. |
| 18      | 2026-05-10 | 5.2             | Op budget. `const opLimit = 10_000` + `var currentOpCount` in evaluator. Increment+check at top of `Eval`; `*ast.Program` early-returns with a counter reset (Option B). One ordering bug found: initial attempt put the reset inside the switch arm — unreachable because the increment+check above it tripped on the leftover counter. Caught by a sequential reset test. Cleanup pass: const/var split, `++` form, comment rewritten to flag non-reentrancy as the actual constraint. |
| 19      | 2026-05-10 | 5.3             | Wall-clock deadline via `context.Context`. `Eval` gained `ctx` as first arg, threaded through every recursion site. `ctx.Err()` check after op-budget check. All 14 call sites updated (main.go, repl.go, 12 in tests) to pass `context.Background()`. `TestEvalRespectsContextDeadline` confirms an already-expired context trips a `"deadline"` error. |
| 20      | 2026-05-10 | 5.4             | Static validation pass. New `Validator` struct + `object.Scope` (sibling to `Environment`). `Validate` mirrors `Eval`'s switch, tracks `ObjectType` per name, pre-seeds `candles → CandleSeriesKind`. Indicator-arg rule wired in `checkIndicatorCall`. Real bug found: `append(slice, fmt, args...)` doesn't format — fixed via `fmt.Sprintf`. False-positive cases (function params, block-scoped bindings) pinned in tests. 14 tests, all green. |
| 21      | 2026-05-10 | 5.5, 5.6 plan   | Best-effort memory accounting. `MaxLiveBytes` (64 KB) in `object.Limits`; `currentLiveBytes` package var resets per program. Single `accountFor` helper at four sites (string lit, string concat, `extractColumn`, `runIndicator`). `extractColumn` return widened to `object.Object` so the error path flows. Two bugs caught by tests during the lesson and fixed: discarded `accountFor` return at every call site, and `runIndicator` not wrapped at all (highest-allocation path). Five tests, all green. Discussion went deep on Go's per-goroutine accounting limits (no — heap is shared, BEAM is the contrast) and WASM evolution path (Stage 2 sandboxed interpreter vs Stage 4 multi-language host). 5.6 added to plan: Stage-2 wazero-based sandbox, Module 5 header rewritten to frame the soft-vs-hard limits arc. |

