package parser_test

import (
	"fmt"
	"testing"

	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/parser"
)

func Test_ParseProgram_Let(t *testing.T) {
	input := `let x = 5;
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
