package resolver_test

import (
	"testing"

	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/parser"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/resolver"
	"github.com/MoroZvlg/tascript/token"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

const runSuffix = "\nfunction Run() {\n1\n}"

func TestResolver_ResolveConstSimple(t *testing.T) {
	tests := []struct {
		input  string
		output string
	}{
		{"const FOO = 5", "const FOO:Integer = 5"},
		{"const FOO = 5.3", "const FOO:Float = 5.3"},
		{`const FOO = "bar"`, `const FOO:String = "bar"`},
		{`const FOO = "a\"b"`, `const FOO:String = "a\"b"`},
		{"const FOO = false", "const FOO:Bool = false"},
		{"const FOO = -(5.3 + 3)", "const FOO:Float = (prefix:Float, -, (infix:Float, +, 5.3, 3))"},
		{"const FOO = 7 % 3 * 2", "const FOO:Integer = (infix:Integer, *, (infix:Integer, %, 7, 3), 2)"},
		{"const FOO = true == 5 > 2", "const FOO:Bool = (infix:Bool, ==, true, (infix:Bool, >, 5, 2))"},
		{"const FOO = math.PI", "const FOO:Float = (member_access:Float, math:math, PI)"},
		{"const FOO = math.sqrt(3*3)", "const FOO:Float = (method_call:Float, math:math, sqrt, number=(coerce:Float, (infix:Integer, *, 3, 3)))"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input + runSuffix)
			p := parser.New(l)
			prog := p.Parse()
			if len(p.Diagnostics()) > 0 {
				for _, d := range p.Diagnostics() {
					t.Log(d)
				}
				t.Fatalf("expected 0 errors, got %d\n", len(p.Diagnostics()))
			}

			reg := registry.DefaultRegistry()
			resolv := resolver.New(prog, reg)

			resolvedProg := resolv.Resolve()

			if len(resolv.Diagnostics()) > 0 {
				for _, d := range resolv.Diagnostics() {
					t.Log(d)
				}
				t.Fatalf("expected 0 errors, got %d\n", len(resolv.Diagnostics()))
			}
			dumpedRes := dumpConst(t, resolvedProg.Consts[0])
			if tt.output != dumpedRes {
				t.Errorf("expected %s, got %s", tt.output, dumpedRes)
			}
		})
	}
}

func TestResolver_ResolveInputsOutputs(t *testing.T) {
	tests := []struct {
		input  string
		output string
	}{
		{"input threshold: Float", "input threshold:Float"},
		{"input btc: {price: Float, vol: Integer}", "input btc:input.btc"},
		{"output alert: String", "output alert:String"},
		{"output sig: {dir: String, price: Float}", "output sig:output.sig"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input + runSuffix)
			p := parser.New(l)
			prog := p.Parse()
			if len(p.Diagnostics()) > 0 {
				for _, d := range p.Diagnostics() {
					t.Log(d)
				}
				t.Fatalf("expected 0 errors, got %d\n", len(p.Diagnostics()))
			}

			reg := registry.DefaultRegistry()
			resolv := resolver.New(prog, reg)
			resolvedProg := resolv.Resolve()

			if len(resolv.Diagnostics()) > 0 {
				for _, d := range resolv.Diagnostics() {
					t.Log(d)
				}
				t.Fatalf("expected 0 errors, got %d\n", len(resolv.Diagnostics()))
			}

			var dumpedRes string
			switch {
			case len(resolvedProg.Inputs) == 1:
				dumpedRes = resolvedProg.Inputs[0].String()
			case len(resolvedProg.Outputs) == 1:
				dumpedRes = resolvedProg.Outputs[0].String()
			default:
				t.Fatal("expected exactly one input or output decl")
			}
			if tt.output != dumpedRes {
				t.Errorf("expected %s, got %s", tt.output, dumpedRes)
			}
		})
	}
}

