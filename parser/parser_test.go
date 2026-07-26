package parser_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/parser"
	"github.com/MoroZvlg/tascript/token"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

const runSuffix = "\nfunction Run() {\nlet a = 1\n}"

func TestParser_ParseConstSimple(t *testing.T) {
	tests := []struct {
		input  string
		output string
	}{
		{"const FOO = bar", "const FOO = bar"},
		{"const FOO = 5", "const FOO = 5"},
		{"const FOO = 5.3", "const FOO = 5.3"},
		{`const FOO = "bar"`, `const FOO = "bar"`},
		{`const FOO = "a\"b"`, `const FOO = "a\"b"`},
		{"const FOO = false", "const FOO = false"},
		{"const FOO = -5.3 + 3", "const FOO = (-5.3 + 3)"},
		{"const FOO = -(5.3 + 3)", "const FOO = -(5.3 + 3)"},
		{"const FOO = 5.3 + 3", "const FOO = (5.3 + 3)"},
		{"const FOO = 7 % 3", "const FOO = (7 % 3)"},
		{"const FOO = 7 % 3 * 2", "const FOO = ((7 % 3) * 2)"},
		{"const FOO = (5.3 + 3) * 2", "const FOO = ((5.3 + 3) * 2)"},
		{"const FOO = true == 5 > 2", "const FOO = (true == (5 > 2))"},
		{"const FOO = module.math.PI", "const FOO = module.math.PI"},
		{"\nconst FOO = 5\n", "const FOO = 5"},
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

			if tt.output != prog.Consts[0].String() {
				t.Errorf("expected %s, got %s", tt.output, prog.Consts[0].String())
			}

			if !prog.Valid {
				t.Errorf("expected prog be valid, got false")
			}
		})
	}
}

func TestParser_ParseConstErrors(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		buildDiags func([]token.Pos) []diag.Diagnostic
	}{
		{
			"missing ident",
			"const ^= 3",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.ASSIGN),
				}
			},
		},
		{
			"missing =",
			"const FOO ^3",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.ASSIGN, token.INTEGER),
				}
			},
		},
		{
			"missing expression",
			"const FOO = ^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.EOF),
				}
			},
		},
		{
			"missing ident and `=`",
			"const ^5",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.INTEGER),
				}
			},
		},
		{
			"missing ident and expression",
			"const ^= ^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.ASSIGN),
					exprExpectedErr(ps[1], token.EOF),
				}
			},
		},
		{
			"missing `=` and expression",
			"const FOO^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.ASSIGN, token.EOF),
				}
			},
		},
		{
			"missing `=` and bad expression",
			"const FOO ^3 + ^#",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.ASSIGN, token.INTEGER),
					exprExpectedErr(ps[1], token.ILLEGAL),
				}
			},
		},
		{
			"missing all",
			"const ^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.EOF),
				}
			},
		},
		{
			"[Infix] missing RHS",
			"const FOO = 3 + ^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.EOF),
				}
			},
		},
		{
			"[Infix] missing LHS",
			"const FOO = ^* 3",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.ASTERISK),
				}
			},
		},
		{
			"[Infix] missing both hands",
			"const FOO = 3 + ^*",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.ASTERISK),
				}
			},
		},
		{
			"[Group] missing operand and illegal token",
			"const FOO = (3 + ^) * ^#",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.RPAREN),
					exprExpectedErr(ps[1], token.ILLEGAL),
				}
			},
		},
		{
			"prefix expr error",
			"const FOO = -^#",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.ILLEGAL),
				}
			},
		},
		{
			"member access call. no ident",
			"const FOO = math.^3",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.INTEGER),
				}
			},
		},
		{
			"integer literal overflow",
			"const FOO = ^" + strings.Repeat("9", 100),
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					parseFailedErr(ps[0], token.INTEGER),
				}
			},
		},
		{
			"float literal overflow",
			"const FOO = ^" + strings.Repeat("9", 400) + ".0",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					parseFailedErr(ps[0], token.FLOAT),
				}
			},
		},
		{
			"empty group expression",
			"const FOO = (^)",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.RPAREN),
				}
			},
		},
		{
			"group expression. missing RPAREN",
			"const FOO = (3 + 1 ^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.RPAREN, token.EOF),
				}
			},
		},
		{
			"trailing token after expression",
			"const FOO = a ^b",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.NEWLINE, token.IDENT),
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

