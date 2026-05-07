package parser_test

import (
	"fmt"
	"testing"

	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/parser"
)

func Test_ParseProgram_Let(t *testing.T) {
	input := `let x = 5.0;
  let y = 10;
  let foobar = 838383;`
	l := lexer.New(input)
	program := parser.New(l)
	resultAST := program.ParseProgram()

	for _, stmt := range resultAST.Statements {
		fmt.Println(stmt.String())
	}
	if len(program.Errors()) != 0 {
		t.Errorf("expected 0 errors, got %d", len(program.Errors()))
	}

	if len(resultAST.Statements) != 3 {
		t.Errorf("expected at least 3 statements, got %d", len(resultAST.Statements))
	}

	expectedStatements := []*ast.LetStatement{
		{Name: &ast.Identifier{Value: "x"}},
		{Name: &ast.Identifier{Value: "y"}},
		{Name: &ast.Identifier{Value: "foobar"}},
	}
	for i, stmt := range resultAST.Statements {
		letStmt, ok := stmt.(*ast.LetStatement)
		if !ok {
			t.Errorf("expected let statement, got %T", stmt)
		}
		if letStmt.Name.String() != expectedStatements[i].Name.String() {
			t.Errorf("expected let statement value %s, got %s", expectedStatements[i].Name.String(), letStmt.Name.String())
		}
	}
}

func Test_ParseProgram_LetError(t *testing.T) {
	input := `let = 5;`
	l := lexer.New(input)
	program := parser.New(l)
	_ = program.ParseProgram()
	if len(program.Errors()) == 0 {
		t.Errorf("expected at least one error, got %d", len(program.Errors()))
	}
}

func Test_ParseProgram_Return(t *testing.T) {
	input := `return 5;
  return 10;
  return 993322;`
	l := lexer.New(input)
	program := parser.New(l)
	resultAST := program.ParseProgram()

	if len(program.Errors()) != 0 {
		t.Errorf("expected 0 errors, got %d", len(program.Errors()))
	}

	if len(resultAST.Statements) != 3 {
		t.Errorf("expected at least 3 statements, got %d", len(resultAST.Statements))
	}

	for _, stmt := range resultAST.Statements {
		_, ok := stmt.(*ast.ReturnStatement)
		if !ok {
			t.Errorf("expected return statement, got %T", stmt)
		}
	}
}

func Test_ParseProgram_SimpleExpressions(t *testing.T) {
	tests := []struct {
		input             string
		expectedExprValue any
	}{
		{"5;", int64(5)},
		{"foobar;", "foobar"},
		{"true;", true},
		{"false;", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			program := parser.New(l)
			resultAST := program.ParseProgram()

			if len(program.Errors()) != 0 {
				t.Errorf("expected 0 errors, got %d", len(program.Errors()))
			}

			if len(resultAST.Statements) != 1 {
				t.Errorf("expected at least 1 statements, got %d", len(resultAST.Statements))
			}

			for _, stmt := range resultAST.Statements {
				exprStmt, ok := stmt.(*ast.ExpressionStatement)
				if !ok {
					t.Errorf("expected expression statement, got %T", stmt)
				}
				assertLiteral(t, exprStmt.Expression, tt.expectedExprValue)
			}
		})
	}
}

func Test_ParseProgram_ExpressionWrong(t *testing.T) {
	input := `@;`
	l := lexer.New(input)
	program := parser.New(l)
	_ = program.ParseProgram()

	if len(program.Errors()) != 1 {
		t.Errorf("expected 1 errors, got %d", len(program.Errors()))
	}
}

func Test_ParseProgram_PrefixExpressions(t *testing.T) {
	tests := []struct {
		input    string
		operator string
		rightVal any
	}{
		{"-5;", "-", int64(5)},
		{"!foobar;", "!", "foobar"},
		{"-42;", "-", int64(42)},
		{"!x;", "!", "x"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			prog := p.ParseProgram()
			assertNoErrors(t, p)

			if len(prog.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
			}
			exprStmt, ok := prog.Statements[0].(*ast.ExpressionStatement)
			if !ok {
				t.Fatalf("expected ExpressionStatement, got %T", prog.Statements[0])
			}
			prefix, ok := exprStmt.Expression.(*ast.PrefixExpression)
			if !ok {
				t.Fatalf("expected PrefixExpression, got %T", exprStmt.Expression)
			}
			if prefix.Operator != tt.operator {
				t.Errorf("expected operator %q, got %q", tt.operator, prefix.Operator)
			}
			assertLiteral(t, prefix.Right, tt.rightVal)
		})
	}
}