func TestResolver_ResolveInputOutputErrors(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		buildDiags func([]token.Pos) []diag.Diagnostic
	}{
		{
			"struct input field access resolves",
			"input btc: {price: Float}\nfunction Run() {\nlet x = btc.price\nx + 1.5\n}",
			func(ps []token.Pos) []diag.Diagnostic { return nil },
		},
		{
			"undefined input type",
			"input btc: ^Foo" + runSuffix,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addUndefinedType(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "Foo"}),
				}
			},
		},
		{
			"undefined field type",
			"input btc: {price: ^Foo}" + runSuffix,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addUndefinedType(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "Foo"}),
				}
			},
		},
		{
			"duplicate input",
			"input btc: Float\n^input ^btc: Integer" + runSuffix,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addDuplicateDecl(
						token.Token{Type: token.INPUT, Pos: ps[0], Literal: "input"},
						token.Token{Type: token.IDENT, Pos: ps[1], Literal: "btc"},
					),
				}
			},
		},
		{
			"output colliding with input",
			"input foo: Float\n^output ^foo: String" + runSuffix,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addDuplicateDecl(
						token.Token{Type: token.OUTPUT, Pos: ps[0], Literal: "output"},
						token.Token{Type: token.IDENT, Pos: ps[1], Literal: "foo"},
					),
				}
			},
		},
		{
			"input colliding with const",
			"const FOO = 1\n^input ^FOO: Float" + runSuffix,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addDuplicateDecl(
						token.Token{Type: token.INPUT, Pos: ps[0], Literal: "input"},
						token.Token{Type: token.IDENT, Pos: ps[1], Literal: "FOO"},
					),
				}
			},
		},
		{
			"read output",
			"output alert: String\nfunction Run() {\nlet x = ^alert\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addOutputNotReadable(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "alert"}),
				}
			},
		},
		{
			"output in expression reports only the read error",
			"output alert: String\nfunction Run() {\n^alert + \"x\"\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addOutputNotReadable(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "alert"}),
				}
			},
		},
		{
			"duplicate field in inline type",
			"input btc: ^{price: Float, ^price: Integer}" + runSuffix,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addDuplicateDecl(
						token.Token{Type: token.LBRACE, Pos: ps[0], Literal: "{"},
						token.Token{Type: token.IDENT, Pos: ps[1], Literal: "price"},
					),
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runDiagCases(t, tt.input, tt.buildDiags)
		})
	}
}

func TestResolver_ResolveIfStmt(t *testing.T) {
	src := `function Run() {
let x = 1
if (x > 0) {
let y = x + 1
} else if (x == 0) {
let y = 2
} else {
let y = 3
}
x
}`
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.Parse()
	if len(p.Diagnostics()) > 0 {
		for _, d := range p.Diagnostics() {
			t.Log(d)
		}
		t.Fatalf("expected 0 parser errors, got %d", len(p.Diagnostics()))
	}

	reg := registry.DefaultRegistry()
	resolv := resolver.New(prog, reg)
	resolvedProg := resolv.Resolve()

	if len(resolv.Diagnostics()) > 0 {
		for _, d := range resolv.Diagnostics() {
			t.Log(d)
		}
		t.Fatalf("expected 0 resolver errors, got %d", len(resolv.Diagnostics()))
	}

	expected := "{" +
		"let x:Integer = 1; " +
		"if (infix:Bool, >, x:Integer, 0) {let y:Integer = (infix:Integer, +, x:Integer, 1)}" +
		" else if (infix:Bool, ==, x:Integer, 0) {let y:Integer = 2}" +
		" else {let y:Integer = 3}; " +
		"x:Integer" +
		"}"
	dumpedRes := dumpStmt(t, resolvedProg.RunFn.Body)
	if expected != dumpedRes {
		t.Errorf("expected %s, got %s", expected, dumpedRes)
	}
}

