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
		// TODO: we need some builtin func with at least 2 args to implement this test
		//{
		//	"arg missing",
		//	`const FOO = math.sqrt^()`,
		//	func(ps []token.Pos) []diag.Diagnostic {
		//		return []diag.Diagnostic{
		//			addMissingArg(token.Token{Type: token.LPAREN, Pos: ps[0], Literal: "("}, "number"),
		//		}
		//	},
		//},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runDiagCases(t, tt.input+runSuffix, tt.buildDiags)
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
			"bad value reports only its own error",
			"function Run() {\nlet x = 1\nx = ^z\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addUndefinedIdent(token.Token{Type: token.IDENT, Pos: ps[0], Literal: "z"}),
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
	src, pos := extractErrorsPos(input)
	expected := buildDiags(pos)

	l := lexer.New(src)
	p := parser.New(l)
	prog := p.Parse()
	reg := registry.DefaultRegistry()
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
