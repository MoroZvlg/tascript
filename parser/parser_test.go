package parser_test

import (
	"strings"
	"testing"

	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/parser"
	"github.com/MoroZvlg/tascript/token"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestParser_ParseConstSimple(t *testing.T) {
	tests := []struct {
		input  string
		output string
	}{
		{"const foo = bar", "const foo = bar"},
		{"const foo = 5", "const foo = 5"},
		{"const foo = 5.3", "const foo = 5.3"},
		{`const foo = "bar"`, `const foo = "bar"`},
		{"const foo = false", "const foo = false"},
		{"const foo = -5.3 + 3", "const foo = (-5.3 + 3)"},
		{"const foo = -(5.3 + 3)", "const foo = -(5.3 + 3)"},
		{"const foo = 5.3 + 3", "const foo = (5.3 + 3)"},
		{"const foo = (5.3 + 3) * 2", "const foo = ((5.3 + 3) * 2)"},
		{"const foo = true == 5 > 2", "const foo = (true == (5 > 2))"},
		{"const foo = module.math.PI", "const foo = module.math.PI"},
		{"\nconst foo = 5\n", "const foo = 5"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
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
			"const foo ^3",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.ASSIGN, token.INTEGER),
				}
			},
		},
		{
			"missing expression",
			"const foo = ^",
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
			"const foo^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.ASSIGN, token.EOF),
				}
			},
		},
		{
			"missing `=` and bad expression",
			"const foo ^3 + ^#",
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
			"const foo = 3 + ^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.EOF),
				}
			},
		},
		{
			"[Infix] missing LHS",
			"const foo = ^* 3",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.ASTERISK),
				}
			},
		},
		{
			"[Infix] missing both hands",
			"const foo = 3 + ^*",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.ASTERISK),
				}
			},
		},
		{
			"???",
			"const foo = (3 + ^) * ^#",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.RPAREN),
					exprExpectedErr(ps[1], token.ILLEGAL),
				}
			},
		},
		{
			"prefix expr error",
			"const foo = -^#",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.ILLEGAL),
				}
			},
		},
		{
			"member access call. no ident",
			"const foo = math.^3",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.INTEGER),
				}
			},
		},
		{
			"integer literal overflow",
			"const foo = ^" + strings.Repeat("9", 100),
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					parseFailedErr(ps[0], token.INTEGER),
				}
			},
		},
		{
			"float literal overflow",
			"const foo = ^" + strings.Repeat("9", 400) + ".0",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					parseFailedErr(ps[0], token.FLOAT),
				}
			},
		},
		{
			"empty group expression",
			"const foo = (^)",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					exprExpectedErr(ps[0], token.RPAREN),
				}
			},
		},
		{
			"group expression. missing RPAREN",
			"const foo = (3 + 1 ^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.RPAREN, token.EOF),
				}
			},
		},
		{
			"trailing token after expression",
			"const foo = a ^b",
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
	src := "const = bar\n const foo = baz\nconst math = pi"
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
			l := lexer.New(tt.input)
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
			"missing right brace",
			"input btc: {btc: btc^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.RBRACE, token.EOF),
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
			// a trailing comma reopens the field loop, which then demands another field
			"trailing comma in custom type",
			"input btc: {foo: Integer,^}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.RBRACE),
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
			runDiagCases(t, tt.input, tt.buildDiags)
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
		t.Fatalf("expected 2 inputs parsed after recovery, got %d", len(prog.Consts))
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
			l := lexer.New(tt.input)
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
			"missing right brace",
			"output alert: {alert: alert^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.RBRACE, token.EOF),
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
			// a trailing comma reopens the field loop, which then demands another field
			"trailing comma in custom type",
			"output alert: {foo: Integer,^}",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.RBRACE),
				}
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
			runDiagCases(t, tt.input, tt.buildDiags)
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

// Program-level recovery: a junk top-level token must not drop the following good const
func TestParser_ErrorModeLeak(t *testing.T) {
	t.Run("junk before good const is recovered", func(t *testing.T) {
		src := "@\nconst foo = 5"
		l := lexer.New(src)
		p := parser.New(l)
		prog := p.Parse()

		if prog.Valid {
			t.Errorf("expected prog invalid (junk `@` present), got valid")
		}
		if len(prog.Consts) != 1 {
			t.Fatalf("expected 1 const recovered after junk, got %d", len(prog.Consts))
		}
		if prog.Consts[0].Identifier.String() != "foo" {
			t.Errorf("expected recovered const to be valid")
		}
		if got := prog.Consts[0].String(); got != "const foo = 5" {
			t.Errorf("expected recovered const %q, got %q", "const foo = 5", got)
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