func TestResolver_ResolveConstErrors(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		buildDiags func([]token.Pos) []diag.Diagnostic
	}{
		{
			"duplicate declaration",
			"const FOO = 3\n ^const ^FOO = 4",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addDuplicateDecl(
						token.Token{Type: token.CONST, Pos: ps[0], Literal: "const"},
						token.Token{Type: token.IDENT, Pos: ps[1], Literal: "FOO"},
					),
				}
			},
		},
		{
			"duplicate does not hide later const errors",
			"const FOO = 3\n^const ^FOO = 4\nconst C = ^bar",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addDuplicateDecl(
						token.Token{Type: token.CONST, Pos: ps[0], Literal: "const"},
						token.Token{Type: token.IDENT, Pos: ps[1], Literal: "FOO"},
					),
					addUndefinedIdent(token.Token{Type: token.IDENT, Pos: ps[2], Literal: "bar"}),
				}
			},
		},
		{
			"int + string",
			`const FOO = 1 ^+ "foo"`,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addInvalidBinaryOp(
						token.Token{Type: token.PLUS, Pos: ps[0], Literal: "+"},
						registry.IntegerID, registry.StringID,
					),
				}
			},
		},
		{
			"not on int",
			`const FOO = ^!1`,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addInvalidUnaryOp(
						token.Token{Type: token.BANG, Pos: ps[0], Literal: "!"},
						registry.IntegerID,
					),
				}
			},
		},
		{
			"undefined ident",
			`const FOO = ^bar`,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addUndefinedIdent(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "bar"}),
				}
			},
		},
		{
			"undefined attribute",
			`const FOO = math.^FOO`,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addUndefinedAttribute(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "FOO"}),
				}
			},
		},
		{
			"undefined method call",
			`const FOO = math.^foo()`,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addUndefinedMethod(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "foo"}),
				}
			},
		},
		{
			"args number missmatch",
			`const FOO = math.sqrt^(1, 2)`,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addArgsNumberMismatch(token.Token{Type: token.LPAREN, Pos: ps[0], Literal: "("}, 1, 2),
				}
			},
		},
		{
			"args number missmatch",
			`const FOO = math.sqrt^("foo")`,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addTypeMissmatch(token.Token{Type: token.LPAREN, Pos: ps[0], Literal: "("}, registry.FloatID, registry.StringID),
				}
			},
		},
		{
			"arg missing",
			`const FOO = test.fn^(1, foo=2)`,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addMissingArg(token.Token{Type: token.LPAREN, Pos: ps[0], Literal: "("}, "right"),
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var setupRegistry func(*registry.Registry)
			if tt.name == "arg missing" {
				setupRegistry = registerResolverTestModule
			}
			runDiagCasesWithRegistry(t, tt.input+runSuffix, tt.buildDiags, setupRegistry)
		})
	}
}

func TestResolver_ResolveRunErrors(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		buildDiags func([]token.Pos) []diag.Diagnostic
	}{
		{
			"assign to undefined variable",
			"function Run() {\n^y = 3\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addUndefinedVar(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "y"}),
				}
			},
		},
		{
			"assign type missmatch",
			"function Run() {\nlet x = 1\nx ^= \"foo\"\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addTypeMissmatch(token.Token{Type: token.ASSIGN, Pos: ps[0], Literal: "="}, registry.IntegerID, registry.StringID),
				}
			},
		},
		{
			"assign to non-ident target",
			"function Run() {\nmath.PI ^= 3\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addInvalidAssignTarget(token.Token{Type: token.ASSIGN, Pos: ps[0], Literal: "="}),
				}
			},
		},
		{
			"reassign let is allowed",
			"function Run() {\nlet x = 1\nx = 2\n}",
			func(ps []token.Pos) []diag.Diagnostic { return nil },
		},
		{
			"assign to const",
			"const K = 1\nfunction Run() {\n^K = 2\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addNotAssignable(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "K"}, "const"),
				}
			},
		},
		{
			"assign to input",
			"input p: Float\nfunction Run() {\n^p = 2.0\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addNotAssignable(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "p"}, "input"),
				}
			},
		},
		{
			"assign to output",
			"output alert: String\nfunction Run() {\n^alert = \"x\"\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addNotAssignable(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "alert"}, "output"),
				}
			},
		},
		{
			"assign to module",
			"function Run() {\n^math = 3\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addNotAssignable(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "math"}, "module"),
				}
			},
		},
		{
			"bad value reports only its own error",
			"function Run() {\nlet x = 1\nx = ^z\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addUndefinedIdent(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "z"}),
				}
			},
		},
		{
			"if condition must be bool",
			"function Run() {\n^if (1) {}\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addTypeMissmatch(token.Token{Type: token.IF, Pos: ps[0], Literal: "if"}, registry.BoolID, registry.IntegerID),
				}
			},
		},
		{
			"bad if condition reports only its own error",
			"function Run() {\nif (^missing) {}\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addUndefinedIdent(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "missing"}),
				}
			},
		},
		{
			"if branch scope does not leak",
			"function Run() {\nif (true) {\nlet x = 1\n}\n^x\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addUndefinedIdent(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "x"}),
				}
			},
		},
		{
			"else branch scope does not leak",
			"function Run() {\nif (true) {\n} else {\nlet x = 1\n}\n^x\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addUndefinedIdent(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "x"}),
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runDiagCases(t, tt.input, tt.buildDiags)
		})
	}
}