func Test_ParseProgram_InfixExpressions(t *testing.T) {
	tests := []struct {
		input    string
		leftVal  any
		operator string
		rightVal any
	}{
		{"5 + 5;", int64(5), "+", int64(5)},
		{"5 - 5;", int64(5), "-", int64(5)},
		{"5 * 5;", int64(5), "*", int64(5)},
		{"5 / 5;", int64(5), "/", int64(5)},
		{"5 > 5;", int64(5), ">", int64(5)},
		{"5 < 5;", int64(5), "<", int64(5)},
		{"5 >= 5;", int64(5), ">=", int64(5)},
		{"5 <= 5;", int64(5), "<=", int64(5)},
		{"5 == 5;", int64(5), "==", int64(5)},
		{"5 != 5;", int64(5), "!=", int64(5)},
		{"a && b;", "a", "&&", "b"},
		{"a || b;", "a", "||", "b"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			prog := p.ParseProgram()
			assertNoErrors(t, p)

			if len(prog.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
			}
			exprStmt, ok := prog.Statements[0].(*ast.ExpressionStatement)
			if !ok {
				t.Fatalf("expected ExpressionStatement, got %T", prog.Statements[0])
			}
			infix, ok := exprStmt.Expression.(*ast.InfixExpression)
			if !ok {
				t.Fatalf("expected InfixExpression, got %T", exprStmt.Expression)
			}
			assertLiteral(t, infix.Left, tt.leftVal)
			if infix.Operator != tt.operator {
				t.Errorf("expected operator %q, got %q", tt.operator, infix.Operator)
			}
			assertLiteral(t, infix.Right, tt.rightVal)
		})
	}
}

func Test_ParseProgram_OperatorPrecedence(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"-a * b", "((-a) * b)"},
		{"!-a", "(!(-a))"},
		{"a + b + c", "((a + b) + c)"},
		{"a + b - c", "((a + b) - c)"},
		{"a * b * c", "((a * b) * c)"},
		{"a * b / c", "((a * b) / c)"},
		{"a + b / c", "(a + (b / c))"},
		{"a + b * c + d / e - f", "(((a + (b * c)) + (d / e)) - f)"},
		{"3 + 4; -5 * 5", "(3 + 4)((-5) * 5)"},
		{"5 > 4 == 3 < 4", "((5 > 4) == (3 < 4))"},
		{"5 < 4 != 3 > 4", "((5 < 4) != (3 > 4))"},
		{"3 + 4 * 5 == 3 * 1 + 4 * 5", "((3 + (4 * 5)) == ((3 * 1) + (4 * 5)))"},
		{"a && b || c", "((a && b) || c)"},
		{"a || b && c", "(a || (b && c))"},
		{"(1 + 2) * 3", "((1 + 2) * 3)"},
		{"2 / (5 + 5)", "(2 / (5 + 5))"},
		{"-(5 + 5)", "(-(5 + 5))"},
		{"!(true == true)", "(!(true == true))"},
		{"if (x < y) { x }", "if (x < y) { x }"},
		{"if (x < y) { x } else { y }", "if (x < y) { x } else { y }"},
		{"a + add(b + c) + d", "((a + add((b + c))) + d)"},
		{"add(a, b, 1, 2 * 3, 4 + 5, add(6, 7 * 8))", "add(a, b, 1, (2 * 3), (4 + 5), add(6, (7 * 8)))"},
		{"-f(x)", "(-f(x))"},
		{"a.b", "a.b"},
		{"a.b.c", "a.b.c"},
		{"f().x", "f().x"},
		{"a + b.c", "(a + b.c)"},
		{"a.b(1)", "a.b(1)"},
		{"a + b[0]", "(a + b[0])"},
		{"b[0] + a", "(b[0] + a)"},
		{"-a[0]", "(-a[0])"},
		{"a[0][1]", "a[0][1]"},
		{"f(x)[0]", "f(x)[0]"},
		{"a.b[0]", "a.b[0]"},
		{"closes[i + 1] * 2", "(closes[(i + 1)] * 2)"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			prog := p.ParseProgram()
			assertNoErrors(t, p)

			got := prog.String()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func Test_ParseProgram_IfElseBlocks(t *testing.T) {
	tests := []struct {
		input    string
		withElse bool
	}{
		{"if (x < y) { x }", false},
		{"if (x < y) { x } else { y }", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			prog := p.ParseProgram()
			assertNoErrors(t, p)

			es, ok := prog.Statements[0].(*ast.ExpressionStatement)
			if !ok {
				t.Fatalf("expected ExpressionStatement, got %T", prog.Statements[0])
			}
			ifExpr, ok := es.Expression.(*ast.IfExpression)
			if !ok {
				t.Fatalf("expected IfExpression, got %T", es.Expression)
			}

			body := ifExpr.Consequence.Statements[0].(*ast.ExpressionStatement)
			assertLiteral(t, body.Expression, "x")

			if !tt.withElse && ifExpr.Alternative != nil {
				t.Errorf("expected nil Alternative, got %+v", ifExpr.Alternative)
			}

			if ifExpr.Alternative != nil {
				body = ifExpr.Alternative.Statements[0].(*ast.ExpressionStatement)
				assertLiteral(t, body.Expression, "y")
			}
		})
	}
}