func TestParser_ParseConstRecovery(t *testing.T) {
	src := "const = bar\n const FOO = baz\nconst MATH = pi"
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.Parse()

	got := p.Diagnostics()
	if len(got) != 1 {
		for i, d := range got {
			t.Logf("got[%d] %+v", i, d)
		}
		t.Fatalf("expected 1 error, got %d", len(got))
	}

	if prog.Valid {
		t.Errorf("expected prog be invalid, got true")
	}

	if len(prog.Consts) != 2 {
		t.Fatalf("expected 2 constant parsed after recovery, got %d", len(prog.Consts))
	}
}

func TestParser_ParseInputSimple(t *testing.T) {
	tests := []struct {
		input  string
		output string
	}{
		{"input btc: CandleSeries", "input btc: CandleSeries"},
		{"input btc: String", "input btc: String"},
		{"input btc: {foo: Integer}", "input btc: {foo: Integer}"},
		{"input btc: {foo: Integer, bar: Float}", "input btc: {foo: Integer, bar: Float}"},
		{"\ninput btc: String\n", "input btc: String"},
		// newlines inside {} are whitespace, not separators: multi-line parses like one line
		{"input btc: {\n foo: Integer,\n bar: Float\n}", "input btc: {foo: Integer, bar: Float}"},
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

			if tt.output != prog.Inputs[0].String() {
				t.Errorf("expected %s, got %s", tt.output, prog.Inputs[0].String())
			}

			if !prog.Valid {
				t.Errorf("expected prog be valid, got false")
			}
		})
	}
}

func TestParser_Input(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		buildDiags func([]token.Pos) []diag.Diagnostic
	}{
		{
			"correct input with type",
			"input btc: CandleSeries",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{}
			},
		},
		{
			"correct input with custom type",
			"input btc: {foo: Integer}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{}
			},
		},
		{
			"correct expr as type",
			"input btc: ^(1 + 3)",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					expectedTypeOrCustomType(ps[0]),
				}
			},
		},
		{
			"empty custom type",
			"input btc: ^{}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					emptyCustomType(ps[0]),
				}
			},
		},
		{
			"missing colon",
			"input btc: {btc ^btc}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.COLON, token.IDENT),
				}
			},
		},
		{
			"missing ident",
			"input btc: {^:btc}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.COLON),
				}
			},
		},
		{
			"missing type",
			"input btc: {btc:^}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.RBRACE),
				}
			},
		},
		{
			// function Run added in test on the next line
			"missing right brace",
			"input btc: {btc: btc",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(token.Pos{Line: 2, Col: 1}, token.RBRACE, token.FUNCTION),
				}
			},
		},
		{
			// TODO: arguable error type... looks like missing `{` but parser see Ident and think it's a builtin Type usage
			// making logic more complicated looks like overengineering
			"missing left brace",
			"input btc: btc^: btc}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.NEWLINE, token.COLON),
				}
			},
		},
		{
			"missing type declaration",
			"input btc: ^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					expectedTypeOrCustomType(ps[0]),
				}
			},
		},
		{
			// keywords are not IDENT, so they can't be used as field names
			"keyword as field name",
			"input btc: {^const: Integer}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.CONST),
				}
			},
		},
		{
			// a newline does not separate fields; without a comma the next field is unexpected
			"missing comma between fields",
			"input btc: {foo: Integer ^bar: Float}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.RBRACE, token.IDENT),
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runDiagCases(t, tt.input+runSuffix, tt.buildDiags)
		})
	}
}

func TestParser_ParseInputRecovery(t *testing.T) {
	src := "input btc: {}\n input eth: String\ninput sol: {value: Integer}"
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.Parse()

	got := p.Diagnostics()
	if len(got) != 1 {
		for i, d := range got {
			t.Logf("got[%d] %+v", i, d)
		}
		t.Fatalf("expected 1 error, got %d", len(got))
	}

	if prog.Valid {
		t.Errorf("expected prog be invalid, got true")
	}

	if len(prog.Inputs) != 2 {
		t.Fatalf("expected 2 inputs parsed after recovery, got %d", len(prog.Inputs))
	}
}