func TestResolver_ResolveEmit(t *testing.T) {
	src := `output alert: String
output sig: {dir: String, price: Float}
output level: Float
function Run() {
emit(alert, "hi")
emit(sig, dir="up", price=1.2)
emit(sig, "down", 0.5)
emit(level, 1)
}`
	expected := []string{
		`emit(alert:String, value="hi")`,
		`emit(sig:output.sig, dir="up",price=1.2)`,
		`emit(sig:output.sig, dir="down",price=0.5)`,
		`emit(level:Float, value=(coerce:Float, 1))`,
	}

	l := lexer.New(src)
	p := parser.New(l)
	prog := p.Parse()
	if len(p.Diagnostics()) > 0 {
		for _, d := range p.Diagnostics() {
			t.Log(d)
		}
		t.Fatalf("expected 0 parser errors, got %d", len(p.Diagnostics()))
	}

	reg := registry.DefaultRegistry()
	resolv := resolver.New(prog, reg)
	resolvedProg := resolv.Resolve()

	if len(resolv.Diagnostics()) > 0 {
		for _, d := range resolv.Diagnostics() {
			t.Log(d)
		}
		t.Fatalf("expected 0 resolver errors, got %d", len(resolv.Diagnostics()))
	}

	stmts := resolvedProg.RunFn.Body.Stmts
	if len(stmts) != len(expected) {
		t.Fatalf("expected %d statements, got %d", len(expected), len(stmts))
	}
	for i, want := range expected {
		if got := dumpStmt(t, stmts[i]); got != want {
			t.Errorf("stmt[%d]: expected %s, got %s", i, want, got)
		}
	}
}

func TestResolver_ResolveEmitErrors(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		buildDiags func([]token.Pos) []diag.Diagnostic
	}{
		{
			"emit to const",
			"const K = 1\nfunction Run() {\nemit(^K, 1)\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addInvalidEmitTarget(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "K"}),
				}
			},
		},
		{
			"emit to input",
			"input p: Float\nfunction Run() {\nemit(^p, 1.0)\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addInvalidEmitTarget(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "p"}),
				}
			},
		},
		{
			"emit to unknown identifier",
			"function Run() {\nemit(^foo, 1)\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addUndefinedIdent(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "foo"}),
				}
			},
		},
		{
			"emit without args",
			"function Run() {\nemit^()\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addInvalidEmitTarget(token.Token{Type: token.LPAREN, Pos: ps[0], Literal: "("}),
				}
			},
		},
		{
			"emit to non-ident target",
			"function Run() {\nemit^(1.5)\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addInvalidEmitTarget(token.Token{Type: token.LPAREN, Pos: ps[0], Literal: "("}),
				}
			},
		},
		{
			"emit value type missmatch",
			"output alert: String\nfunction Run() {\nemit^(alert, 3)\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addTypeMissmatch(token.Token{Type: token.LPAREN, Pos: ps[0], Literal: "("}, registry.StringID, registry.IntegerID),
				}
			},
		},
		{
			"emit missing field",
			"output sig: {dir: String, price: Float}\nfunction Run() {\nemit^(sig, dir=\"up\")\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addArgsNumberMismatch(token.Token{Type: token.LPAREN, Pos: ps[0], Literal: "("}, 2, 1),
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runDiagCases(t, tt.input, tt.buildDiags)
		})
	}
}

