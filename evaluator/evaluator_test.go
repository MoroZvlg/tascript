package evaluator_test

import (
	"testing"

	"github.com/MoroZvlg/tascript/evaluator"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/parser"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/token"
)

func TestEvaluator_EvalConstAndRunBlock(t *testing.T) {
	reg := registry.DefaultRegistry()
	reg.RegisterBinary(token.PLUS, registry.IntegerID, registry.IntegerID, registry.BinaryRule{
		EvalType: registry.IntegerID,
		EvalFn: func(left, right registry.Value) registry.Value {
			return registry.Integer(int(left.(registry.Integer)) + int(right.(registry.Integer)))
		},
	})

	src := `
const a = 1 + 2
function Run() {
let b = a + 3
b
}
`
	p := parser.New(lexer.New(src))
	prog := p.Parse()
	if len(p.Diagnostics()) > 0 {
		t.Fatalf("parser diagnostics: %v", p.Diagnostics())
	}

	got, _ := evaluator.New(prog, reg).EvalRun()
	if got != registry.Integer(6) {
		t.Fatalf("got %v, want 6", got)
	}
}

func TestEvaluator_StdOps(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want registry.Value
	}{
		{"string concat", `"foo" + "bar"`, registry.String("foobar")},
		{"int negation", `-5`, registry.Integer(-5)},
		{"float negation", `-2.5`, registry.Float(-2.5)},
		{"bool not", `!true`, registry.Bool(false)},
		{"double negation", `--5`, registry.Integer(5)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := "function Run() {\n" + tt.expr + "\n}"
			p := parser.New(lexer.New(src))
			prog := p.Parse()
			if len(p.Diagnostics()) > 0 {
				t.Fatalf("parser diagnostics: %v", p.Diagnostics())
			}

			got, err := evaluator.New(prog, registry.DefaultRegistry()).EvalRun()
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