func TestParser_ParseOutputSimple(t *testing.T) {
	tests := []struct {
		input  string
		output string
	}{
		{"output alert: String", "output alert: String"},
		{"output alert: Signal", "output alert: Signal"},
		{"output alert: {foo: Integer}", "output alert: {foo: Integer}"},
		{"output alert: {foo: Integer, bar: Float}", "output alert: {foo: Integer, bar: Float}"},
		{"\noutput alert: String\n", "output alert: String"},
		// newlines inside {} are whitespace, not separators: multi-line parses like one line
		{"output alert: {\n foo: Integer,\n bar: Float\n}", "output alert: {foo: Integer, bar: Float}"},
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

			if tt.output != prog.Outputs[0].String() {
				t.Errorf("expected %s, got %s", tt.output, prog.Outputs[0].String())
			}

			if !prog.Valid {
				t.Errorf("expected prog be valid, got false")
			}
		})
	}
}

func TestParser_Output(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		buildDiags func([]token.Pos) []diag.Diagnostic
	}{
		{
			"correct output with type",
			"output alert: String",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{}
			},
		},
		{
			"correct output with custom type",
			"output alert: {foo: Integer}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{}
			},
		},
		{
			"correct expr as type",
			"output alert: ^(1 + 3)",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					expectedTypeOrCustomType(ps[0]),
				}
			},
		},
		{
			"empty custom type",
			"output alert: ^{}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					emptyCustomType(ps[0]),
				}
			},
		},
		{
			"missing colon",
			"output alert: {alert ^alert}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.COLON, token.IDENT),
				}
			},
		},
		{
			"missing ident",
			"output alert: {^:alert}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.COLON),
				}
			},
		},
		{
			"missing type",
			"output alert: {alert:^}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.RBRACE),
				}
			},
		},
		{
			// function Run added in test on the next line
			"missing right brace",
			"output alert: {alert: alert",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(token.Pos{Line: 2, Col: 1}, token.RBRACE, token.FUNCTION),
				}
			},
		},
		{
			// TODO: arguable error type... looks like missing `{` but parser see Ident and think it's a builtin Type usage
			// making logic more complicated looks like overengineering
			"missing left brace",
			"output alert: alert^: alert}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.NEWLINE, token.COLON),
				}
			},
		},
		{
			"missing type declaration",
			"output alert: ^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					expectedTypeOrCustomType(ps[0]),
				}
			},
		},
		{
			"trailing comma in custom type",
			"output alert: {foo: Integer,}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{}
			},
		},
		{
			"trailing comma before a newline",
			"output alert: {\nfoo: Integer,\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{}
			},
		},
		{
			// keywords are not IDENT, so they can't be used as field names
			"keyword as field name",
			"output alert: {^const: Integer}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.CONST),
				}
			},
		},
		{
			// a newline does not separate fields; without a comma the next field is unexpected
			"missing comma between fields",
			"output alert: {foo: Integer ^bar: Float}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.RBRACE, token.IDENT),
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runDiagCases(t, tt.input+runSuffix, tt.buildDiags)
		})
	}
}

func TestParser_ParseOutputRecovery(t *testing.T) {
	src := "output a: {}\n output b: String\noutput c: {value: Integer}"
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.Parse()

	got := p.Diagnostics()
	if len(got) != 1 {
		for i, d := range got {
			t.Logf("got[%d] %+v", i, d)
		}
		t.Fatalf("expected 1 error, got %d", len(got))
	}

	if prog.Valid {
		t.Errorf("expected prog be invalid, got true")
	}

	if len(prog.Outputs) != 2 {
		t.Fatalf("expected 2 outputs parsed after recovery, got %d", len(prog.Outputs))
	}
}