func TestResolver_ResolveStateSimple(t *testing.T) {
	type field struct {
		name    string
		t       registry.TypeID
		hasInit bool
	}
	tests := []struct {
		name   string
		input  string
		fields []field
	}{
		{
			"const initializer",
			"state cooldown: Integer = 0",
			[]field{{"cooldown", registry.IntegerID, true}},
		},
		{
			// Integer literal coerces into the declared Float
			"coerced initializer",
			"state threshold: Float = 1",
			[]field{{"threshold", registry.FloatID, true}},
		},
		{
			"initializer from const and module call",
			"const N = 3\nstate x: Float = math.sqrt(9.0) * N",
			[]field{{"x", registry.FloatID, true}},
		},
		{
			"no initializer, seeded in Init",
			"state s: String\nfunction Init() {\nstate.s = \"x\"\n}",
			[]field{{"s", registry.StringID, false}},
		},
		{
			"decl order preserved",
			"state a: Integer = 0\nstate b: Float = 2.5",
			[]field{{"a", registry.IntegerID, true}, {"b", registry.FloatID, true}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input + runSuffix)
			p := parser.New(l)
			prog := p.Parse()
			if len(p.Diagnostics()) > 0 {
				t.Fatalf("parser diagnostics: %v", p.Diagnostics())
			}

			reg := registry.DefaultRegistry()
			resolv := resolver.New(prog, reg)
			resolvedProg := resolv.Resolve()
			if len(resolv.Diagnostics()) > 0 {
				for _, d := range resolv.Diagnostics() {
					t.Log(d)
				}
				t.Fatalf("expected 0 errors, got %d\n", len(resolv.Diagnostics()))
			}

			if resolvedProg.State == nil {
				t.Fatalf("expected resolved state, got nil")
			}
			if len(resolvedProg.State.Fields) != len(tt.fields) {
				t.Fatalf("expected %d state fields, got %d", len(tt.fields), len(resolvedProg.State.Fields))
			}
			for i, want := range tt.fields {
				got := resolvedProg.State.Fields[i]
				if got.Name != want.name {
					t.Errorf("field %d: expected name %s, got %s", i, want.name, got.Name)
				}
				if got.T != want.t {
					t.Errorf("field %d: expected type %s, got %s", i, want.t, got.T)
				}
				if (got.InitValue != nil) != want.hasInit {
					t.Errorf("field %d: expected hasInit=%t, got %t", i, want.hasInit, got.InitValue != nil)
				}
			}
		})
	}
}

