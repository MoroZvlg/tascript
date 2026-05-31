package parser_test

import (
	"testing"

	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/parser"
	"github.com/MoroZvlg/tascript/token"
	"github.com/google/go-cmp/cmp"
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
				t.Errorf("expected 0 errors, got %d\n", len(p.Diagnostics()))
				return
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
			"missing all",
			"const ^",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					unexpectedErr(ps[0], token.IDENT, token.EOF),
				}
			},
		},
		// errors in expression parsing
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, pos := extractErrorsPos(tt.input)
			expected := tt.buildDiags(pos)
			l := lexer.New(input)
			p := parser.New(l)
			prog := p.Parse()

			got := p.Diagnostics()
			if len(got) != len(expected) {
				for i, d := range got {
					t.Logf("got[%d] %+v", i, d)
				}
				t.Fatalf("diag count: got %d, want %d", len(got), len(expected))
			}

			if diffs := cmp.Diff(expected, got); diffs != "" {
				t.Errorf("diagnostics mismatch (-want +got):\n%s", diffs)
			}

			if prog.Valid {
				t.Errorf("expected prog be invalid, got true")
			}
		})
	}
}

func TestParser_ParseConstRecovery(t *testing.T) {
	src := `const = bar\n const foo = baz`
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.Parse()

	got := p.Diagnostics()
	if len(got) != 1 {
		for i, d := range got {
			t.Logf("got[%d] %+v", i, d)
		}
		t.Errorf("expected 1 error, got %d", len(got))
	}
	if len(prog.Consts) != 1 {
		t.Errorf("expecter 1 constant parsed after recovery, got %d", len(prog.Consts))
	}

	if prog.Valid {
		t.Errorf("expected prog be invalid, got true")
	}
}

func unexpectedErr(pos token.Pos, expected, got token.TokenType) *diag.UnexpectedToken {
	return &diag.UnexpectedToken{Phase: diag.PhaseParse, Pos: pos, Expected: expected, Got: got}
}

func exprExpectedErr(pos token.Pos, got token.TokenType) *diag.ExpressionExpected {
	return &diag.ExpressionExpected{Phase: diag.PhaseParse, Pos: pos, Got: got}
}

func extractErrorsPos(input string) (string, []token.Pos) {
	var out []byte
	var pos []token.Pos
	line, col := 1, 1
	for i := 0; i < len(input); i++ {
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