func Test_ParseProgram_FuncCall(t *testing.T) {
	input := `add(1, 2 * 3, 4 + 5)`
	l := lexer.New(input)
	program := parser.New(l)
	resultAST := program.ParseProgram()

	if len(program.Errors()) != 0 {
		t.Errorf("expected 0 errors, got %d", len(program.Errors()))
	}

	if len(resultAST.Statements) != 1 {
		t.Errorf("expected at least 1 statements, got %d", len(resultAST.Statements))
	}

	exprStmt, ok := resultAST.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Errorf("expected expression statement, got %T", resultAST.Statements[0])
	}
	funcCall, ok := exprStmt.Expression.(*ast.FunctionCall)
	if !ok {
		t.Errorf("expected function call, got %T", resultAST.Statements[0])
	}
	if funcCall.Function.String() != "add" {
		t.Errorf("expected add, got %s", funcCall.Function.String())
	}
	if len(funcCall.Arguments) != 3 {
		t.Errorf("expected 3 arguments, got %d", len(funcCall.Arguments))
	}
	assertLiteral(t, funcCall.Arguments[0], int64(1))

	infArg, ok := funcCall.Arguments[1].(*ast.InfixExpression)
	if !ok {
		t.Errorf("expected InfixExpression, got %T", funcCall.Arguments[1])
	}
	if infArg.Operator != "*" {
		t.Errorf("expected %s, got %s", "*", infArg.Operator)
	}
	assertLiteral(t, infArg.Left, int64(2))
	assertLiteral(t, infArg.Right, int64(3))

	infArg, ok = funcCall.Arguments[2].(*ast.InfixExpression)
	if !ok {
		t.Errorf("expected InfixExpression, got %T", funcCall.Arguments[2])

	}
	if infArg.Operator != "+" {
		t.Errorf("expected %s, got %s", "+", infArg.Operator)
	}
	assertLiteral(t, infArg.Left, int64(4))
	assertLiteral(t, infArg.Right, int64(5))
}

func Test_ParseProgram_FuncCallInline(t *testing.T) {
	input := `function(x) { x }(5)`
	l := lexer.New(input)
	program := parser.New(l)
	resultAST := program.ParseProgram()

	if len(program.Errors()) != 0 {
		t.Errorf("expected 0 errors, got %d", len(program.Errors()))
	}

	if len(resultAST.Statements) != 1 {
		t.Errorf("expected at least 1 statements, got %d", len(resultAST.Statements))
	}

	exprStmt, ok := resultAST.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Errorf("expected expression statement, got %T", resultAST.Statements[0])
	}
	funcCall, ok := exprStmt.Expression.(*ast.FunctionCall)
	if !ok {
		t.Errorf("expected function call, got %T", resultAST.Statements[0])
	}
	if funcCall.Function.String() != "function(x) { x }" {
		t.Errorf("expected function declaration as name, got %s", funcCall.Function.String())
	}
	assertLiteral(t, funcCall.Arguments[0], int64(5))
}

