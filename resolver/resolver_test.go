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

const runSuffix = "\nfunction Run() {\nlet a = 1\n}"

func TestResolver_ResolveConst(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		buildDiags func([]token.Pos) []diag.Diagnostic
	}{
		{
			"duplicate declaration",
			"const foo = bar\n ^const ^foo = baz",
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addDuplicateDecl(
						token.Token{Type: token.CONST, Pos: ps[0], Literal: "const"},
						token.Token{Type: token.IDENT, Pos: ps[1], Literal: "foo"},
					),
				}
			},
		},
		{
			"int + string",
			`const foo = 1 ^+ "foo"`,
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
			`const foo = ^!1`,
			func(ps []token.Pos) []diag.Diagnostic {
				return []diag.Diagnostic{
					addInvalidUnaryOp(
						token.Token{Type: token.BANG, Pos: ps[0], Literal: "!"},
						registry.IntegerID,
					),
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
	result := resolv.Resolve()

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

	if len(expected) > 0 && result {
		t.Errorf("expected resolver to return false")
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