func TestResolver_ResolveStateErrors(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		buildDiags func([]token.Pos) []diag.Diagnostic
	}{
		{
			"undeclared field read",
			"function Run() {\nlet x = state.^foo\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addStateUndeclared(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "foo"}),
				}
			},
		},
		{
			"undeclared field write",
			"function Run() {\nstate.^foo = 1\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addStateUndeclared(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "foo"}),
				}
			},
		},
		{
			"initializer type mismatch",
			"state ^x: Integer = true" + runSuffix,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addTypeMissmatch(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "x"}, registry.IntegerID, registry.BoolID),
				}
			},
		},
		{
			"unknown type",
			"state x: ^Foo = 0" + runSuffix,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addUndefinedType(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "Foo"}),
				}
			},
		},
		{
			"initializer referencing input",
			"input btc: Integer\nstate x: Integer = ^btc" + runSuffix,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addUndefinedIdent(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "btc"}),
				}
			},
		},
		{
			"initializer referencing state field",
			"state a: Integer = 0\nstate b: Integer = state.^a" + runSuffix,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addStateUndeclared(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "a"}),
				}
			},
		},
		{
			"initializer referencing itself",
			"state a: Integer = state.^a + 1" + runSuffix,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addStateUndeclared(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "a"}),
				}
			},
		},
		{
			"duplicate field",
			"state a: Integer = 0\n^state ^a: Float = 1.0" + runSuffix,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addDuplicateDecl(
						token.Token{Type: token.STATE, Pos: ps[0], Literal: "state"},
						token.Token{Type: token.IDENT, Pos: ps[1], Literal: "a"},
					),
				}
			},
		},
		{
			"no initializer and no Init",
			"state ^x: Integer" + runSuffix,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addStateUninitialized(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "x"}),
				}
			},
		},
		{
			"no initializer and Init does not assign it",
			"state ^x: Integer\nstate y: Integer\nfunction Init() {\nstate.y = 1\n}" + runSuffix,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addStateUninitialized(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "x"}),
				}
			},
		},
		{
			"seeded only under if",
			"state ^x: Integer\nfunction Init() {\nif (true) {\nstate.x = 1\n}\n}" + runSuffix,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addStateUninitialized(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "x"}),
				}
			},
		},
		{
			"seeded at top level then conditionally overwritten",
			"state x: Integer\nfunction Init() {\nstate.x = 1\nif (true) {\nstate.x = 2\n}\n}" + runSuffix,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{}
			},
		},
		{
			"lone if first, top-level seed after",
			"state x: Integer\nfunction Init() {\nif (true) {\nstate.x = 2\n}\nstate.x = 1\n}" + runSuffix,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{}
			},
		},
		{
			"seeded in both if/else branches",
			"state x: Integer\nfunction Init() {\nif (true) {\nstate.x = 1\n} else {\nstate.x = 2\n}\n}" + runSuffix,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{}
			},
		},
		{
			"seeded in only one if/else branch",
			"state ^x: Integer\nfunction Init() {\nif (true) {\nstate.x = 1\n} else {\nlet y = 2\n}\n}" + runSuffix,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addStateUninitialized(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "x"}),
				}
			},
		},
		{
			"seeded through else-if chain",
			"state x: Integer\nfunction Init() {\nif (true) {\nstate.x = 1\n} else if (false) {\nstate.x = 2\n} else {\nstate.x = 3\n}\n}" + runSuffix,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{}
			},
		},
		{
			"assign type mismatch in Run",
			"state x: Integer = 0\nfunction Run() {\nstate.x ^= true\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addTypeMissmatch(token.Token{Type: token.ASSIGN, Pos: ps[0], Literal: "="}, registry.IntegerID, registry.BoolID),
				}
			},
		},
		{
			"assign coercion in Run",
			"state f: Float = 0.0\nfunction Run() {\nstate.f = 1\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{}
			},
		},
		{
			"state field read in expression",
			"state c: Integer = 5\nfunction Run() {\nlet x = state.c + 1\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{}
			},
		},
		{
			"bare state as value",
			"state c: Integer = 5\nfunction Run() {\nlet x = ^state\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addUndefinedIdent(token.Token{Type: token.STATE, Pos: ps[0], Literal: "state"}),
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runDiagCases(t, tt.input, tt.buildDiags)
		})
	}
}

// runDiagCases parses input (with ^ markers stripped to positions), then asserts the parser's
// diagnostics match what buildDiags(positions) returns. When any diagnostic is expected, the
// program must also be marked invalid. Empty vs nil diagnostic slices compare equal.
func runDiagCases(t *testing.T, input string, buildDiags func([]token.Pos) []diag.Diagnostic) {
	t.Helper()
	runDiagCasesWithRegistry(t, input, buildDiags, nil)
}

func runDiagCasesWithRegistry(t *testing.T, input string, buildDiags func([]token.Pos) []diag.Diagnostic, setupRegistry func(*registry.Registry)) {
	t.Helper()
	src, pos := extractErrorsPos(input)
	expected := buildDiags(pos)

	l := lexer.New(src)
	p := parser.New(l)
	prog := p.Parse()
	reg := registry.DefaultRegistry()
	if setupRegistry != nil {
		setupRegistry(reg)
	}
	resolv := resolver.New(prog, reg)
	_ = resolv.Resolve()

	got := resolv.Diagnostics()
	if len(got) != len(expected) {
		for i, d := range got {
			t.Logf("got[%d] %+v", i, d)
		}
		t.Fatalf("diag count: got %d, want %d", len(got), len(expected))
	}

	if diff := cmp.Diff(expected, got, cmpopts.EquateEmpty(), cmpopts.EquateComparable(registry.TypeID{})); diff != "" {
		t.Errorf("diagnostics mismatch (-want +got):\n%s", diff)
	}
}

