package repl

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/MoroZvlg/tascript/evaluator"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/object"
	"github.com/MoroZvlg/tascript/parser"
)

const (
	Prompt             = ">> "
	DefaultCandlesPath = "./data.csv"
)

func Start(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	env := object.NewEnvironment()

	if cs, err := LoadCandlesCSV(DefaultCandlesPath); err == nil {
		env.Set("candles", cs)
		fmt.Fprintf(out, "loaded %d candles from %s\n", len(cs.Value), DefaultCandlesPath)
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(out, "candles load error: %s\n", err)
	}
	evaluator.RegisterBuiltins(env)

	for {
		fmt.Fprint(out, Prompt)
		if !scanner.Scan() {
			return // EOF (Ctrl-D) or error
		}
		line := scanner.Text()

		l := lexer.New(line)
		p := parser.New(l)
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			for _, err := range p.Errors() {
				fmt.Fprintln(out, err)
			}
			continue
		}
		result := evaluator.Eval(context.Background(), program, env)
		fmt.Fprintln(out, result.Inspect())
	}
}