func Test_ParseProgram_FunctionLiteral(t *testing.T) {
	tests := []struct {
		input   string
		params  []string
		bodyStr string // expected stringification of the body
	}{
		{"function() { }", []string{}, "{  }"},
		{"function(x) { x }", []string{"x"}, "{ x }"},
		{"function(x, y, z) { x + y }", []string{"x", "y", "z"}, "{ (x + y) }"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			prog := p.ParseProgram()
			assertNoErrors(t, p)

			exprStmt, ok := prog.Statements[0].(*ast.ExpressionStatement)
			if !ok {
				t.Fatalf("expected ExpressionStatement, got %T", prog.Statements[0])
			}
			fn, ok := exprStmt.Expression.(*ast.FunctionLiteral)
			if !ok {
				t.Fatalf("expected FunctionLiteral, got %T", exprStmt.Expression)
			}

			if len(fn.Parameters) != len(tt.params) {
				t.Fatalf("expected %d params, got %d", len(tt.params), len(fn.Parameters))
			}
			for i, name := range tt.params {
				if fn.Parameters[i].Value != name {
					t.Errorf("param[%d]: expected %q, got %q", i, name, fn.Parameters[i].Value)
				}
			}
			if fn.Body.String() != tt.bodyStr {
				t.Errorf("body: expected %q, got %q", tt.bodyStr, fn.Body.String())
			}
		})
	}
}

func Test_ParseProgram_MemberExpression_LeftAssoc(t *testing.T) {
	input := `a.b.c`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	assertNoErrors(t, p)

	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}
	exprStmt, ok := prog.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected ExpressionStatement, got %T", prog.Statements[0])
	}
	outer, ok := exprStmt.Expression.(*ast.MemberExpression)
	if !ok {
		t.Fatalf("expected outer MemberExpression, got %T", exprStmt.Expression)
	}
	if outer.Property.Value != "c" {
		t.Errorf("expected outer property %q, got %q", "c", outer.Property.Value)
	}
	inner, ok := outer.Object.(*ast.MemberExpression)
	if !ok {
		t.Fatalf("expected inner MemberExpression, got %T", outer.Object)
	}
	if inner.Property.Value != "b" {
		t.Errorf("expected inner property %q, got %q", "b", inner.Property.Value)
	}
	root, ok := inner.Object.(*ast.Identifier)
	if !ok {
		t.Fatalf("expected root Identifier, got %T", inner.Object)
	}
	if root.Value != "a" {
		t.Errorf("expected root ident %q, got %q", "a", root.Value)
	}
}

func Test_ParseProgram_MemberExpression_MissingIdent(t *testing.T) {
	input := `a.;`
	l := lexer.New(input)
	p := parser.New(l)
	_ = p.ParseProgram()

	if len(p.Errors()) == 0 {
		t.Fatalf("expected at least one parser error, got 0")
	}
}

func Test_ParseProgram_IndexExpression_Literal(t *testing.T) {
	input := `closes[0]`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	assertNoErrors(t, p)

	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}
	exprStmt, ok := prog.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected ExpressionStatement, got %T", prog.Statements[0])
	}
	idx, ok := exprStmt.Expression.(*ast.IndexExpression)
	if !ok {
		t.Fatalf("expected IndexExpression, got %T", exprStmt.Expression)
	}
	assertLiteral(t, idx.Left, "closes")
	assertLiteral(t, idx.Index, int64(0))
}

func Test_ParseProgram_IndexExpression_IdentifierIndex(t *testing.T) {
	input := `closes[i]`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	assertNoErrors(t, p)

	exprStmt, ok := prog.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected ExpressionStatement, got %T", prog.Statements[0])
	}
	idx, ok := exprStmt.Expression.(*ast.IndexExpression)
	if !ok {
		t.Fatalf("expected IndexExpression, got %T", exprStmt.Expression)
	}
	assertLiteral(t, idx.Left, "closes")
	assertLiteral(t, idx.Index, "i")
}

func Test_ParseProgram_IndexExpression_ExpressionIndex(t *testing.T) {
	input := `closes[i + 1]`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	assertNoErrors(t, p)

	exprStmt, ok := prog.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected ExpressionStatement, got %T", prog.Statements[0])
	}
	idx, ok := exprStmt.Expression.(*ast.IndexExpression)
	if !ok {
		t.Fatalf("expected IndexExpression, got %T", exprStmt.Expression)
	}
	assertLiteral(t, idx.Left, "closes")

	inf, ok := idx.Index.(*ast.InfixExpression)
	if !ok {
		t.Fatalf("expected InfixExpression as Index, got %T", idx.Index)
	}
	if inf.Operator != "+" {
		t.Errorf("expected operator %q, got %q", "+", inf.Operator)
	}
	assertLiteral(t, inf.Left, "i")
	assertLiteral(t, inf.Right, int64(1))
}

