# tascript

[![CI](https://github.com/MoroZvlg/tascript/actions/workflows/ci.yml/badge.svg)](https://github.com/MoroZvlg/tascript/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/MoroZvlg/tascript)](go.mod)
[![codecov](https://codecov.io/gh/MoroZvlg/tascript/branch/main/graph/badge.svg)](https://codecov.io/gh/MoroZvlg/tascript)
[![Go Reference](https://pkg.go.dev/badge/github.com/MoroZvlg/tascript.svg)](https://pkg.go.dev/github.com/MoroZvlg/tascript)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

An embeddable scripting language for **per-event block logic**, written in Go.

A tascript program is the logic inside one block on a dataflow graph: it declares typed
**input ports**, runs when the host activates it, keeps values across activations in
**slots**, and **emits** to **output ports** wired to host sinks.

> **inputs → logic + slots → emitted events**

The core is **domain-blind**. It knows primitives, control flow, ports, and a registry — it
does not know what a candle, an indicator, or an order is. Every domain word, type,
operator, and even every declaration keyword beyond `const` / `input` / `output` is supplied
by a **host** through the registry. One host per block type.

Status: **v0.1**. The normative language definition is [`docs/SPEC.md`](docs/SPEC.md).

## Install

```bash
go get github.com/MoroZvlg/tascript
```

## The language

```js
const THRESHOLD = 100

input metric: Float

output alerts: {
  level: String,
  value: Float
}

state cooldown: Integer = 0

function Run() {
  state.cooldown = math.max(0, state.cooldown - 1)

  if (metric > THRESHOLD && state.cooldown == 0) {
    emit(alerts, level = "high", value = metric)
    state.cooldown = 20
  }
}
```

JavaScript-flavored surface: newlines end statements (no `;`), C-style blocks and logical
operators (`&&`, `||`, `!`), `let` for function-local bindings. Core scalars are `Integer`,
`Float`, `Bool`, and `String`; `Time` and `Duration` come from the `time` prelude module.
There is no `Null`, no collection type, and no user-defined function — `Init()` and `Run()`
are the only two a program may declare, and only `Run` is required.

`state` above is **not** core: it is a declaration kind the host registered. So are
`setting` and `indicator` in the reference host under [`examples/signal`](examples/signal).

## Embedding

```go
builder := tascript.NewBuilder()

reg := builder.Registry()
if err := stdlib.Register(reg); err != nil {
    return err // the host registry already claims math, time, Time, or Duration
}
// ... register host types, operators, calls, and declaration kinds

program, diags, err := builder.Compile(src)
if err != nil {
    return err // host/API misuse, not a script problem
}
if len(diags) > 0 {
    return reject(diags) // the script is rejected
}

program.BindInput("candles", series)
program.BindOutput("signal", router)

// a host fill before Init wins over the script's own initializer
if slot, ok := program.Slot("setting", "slowPeriod"); ok {
    slot.Set(registry.Integer(7))
}

if err := program.Init(); err != nil {
    return err
}

for range activations {
    if err := program.Run(); err != nil {
        return err
    }
}
```

A `Builder` compiles one script; build another to compile again. Compiling seals the
registry, so registration after that point returns `registry.ErrSealed`.

### Diagnostics

Check-time problems come back from `Compile` as a `[]diag.Diagnostic` — a non-empty slice
means the program is rejected. Each carries a phase, a stable category code, a source
location, and a message:

```
error[TYPE_MISMATCH] 2:19: expected Float, found String
error[DIVISION_BY_ZERO] 3:9: integer division by zero
```

The parser collects up to 100 diagnostics before aborting, resynchronizing at statement
boundaries. Runtime traps surface as an `error` from `Init()` or `Run()` and borrow their
category code from the runtime error kind.

## Concurrency

Not safe for concurrent use. A `Builder`, the `Executable` it produces, and that
executable's state are single-threaded by contract. To run scripts in parallel, give each
goroutine its own `Builder` and `Executable`.

## Packages

| Package                 | Role                                                                |
|-------------------------|---------------------------------------------------------------------|
| `tascript`              | Public façade — `Builder`, `Executable`, `Slot`                     |
| `lexer` / `token`       | Tokenisation                                                        |
| `parser` / `ast`        | Grammar and untyped syntax tree                                     |
| `resolver` / `resolved` | Name resolution, typing, slot layout                                |
| `evaluator`             | Tree-walking execution                                              |
| `registry`              | Host extension surface — types, operators, calls, declaration kinds |
| `stdlib`                | Prelude modules (`math`, `time`)                                    |
| `diag`                  | Diagnostics                                                         |

## Examples

- [`examples/signal`](examples/signal) — a full host: candle types, an indicator kind,
  `setting` / `state` declaration kinds, a bound input series, and an order-router sink.
- [`examples/main.go`](examples/main.go) — the minimal compile-and-run path.

## Development

```bash
make test      # go test ./... -race -count=1
make lint      # golangci-lint run ./...
make check     # lint-fix, lint, test
```

## License

MIT — see [LICENSE](LICENSE).