func TestParser_ParseStateFieldSimple(t *testing.T) {
	tests := []struct {
		input  string
		output string
	}{
		{"state cooldown: Integer = 0", "state cooldown: Integer = 0"},
		{"state last_signal: Time", "state last_signal: Time"},
		{"state threshold: Float = 1.5 + 2.0", "state threshold: Float = (1.5 + 2)"},
		{"state cd: Duration = 30 * time.MINUTE", "state cd: Duration = (30 * time.MINUTE)"},
		{"state active: Bool = false", "state active: Bool = false"},
		{"\nstate cooldown: Integer = 0\n", "state cooldown: Integer = 0"},
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

			if len(prog.StateFields) != 1 {
				t.Fatalf("expected 1 state field, got %d", len(prog.StateFields))
			}

			if tt.output != prog.StateFields[0].String() {
				t.Errorf("expected %s, got %s", tt.output, prog.StateFields[0].String())
			}

			if !prog.Valid {
				t.Errorf("expected prog be valid, got false")
			}
		})
	}
}

func TestParser_StateField(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		buildDiags func([]token.Pos) []diag.Diagnostic
	}{
		{
			"correct with initializer",
			"state cooldown: Integer = 0",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{}
			},
		},
		{
			"correct without initializer",
			"state last_signal: Time",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{}
			},
		},
		{
			"missing ident",
			"state ^: Integer",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.COLON),
				}
			},
		},
		{
			"missing colon",
			"state cooldown ^Integer = 0",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.COLON, token.IDENT),
				}
			},
		},
		{
			"missing type",
			"state cooldown: ^= 0",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					expectedTypeOrCustomType(ps[0]),
				}
			},
		},
		{
			"expr as type",
			"state cooldown: ^(1 + 3)",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					expectedTypeOrCustomType(ps[0]),
				}
			},
		},
		{
			// schema types are not supported in state decls (scalar entries only)
			"schema as type",
			"state pair: ^{a: Float}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					expectedTypeOrCustomType(ps[0]),
				}
			},
		},
		{
			// runSuffix puts a NEWLINE right after the dangling `=`
			"missing initializer expression",
			"state cooldown: Integer = ^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.NEWLINE),
				}
			},
		},
		{
			"keyword as name",
			"state ^const: Integer = 0",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.CONST),
				}
			},
		},
		{
			// assignment to a state field is decl-position syntax error at the top level
			"top-level state assignment",
			"state^.cooldown = 0",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.DOT),
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runDiagCases(t, tt.input+runSuffix, tt.buildDiags)
		})
	}
}

func TestParser_ParseStateFieldRecovery(t *testing.T) {
	src := "state a: {}\nstate b: Integer = 0\nstate c: Float" + runSuffix
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.Parse()

	got := p.Diagnostics()
	if len(got) != 1 {
		for i, d := range got {
			t.Logf("got[%d] %+v", i, d)
		}
		t.Fatalf("expected 1 error, got %d", len(got))
	}

	if prog.Valid {
		t.Errorf("expected prog be invalid, got true")
	}

	if len(prog.StateFields) != 2 {
		t.Fatalf("expected 2 state fields parsed after recovery, got %d", len(prog.StateFields))
	}
}

func TestParser_ParseFunc(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		buildDiags func([]token.Pos) []diag.Diagnostic
	}{
		{
			"missing ident",
			"function ^() {}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.LPAREN),
				}
			},
		},
		{
			"missing (",
			"function Init^) {}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.LPAREN, token.RPAREN),
				}
			},
		},
		{
			"missing )",
			"function Init(^{}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.RPAREN, token.LBRACE),
				}
			},
		},
		{
			"missing ()",
			"function Init^{}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.LPAREN, token.LBRACE),
				}
			},
		},
		{
			"missing iden and (",
			"function ^){}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.RPAREN),
				}
			},
		},
		{
			"just keyword",
			"function^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.EOF),
				}
			},
		},
		{
			"with trailing tokens",
			"function Init() {}^3",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.NEWLINE, token.INTEGER),
				}
			},
		},
		{
			"missing {",
			"function Init() ^}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.LBRACE, token.RBRACE),
				}
			},
		},
		{
			"trailing token after function end",
			"function Run() {let a = 3}^3",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.NEWLINE, token.INTEGER),
				}
			},
		},
		{
			"empty Init allowed",
			"function Init() {}^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					missingRunErr(ps[0]),
				}
			},
		},
		{
			"missing Run on otherwise empty program",
			"^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					missingRunErr(ps[0]),
				}
			},
		},
		{
			"unclosed body reports missing }",
			"function Run() {let a = 3^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.RBRACE, token.EOF),
				}
			},
		},
		{
			// unclosed empty body: missing } wins, not the empty-body error
			"unclosed empty Init reports missing }",
			"function Init() {^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.RBRACE, token.EOF),
				}
			},
		},
		{
			"empty Run",
			"function Run() {^}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					emptyFuncErr(ps[0]),
				}
			},
		},
		{
			"forbidden func name",
			"function ^MyFunction() {}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					forbiddenFuncErr(ps[0]),
				}
			},
		},
		{
			// the body is parsed, not skipped: recovery must not land inside it
			"forbidden func name with body",
			"function ^MyFunction() {\nlet a = 3\na + 1\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					forbiddenFuncErr(ps[0]),
				}
			},
		},
		{
			"correct Run",
			"function Run() {let a = 3}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runDiagCases(t, tt.input, tt.buildDiags)
		})
	}
}

