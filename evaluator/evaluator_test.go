package evaluator_test

import (
	"math"
	"testing"

	"github.com/MoroZvlg/tascript/evaluator"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/parser"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/resolver"
)

func evalSrc(t *testing.T, src string) registry.Value {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.Parse()
	if len(p.Diagnostics()) > 0 {
		t.Fatalf("parser diagnostics: %v", p.Diagnostics())
	}
	reg := registry.DefaultRegistry()
	resolv := resolver.New(prog, reg)
	resolvedProg := resolv.Resolve()
	if len(resolv.Diagnostics()) > 0 {
		t.Fatalf("resolver diagnostics: %v", resolv.Diagnostics())
	}

	got, err := evaluator.New(resolvedProg, reg).EvalRun()
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	return got
}

func evalRunBody(t *testing.T, body string) registry.Value {
	t.Helper()
	return evalSrc(t, "function Run() {\n"+body+"\n}")
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
		{"member access", `math.PI`, registry.Float(math.Pi)},
		{"method call", `math.sqrt(9)`, registry.Float(3)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalRunBody(t, tt.expr)
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluator_Arithmetic(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want registry.Value
	}{
		{"int addition", `1 + 2`, registry.Integer(3)},
		{"int subtraction", `1 - 2`, registry.Integer(-1)},
		{"int multiplication", `6 * 7`, registry.Integer(42)},
		{"int modulo", `7 % 3`, registry.Integer(1)},
		{"int division is float", `1 / 2`, registry.Float(0.5)},
		{"mixed int float", `1 + 2.5`, registry.Float(3.5)},
		{"mixed float int", `2.5 * 2`, registry.Float(5)},
		{"float modulo", `7.5 % 2`, registry.Float(1.5)},
		{"precedence", `2 + 3 * 4`, registry.Integer(14)},
		{"grouping", `(2 + 3) * 4`, registry.Integer(20)},
		{"negation in expression", `-2 * -3`, registry.Integer(6)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalRunBody(t, tt.expr)
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluator_Comparisons(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want registry.Value
	}{
		{"int less", `1 < 2`, registry.Bool(true)},
		{"int greater false", `1 > 2`, registry.Bool(false)},
		{"int equal", `2 == 2`, registry.Bool(true)},
		{"int not equal", `2 != 2`, registry.Bool(false)},
		{"lteq boundary", `2 <= 2`, registry.Bool(true)},
		{"gteq boundary", `3 >= 4`, registry.Bool(false)},
		// int/float mix must compare numerically, not by dynamic type
		{"int equals float", `1 == 1.0`, registry.Bool(true)},
		{"float less than int", `1.5 < 2`, registry.Bool(true)},
		{"string equality", `"a" == "a"`, registry.Bool(true)},
		{"string inequality", `"a" != "b"`, registry.Bool(true)},
		{"bool equality", `true == true`, registry.Bool(true)},
		{"comparison chain via bool", `true == (2 > 1)`, registry.Bool(true)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalRunBody(t, tt.expr)
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluator_Statements(t *testing.T) {
	tests := []struct {
		name string
		body string
		want registry.Value
	}{
		{"let then read", "let x = 2\nx * 3", registry.Integer(6)},
		{"assign overwrites", "let x = 1\nx = x + 41\nx", registry.Integer(42)},
		{"two lets", "let a = 2\nlet b = 3\na * b", registry.Integer(6)},
		{"let from expression", "let x = math.sqrt(4) + 1\nx", registry.Float(3)},
		{"let value is coerced", "let x = 1\nx + 0.5", registry.Float(1.5)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalRunBody(t, tt.body)
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluator_Consts(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want registry.Value
	}{
		{
			"const read in run",
			"const K = 2\nfunction Run() {\nK + 1\n}",
			registry.Integer(3),
		},
		{
			"const from const",
			"const A = 2\nconst B = A * 3\nfunction Run() {\nB\n}",
			registry.Integer(6),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalSrc(t, tt.src)
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
