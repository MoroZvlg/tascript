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

**Current Lesson:** 3.4
**Last Session Date:** 2026-05-03
**Notes:** Conditionals + environments wired in. New `object/environment.go` with `Environment{store, outer}`, `NewEnvironment`, `NewEnclosedEnvironment`, `Get` (walks `outer`), `Set`. `Eval` signature now `Eval(ast.Node, *object.Environment)`. Handles `LetStatement`, `ConstStatement` (treated identically — no immutability enforcement yet), `Identifier` (returns `"identifier not found: %s"` error), `IfExpression` (no-else → `NULL`), `BlockStatement` (`evalBlock` mirrors `evalProgram`). Truthiness: `false`/`null` falsy, everything else truthy (book's middle path). `if` blocks do NOT create new scope (matches book; deferred). Tests cover let/const, if/else, nested-scope read, error propagation through `let`.

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

- [ ] **3.4 — Functions and Closures**
  - Function evaluation, closures, call stack
  - Task: Define and call indicator-helper functions
  - Example: `const doubleAtr = function(period) { atr(period) * 2 }; doubleAtr(14);`

- [ ] **3.5 — The REPL, Part 2**
  - Upgrade REPL to evaluate expressions and show results
  - Task: Full working REPL that can evaluate indicator/signal expressions

---

## Module 4: Indicators & Signal Output (beyond the book)

Make the language useful for computing indicators and emitting signals.
**Out of scope:** orders, positions, PnL, backtesting.

- [ ] **4.1 — Candle Input**
  - Load a candle stream from CSV or JSON into a `Series` value
  - Task: Minimal loader — accept either format, expose `open/high/low/close/volume` series
  - Not a focus: keep the loader small, don't over-engineer the format

- [ ] **4.2 — Built-in Indicators via talive**
  - Wire indicator builtins (`sma`, `ema`, `rsi`, …) to the `talive` library
  - Task: Bind at least 3 talive indicators as callable builtins
  - Rule: do NOT reimplement indicator math — always delegate to talive

- [ ] **4.3 — Arrays and Series Indexing**
  - Index expressions on series: `close[0]`, `close[-1]`
  - Task: Evaluate series indexing and pass series into indicator builtins

- [ ] **4.4 — Signals & Output**
  - A `signal(name, condition)` builtin (or `signal` block) that emits a named boolean/value per bar
  - Task: Run the program over a candle stream and print emitted signals (name, timestamp, value)
  - Example:
    ```ts
    const r = rsi(close, 14);
    signal("rsi_oversold", r < 30);
    signal("rsi_overbought", r > 70);
    ```

- [ ] **4.5 — Final Project**
  - Write a signal-only program using several indicators
  - Review: clean up code, add tests, reflect on the architecture

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

