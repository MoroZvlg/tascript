package main

import (
	"fmt"
	"os"
	"os/user"

	"github.com/MoroZvlg/tascript/evaluator"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/object"
	"github.com/MoroZvlg/tascript/parser"
	"github.com/MoroZvlg/tascript/repl"
)

func main() {
	if len(os.Args) > 1 {
		runScript(os.Args[1])
		return
	}

	u, err := user.Current()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Hello %s! Welcome to tascript.\n", u.Username)
	fmt.Println("Type away:")
	repl.Start(os.Stdin, os.Stdout)
}

func runScript(path string) {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	env := object.NewEnvironment()
	cs, err := repl.LoadCandlesCSV(repl.DefaultCandlesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "candles load error: %s\n", err)
		os.Exit(1)
	}
	env.Set("candles", cs)
	evaluator.RegisterBuiltins(env)

	l := lexer.New(string(src))
	p := parser.New(l)
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, e)
		}
		os.Exit(1)
	}
	result := evaluator.Eval(prog, env)
	if object.IsError(result) {
		fmt.Fprintln(os.Stderr, result.Inspect())
		os.Exit(1)
	}
}
