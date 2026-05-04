package repl

import (
	"bufio"
	"fmt"
	"io"

	"github.com/MoroZvlg/tascript/evaluator"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/object"
	"github.com/MoroZvlg/tascript/parser"
)

const Prompt = ">> "

func Start(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	env := object.NewEnvironment()

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
		result := evaluator.Eval(program, env)
		fmt.Fprintln(out, result.Inspect())
	}
}
