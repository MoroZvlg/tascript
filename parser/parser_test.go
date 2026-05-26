package parser_test

import (
	"testing"

	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/parser"
	"github.com/MoroZvlg/tascript/token"
)

func TestParser_PrattPrecedenceInEmitKwarg(t *testing.T) {
	prog := parse(t, `output alerts {
  v: Number
}
function Init() {}
function Run() {
  emit(alerts, v=1 + 2 * 3)
}
`)
	em := prog.Run.Body[0].(*ast.EmitStmt)
	expr := em.Kwargs[0].Value.(*ast.BinaryExpr)
	if expr.Op != token.PLUS {
		t.Fatalf("root op = %s, want +", expr.Op)
	}
	right := expr.Right.(*ast.BinaryExpr)
	if right.Op != token.ASTERISK {
		t.Fatalf("right op = %s, want *", right.Op)
	}
}

func TestParser_PostfixExpressions(t *testing.T) {
	prog := parse(t, `output alerts {
  v: Number
}
function Init() {}
function Run() {
  emit(alerts, v=btc.rsi(14)[1] + math.max(1, 2))
}
`)
	em := prog.Run.Body[0].(*ast.EmitStmt)
	sum := em.Kwargs[0].Value.(*ast.BinaryExpr)
	if sum.Op != token.PLUS {
		t.Fatalf("root op = %s, want +", sum.Op)
	}

	idx := sum.Left.(*ast.IndexExpr)
	call := idx.Object.(*ast.CallExpr)
	member := call.Callee.(*ast.MemberExpr)
	if ident := member.Object.(*ast.Ident); ident.Name != "btc" || member.Name != "rsi" {
		t.Fatalf("left callee = %#v.%s, want btc.rsi", member.Object, member.Name)
	}

	rightCall := sum.Right.(*ast.CallExpr)
	rightMember := rightCall.Callee.(*ast.MemberExpr)
	if ident := rightMember.Object.(*ast.Ident); ident.Name != "math" || rightMember.Name != "max" {
		t.Fatalf("right callee = %#v.%s, want math.max", rightMember.Object, rightMember.Name)
	}
	if len(rightCall.Args) != 2 {
		t.Fatalf("math.max args = %d, want 2", len(rightCall.Args))
	}
}

func parse(t *testing.T, src string) *ast.Program {
	t.Helper()
	toks := lexer.New([]byte(src), "").Tokenize()
	p := parser.New(toks)
	prog := p.Parse()
	if len(p.Diagnostics()) > 0 {
		t.Fatalf("parser diagnostics: %#v", p.Diagnostics())
	}
	return prog
}