func TestParser_ParseFuncBlockSimple(t *testing.T) {
	tests := []struct {
		input  string
		output string
	}{
		{"let foo = 5", "let foo = 5"},
		{"foo.bar()", "foo.bar()"},
		{"foo.bar(2)", "foo.bar(2)"},
		{"foo.bar((2+5))", "foo.bar((2 + 5))"},
		{"foo.bar(key=value)", "foo.bar(key = value)"},
		{"foo.bar(2, key=value)", "foo.bar(2, key = value)"},
		{"foo.bar((2+5), key=value)", "foo.bar((2 + 5), key = value)"},
		{"foo.bar((2+5), key=(3+5))", "foo.bar((2 + 5), key = (3 + 5))"},
		{"foo.bar(\n(2+5), \nkey=(3+5)\n)", "foo.bar((2 + 5), key = (3 + 5))"},
		{"foo.bar(2,)", "foo.bar(2)"},
		{"foo.bar(key=value, )", "foo.bar(key = value)"},
		{"foo.bar(2, key=value,\n)", "foo.bar(2, key = value)"},
		{"emit(foo)", "emit(foo)"},
		{`emit(foo, "bar")`, `emit(foo, "bar")`},
		{"emit(foo, bar=value)", "emit(foo, bar = value)"},
		{"emit(\nfoo, \nbar=value\n)", "emit(foo, bar = value)"},
		{"if (a) {\nlet b = 3\n}", "if (a) {\nlet b = 3\n}"},
		{"if (a) {\nlet b = 3\nlet c = 4\n}", "if (a) {\nlet b = 3\nlet c = 4\n}"},
		{"if (a > b) {\nlet c = 3\n}", "if ((a > b)) {\nlet c = 3\n}"},
		{"if (a) {\nlet b = 3\n} else {\nlet c = 4\n}", "if (a) {\nlet b = 3\n} else {\nlet c = 4\n}"},
		{"if (a) {\nlet b = 3\n} else if (c) {\nlet d = 4\n}", "if (a) {\nlet b = 3\n} else if (c) {\nlet d = 4\n}"},
		// `else` may start its own line — nothing else can follow a `}` this way
		{"if (a) {\nlet b = 3\n}\nelse {\nlet c = 4\n}", "if (a) {\nlet b = 3\n} else {\nlet c = 4\n}"},
		{"if (a) {\nlet b = 3\n}\nelse if (c) {\nlet d = 4\n}", "if (a) {\nlet b = 3\n} else if (c) {\nlet d = 4\n}"},
		{"if (a) {\nlet b = 3\n} // why\nelse {\nlet c = 4\n}", "if (a) {\nlet b = 3\n} else {\nlet c = 4\n}"},
		{"if (a) {\nif (b) {\nlet c = 3\n}\n}", "if (a) {\nif (b) {\nlet c = 3\n}\n}"},
		// newlines are suppressed inside (), so a multi-line condition parses fine
		{"if (a &&\nb) {\nlet c = 3\n}", "if ((a && b)) {\nlet c = 3\n}"},
		// bare assignment (reassignment of an existing binding, no `let`)
		{"x = 5", "x = 5"},
		{"uptrend = a > b", "uptrend = (a > b)"},
		// member-target assignment
		{"state.cooldown = 0", "state.cooldown = 0"},
		{"state.cooldown = math.max(0, state.cooldown - 1)", "state.cooldown = math.max(0, (state.cooldown - 1))"},
		{"state.cooldown = state.cooldown - 1", "state.cooldown = (state.cooldown - 1)"},
		{"let x = state.cooldown", "let x = state.cooldown"},
		// empty else block still renders (gated on the else token, not the slice)
		{"if (a) {} else {}", "if (a) {} else {}"},
		// index (history) access
		{"foo[1]", "foo[1]"},
		{"foo[i + 1]", "foo[(i + 1)]"},
		{"close[1] > close[2]", "(close[1] > close[2])"},
		{"foo.bar[1]", "foo.bar[1]"},
		{"foo[0].bar", "foo[0].bar"},
		{"foo[bar[0]]", "foo[bar[0]]"},
		{"foo[0][1]", "foo[0][1]"},
		{"sma(candles, 14)[0]", "sma(candles, 14)[0]"},
		{"foo()[0].bar", "foo()[0].bar"},
		{`state["my key"]`, `state["my key"]`},
		{"foo[\ni + 1\n]", "foo[(i + 1)]"},
		{"foo[1] * 2", "(foo[1] * 2)"},
		{"-close[1]", "-close[1]"},
		// index target assignment (assignability is the analyzer's call)
		{"foo[0] = 5", "foo[0] = 5"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			input := fmt.Sprintf("function Run() {\n %s \n}", tt.input)
			l := lexer.New(input)
			p := parser.New(l)
			prog := p.Parse()
			if len(p.Diagnostics()) > 0 {
				for _, d := range p.Diagnostics() {
					t.Log(d)
				}
				t.Fatalf("expected 0 errors, got %d\n", len(p.Diagnostics()))
			}

			if tt.output != prog.RunFn.Body.Stmts[0].String() {
				t.Errorf("expected %s, got %s", tt.output, prog.RunFn.Body.Stmts[0].String())
			}

			if !prog.Valid {
				t.Errorf("expected prog be valid, got false")
			}
		})
	}
}

