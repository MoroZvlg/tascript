// Command tascript-wasm is the WASI build of the tascript interpreter.
//
// Build with:
//
//	GOOS=wasip1 GOARCH=wasm go build -o tascript.wasm ./cmd/tascript-wasm
//
// This binary is meant to be run inside a WASM runtime (cmd/sandbox) — not
// natively. From the module's point of view it's an ordinary Go program that
// reads the script from stdin and (optionally) reads candles from ./data.csv;
// the host process decides what stdin contains and which directory ./data.csv
// resolves to via WASI preopens.
//
// No interpreter logic lives here — this is just glue.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/MoroZvlg/tascript/evaluator"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/object"
	"github.com/MoroZvlg/tascript/parser"
	"github.com/MoroZvlg/tascript/repl"
)

func main() {
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read stdin: %s\n", err)
		os.Exit(1)
	}

	env := object.NewEnvironment()
	evaluator.RegisterBuiltins(env)

	// Candles are optional: if the host hasn't mounted a directory containing
	// data.csv, we still let the script run — it just won't have `candles`
	// bound. This keeps simple scripts (e.g. "1 + 1") working inside the
	// sandbox without requiring a host-side filesystem mount.
	if cs, err := repl.LoadCandlesCSV(repl.DefaultCandlesPath); err == nil {
		env.Set("candles", cs)
	}

	l := lexer.New(string(src))
	p := parser.New(l)
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, e)
		}
		os.Exit(1)
	}

	result := evaluator.Eval(context.Background(), prog, env)
	if object.IsError(result) {
		fmt.Fprintln(os.Stderr, result.Inspect())
		os.Exit(1)
	}
}