func registerResolverTestModule(reg *registry.Registry) {
	testModule, _ := reg.RegisterModule("test")
	reg.RegisterCall(testModule.TypeID(), "fn", registry.CallRule{
		Args: []registry.ParamRule{
			{Type: registry.FloatID, Name: "left"},
			{Type: registry.FloatID, Name: "right"},
		},
		EvalType: registry.FloatID,
		EvalFn: func(registry.Value, map[string]registry.Value) registry.Value {
			return registry.Float(0)
		},
	})
}

func extractErrorsPos(input string) (string, []token.Pos) {
	var out []byte
	var pos []token.Pos
	line, col := 1, 1
	for i := range len(input) {
		switch char := input[i]; char {
		case '\n':
			line++
			col = 1
			out = append(out, char)
		case '^':
			pos = append(pos, token.Pos{Line: line, Col: col})
		default:
			col++
			out = append(out, char)
		}
	}
	return string(out), pos
}

func addDuplicateDecl(kwToken, identToken token.Token) *diag.DuplicateDeclaration {
	return &diag.DuplicateDeclaration{Phase: diag.PhaseCheck, KeywordToken: kwToken, IdentToken: identToken}
}

func addInvalidBinaryOp(tok token.Token, left, right registry.TypeID) *diag.InvalidBinaryOperation {
	return &diag.InvalidBinaryOperation{Phase: diag.PhaseCheck, Token: tok, Left: left, Right: right}
}

func addInvalidUnaryOp(tok token.Token, right registry.TypeID) *diag.UnaryBinaryOperation {
	return &diag.UnaryBinaryOperation{Phase: diag.PhaseCheck, Token: tok, Right: right}
}

func addUndefinedIdent(tok token.Token) *diag.UndefinedIdent {
	return &diag.UndefinedIdent{Phase: diag.PhaseCheck, Token: tok}
}

func addUndefinedAttribute(member token.Token) *diag.UndefinedAttribute {
	return &diag.UndefinedAttribute{Phase: diag.PhaseCheck, Member: member}
}

func addUndefinedMethod(method token.Token) *diag.UndefinedMethod {
	return &diag.UndefinedMethod{Phase: diag.PhaseCheck, Method: method}
}

func addArgsNumberMismatch(tok token.Token, expected, got int) *diag.ArgsNumberMissmatch {
	return &diag.ArgsNumberMissmatch{Phase: diag.PhaseCheck, Token: tok, Expected: expected, Got: got}
}

func addMissingArg(tok token.Token, expected string) *diag.MissingArg {
	return &diag.MissingArg{Phase: diag.PhaseCheck, Token: tok, Expected: expected}
}

func addTypeMissmatch(tok token.Token, expected, got registry.TypeID) *diag.TypeMissmatch {
	return &diag.TypeMissmatch{Phase: diag.PhaseCheck, Token: tok, Expected: expected, Got: got}
}

func addUndefinedVar(tok token.Token) *diag.UndefinedVar {
	return &diag.UndefinedVar{Phase: diag.PhaseCheck, Token: tok}
}

func addUndefinedType(tok token.Token) *diag.UndefinedType {
	return &diag.UndefinedType{Phase: diag.PhaseCheck, Token: tok}
}

func addInvalidAssignTarget(tok token.Token) *diag.InvalidAssignTarget {
	return &diag.InvalidAssignTarget{Phase: diag.PhaseCheck, Token: tok}
}

func addNotAssignable(tok token.Token, kind string) *diag.NotAssignable {
	return &diag.NotAssignable{Phase: diag.PhaseCheck, Token: tok, Kind: kind}
}

func addInvalidEmitTarget(tok token.Token) *diag.InvalidEmitTarget {
	return &diag.InvalidEmitTarget{Phase: diag.PhaseCheck, Token: tok}
}

func addOutputNotReadable(tok token.Token) *diag.OutputNotReadable {
	return &diag.OutputNotReadable{Phase: diag.PhaseCheck, Token: tok}
}

func addStateUndeclared(tok token.Token) *diag.StateUndeclared {
	return &diag.StateUndeclared{Phase: diag.PhaseCheck, Token: tok}
}

func addStateUninitialized(tok token.Token) *diag.StateUninitialized {
	return &diag.StateUninitialized{Phase: diag.PhaseCheck, Token: tok}
}