func TestParser_ParseFuncBody(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		buildDiags func([]token.Pos) []diag.Diagnostic
	}{
		{
			"call expr",
			"let foo = math.pow(2)",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{}
			},
		},
		{
			"missing ident",
			"let ^= 3",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.ASSIGN),
				}
			},
		},
		{
			"missing expression",
			"let foo =^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.NEWLINE),
				}
			},
		},
		{
			"member method missing",
			"foo.^(2)\n2",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.LPAREN),
				}
			},
		},
		{
			"member ) in call expr",
			"foo.bar(2 ^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					// NOTE: error on next line after input
					unexpectedErr(token.Pos{Line: 3, Col: 1}, token.RPAREN, token.RBRACE),
				}
			},
		},
		{
			"missing new line between statements",
			"foo.bar ^foo.bar",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.NEWLINE, token.IDENT),
				}
			},
		},
		{
			"if missing (",
			"if ^a) {}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.LPAREN, token.IDENT),
				}
			},
		},
		{
			"if missing )",
			"if (a ^{}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.RPAREN, token.LBRACE),
				}
			},
		},
		{
			"if missing {",
			"if (a) ^x",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.LBRACE, token.IDENT),
				}
			},
		},
		{
			"if empty condition",
			"if (^) {}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.RPAREN),
				}
			},
		},
		{
			"else without block",
			"if (a) {} else ^x",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.LBRACE, token.IDENT),
				}
			},
		},
		{
			"bad statement inside if body recovers",
			"if (a) {\nlet ^= 3\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.ASSIGN),
				}
			},
		},
		{
			// keep-partial: a broken condition must NOT hide the body error
			"if bad condition and bad body in one pass",
			"if (^) {\nlet ^= 3\n}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.RPAREN),
					unexpectedErr(ps[1], token.IDENT, token.ASSIGN),
				}
			},
		},
		{
			"assignment missing rhs",
			"x =^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.NEWLINE),
				}
			},
		},
		{
			"index empty subscript",
			"foo[^]",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.RBRACKET),
				}
			},
		},
		{
			// newlines are suppressed inside [], so the missing ] is reported on the next line
			"index missing ]",
			"foo[1 ^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(token.Pos{Line: 3, Col: 1}, token.RBRACKET, token.RBRACE),
				}
			},
		},
		{
			"index bad subscript",
			"foo[^#]",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.ILLEGAL),
				}
			},
		},
		{
			"index trailing infix",
			"foo[1 + ^]",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.RBRACKET),
				}
			},
		},
		{
			"index comma expression",
			"foo[1^, 2]",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.RBRACKET, token.COMMA),
				}
			},
		},
		{
			// two broken statements in the same block both report
			"two bad statements in one block",
			"let ^= 3\nlet ^= 4",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.ASSIGN),
					unexpectedErr(ps[1], token.IDENT, token.ASSIGN),
				}
			},
		},
		{
			// state is a keyword, so it can't be a let binding name
			"state as let name",
			"let ^state = 5",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.STATE),
				}
			},
		},
		{
			// a state decl belongs at the top level only
			"state decl in body",
			"^state cooldown: Integer = 0",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					topDeclInBodyErr(ps[0], token.STATE),
				}
			},
		},
		{
			"const decl in body",
			"^const FOO = 1",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					topDeclInBodyErr(ps[0], token.CONST),
				}
			},
		},
		{
			"input decl in body",
			"^input btc: CandleSeries",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					topDeclInBodyErr(ps[0], token.INPUT),
				}
			},
		},
		{
			"output decl in body",
			"^output alerts: Integer",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					topDeclInBodyErr(ps[0], token.OUTPUT),
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := fmt.Sprintf("function Run() {\n%s\n}", tt.input)
			runDiagCases(t, input, tt.buildDiags)
		})
	}
}