func Test_ParseProgram_IndexExpression_Chained(t *testing.T) {
	input := `m[0][1]`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	assertNoErrors(t, p)

	exprStmt, ok := prog.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected ExpressionStatement, got %T", prog.Statements[0])
	}
	outer, ok := exprStmt.Expression.(*ast.IndexExpression)
	if !ok {
		t.Fatalf("expected outer IndexExpression, got %T", exprStmt.Expression)
	}
	assertLiteral(t, outer.Index, int64(1))

	inner, ok := outer.Left.(*ast.IndexExpression)
	if !ok {
		t.Fatalf("expected inner IndexExpression, got %T", outer.Left)
	}
	assertLiteral(t, inner.Left, "m")
	assertLiteral(t, inner.Index, int64(0))
}

func Test_ParseProgram_IndexExpression_OnCallResult(t *testing.T) {
	input := `sma(candles, 14)[0]`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	assertNoErrors(t, p)

	exprStmt, ok := prog.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected ExpressionStatement, got %T", prog.Statements[0])
	}
	idx, ok := exprStmt.Expression.(*ast.IndexExpression)
	if !ok {
		t.Fatalf("expected IndexExpression, got %T", exprStmt.Expression)
	}
	assertLiteral(t, idx.Index, int64(0))

	call, ok := idx.Left.(*ast.FunctionCall)
	if !ok {
		t.Fatalf("expected FunctionCall as Left, got %T", idx.Left)
	}
	if call.Function.String() != "sma" {
		t.Errorf("expected function %q, got %q", "sma", call.Function.String())
	}
	if len(call.Arguments) != 2 {
		t.Errorf("expected 2 arguments, got %d", len(call.Arguments))
	}
}

func Test_ParseProgram_IndexExpression_MissingBracket(t *testing.T) {
	input := `closes[0;`
	l := lexer.New(input)
	p := parser.New(l)
	_ = p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatalf("expected at least one parser error, got 0")
	}
}

func Test_ParseProgram_IndexExpression_EmptyBrackets(t *testing.T) {
	input := `closes[]`
	l := lexer.New(input)
	p := parser.New(l)
	_ = p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatalf("expected at least one parser error, got 0")
	}
}

func Test_ParseProgram_StringLiteral(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"hello world";`, "hello world"},
		{`"";`, ""},
		{`"oversold";`, "oversold"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			prog := p.ParseProgram()
			assertNoErrors(t, p)

			if len(prog.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
			}
			exprStmt, ok := prog.Statements[0].(*ast.ExpressionStatement)
			if !ok {
				t.Fatalf("expected ExpressionStatement, got %T", prog.Statements[0])
			}
			str, ok := exprStmt.Expression.(*ast.StringLiteral)
			if !ok {
				t.Fatalf("expected StringLiteral, got %T", exprStmt.Expression)
			}
			if str.Value != tt.expected {
				t.Errorf("expected value %q, got %q", tt.expected, str.Value)
			}
		})
	}
}

func assertNoErrors(t *testing.T, p *parser.Parser) {
	t.Helper()
	if len(p.Errors()) == 0 {
		return
	}
	for _, e := range p.Errors() {
		t.Logf("parser error: %s", e)
	}
	t.Fatalf("expected 0 errors, got %d", len(p.Errors()))
}

func assertLiteral(t *testing.T, expr ast.Expression, expected any) {
	t.Helper()
	switch v := expected.(type) {
	case int64:
		lit, ok := expr.(*ast.IntegerLiteral)
		if !ok {
			t.Fatalf("expected IntegerLiteral, got %T", expr)
		}
		if lit.Value != v {
			t.Errorf("expected value %d, got %d", v, lit.Value)
		}
	case string:
		ident, ok := expr.(*ast.Identifier)
		if !ok {
			t.Fatalf("expected Identifier, got %T", expr)
		}
		if ident.Value != v {
			t.Errorf("expected value %q, got %q", v, ident.Value)
		}
	case bool:
		boolV, ok := expr.(*ast.Boolean)
		if !ok {
			t.Fatalf("expected Boolean, got %T", expr)
		}
		if boolV.Value != v {
			t.Errorf("expected value %v, got %v", v, boolV.Value)
		}
	default:
		t.Fatalf("unhandled literal type %T", expected)
	}
}