func unexpectedErr(pos token.Pos, expected, got token.TokenType) *diag.UnexpectedToken {
	return &diag.UnexpectedToken{Phase: diag.PhaseParse, Pos: pos, Expected: expected, Got: got}
}

func expectedTypeOrCustomType(pos token.Pos) *diag.TypeOrCustomTypeExpected {
	return &diag.TypeOrCustomTypeExpected{Phase: diag.PhaseParse, Pos: pos}
}

func emptyCustomType(pos token.Pos) *diag.EmptyCustomType {
	return &diag.EmptyCustomType{Phase: diag.PhaseParse, Pos: pos}
}

func exprExpectedErr(pos token.Pos, got token.TokenType) *diag.ExpressionExpected {
	return &diag.ExpressionExpected{Phase: diag.PhaseParse, Pos: pos, Got: got}
}

func parseFailedErr(pos token.Pos, target token.TokenType) *diag.ParseFailed {
	return &diag.ParseFailed{Phase: diag.PhaseParse, Pos: pos, Target: target}
}

func emptyFuncErr(pos token.Pos) *diag.EmptyFunctionBody {
	return &diag.EmptyFunctionBody{Phase: diag.PhaseParse, Pos: pos}
}

func forbiddenFuncErr(pos token.Pos) *diag.ForbiddenFunc {
	return &diag.ForbiddenFunc{Phase: diag.PhaseParse, Pos: pos}
}

func missingRunErr(pos token.Pos) *diag.MissingRunFunc {
	return &diag.MissingRunFunc{Phase: diag.PhaseParse, Pos: pos}
}

func topDeclInBodyErr(pos token.Pos, keyword token.TokenType) *diag.TopDeclInBody {
	return &diag.TopDeclInBody{Phase: diag.PhaseParse, Pos: pos, Keyword: keyword}
}

func unexpectedTopDeclErr(pos token.Pos) *diag.UnexpectedTopDecl {
	return &diag.UnexpectedTopDecl{Phase: diag.PhaseParse, Pos: pos}
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

	got := p.Diagnostics()
	if len(got) != len(expected) {
		for i, d := range got {
			t.Logf("got[%d] %+v", i, d)
		}
		t.Fatalf("diag count: got %d, want %d", len(got), len(expected))
	}

	if diff := cmp.Diff(expected, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("diagnostics mismatch (-want +got):\n%s", diff)
	}

	if len(expected) > 0 && prog.Valid {
		t.Errorf("expected prog be invalid, got true")
	}
}

func TestParser_ErrorCap(t *testing.T) {
	src := strings.Repeat("@\n", 300)
	p := parser.New(lexer.New(src))
	prog := p.Parse()

	if prog.Valid {
		t.Errorf("expected prog be invalid, got true")
	}
	if got := len(p.Diagnostics()); got != 100 {
		t.Fatalf("expected diagnostics capped at 100, got %d", got)
	}
}

// Program-level recovery: a junk top-level token must not drop the following good const
func TestParser_ErrorModeLeak(t *testing.T) {
	t.Run("junk before good const is recovered", func(t *testing.T) {
		src := "@\nconst FOO = 5"
		l := lexer.New(src)
		p := parser.New(l)
		prog := p.Parse()

		if prog.Valid {
			t.Errorf("expected prog invalid (junk `@` present), got valid")
		}
		if len(prog.Consts) != 1 {
			t.Fatalf("expected 1 const recovered after junk, got %d", len(prog.Consts))
		}
		if prog.Consts[0].Identifier.String() != "FOO" {
			t.Errorf("expected recovered const to be valid")
		}
		if got := prog.Consts[0].String(); got != "const FOO = 5" {
			t.Errorf("expected recovered const %q, got %q", "const FOO = 5", got)
		}
	})

	t.Run("erroring program reports invalid", func(t *testing.T) {
		// No CONST branch runs, but there is an error -> Valid must be false.
		for _, src := range []string{"5", "@"} {
			l := lexer.New(src)
			p := parser.New(l)
			prog := p.Parse()

			if len(p.Diagnostics()) == 0 {
				t.Errorf("%q: expected at least one diagnostic, got none", src)
			}
			if prog.Valid {
				t.Errorf("%q: expected prog invalid (has errors), got valid", src)
			}
		}
	})
}

func TestParser_UnexpectedTopDecl(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		buildDiags func([]token.Pos) []diag.Diagnostic
	}{
		{"stray integer", "^5", func(ps []token.Pos) []diag.Diagnostic {
			return []diag.Diagnostic{unexpectedTopDeclErr(ps[0])}
		}},
		{"stray illegal", "^@", func(ps []token.Pos) []diag.Diagnostic {
			return []diag.Diagnostic{unexpectedTopDeclErr(ps[0])}
		}},
		{"stray rbrace", "^}", func(ps []token.Pos) []diag.Diagnostic {
			return []diag.Diagnostic{unexpectedTopDeclErr(ps[0])}
		}},
		{"let at top level", "^let x = 1", func(ps []token.Pos) []diag.Diagnostic {
			return []diag.Diagnostic{unexpectedTopDeclErr(ps[0])}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runDiagCases(t, tt.input+runSuffix, tt.buildDiags)
		})
	}
}

func TestParser_DeepNesting(t *testing.T) {
	const depth = 5000
	tests := map[string]string{
		"nested parens": "const VALUE = " + strings.Repeat("(", depth) + "5" + strings.Repeat(")", depth) + runSuffix,
		"prefix minus":  "const VALUE = " + strings.Repeat("-", depth) + "5" + runSuffix,
		"prefix bang":   "const VALUE = " + strings.Repeat("!", depth) + "true" + runSuffix,
		"else if chain": "function Run() {\nif (a) {}" + strings.Repeat(" else if (a) {}", depth) + "\n}",
	}
	for name, src := range tests {
		t.Run(name, func(t *testing.T) {
			p := parser.New(lexer.New(src))
			prog := p.Parse()

			if prog.Valid {
				t.Errorf("expected invalid program for deeply nested input, got valid")
			}
			found := false
			for _, d := range p.Diagnostics() {
				if _, ok := d.(*diag.NestingTooDeep); ok {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected a NESTING_TOO_DEEP diagnostic, got: %v", p.Diagnostics())
			}
		})
	}
}
