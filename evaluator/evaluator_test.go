package evaluator_test

import (
	"testing"

	"github.com/MoroZvlg/tascript/evaluator"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/object"
	"github.com/MoroZvlg/tascript/parser"
)

func testEval(t *testing.T, input string) object.Object {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser errors for %q: %v", input, errs)
	}
	return evaluator.Eval(prog, object.NewEnvironment())
}

func TestEvalIntegerExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"5", "5"},
		{"10", "10"},
		{"-5", "-5"},
		{"-10", "-10"},
		{"5 + 5", "10"},
		{"5 + 5 * 2", "15"},
		{"(1 + 2) * 3", "9"},
		{"50 / 2 * 2 + 10", "60"},
		{"20 + 2 * -10", "0"},
		{"3 * (3 * 3) + 10", "37"},
	}
	for _, tt := range tests {
		got := testEval(t, tt.input)
		if got.Inspect() != tt.expected {
			t.Errorf("input %q: expected %s, got %s", tt.input, tt.expected, got.Inspect())
		}
	}
}

func TestEvalFloatExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1.5", "1.5"},
		{"-1.5", "-1.5"},
		{"1.5 + 2.5", "4"},
		{"2.0 * 3.5", "7"},
		{"1 + 1.5", "2.5"},
		{"2.5 - 1", "1.5"},
		{"10 / 4.0", "2.5"},
	}
	for _, tt := range tests {
		got := testEval(t, tt.input)
		if got.Inspect() != tt.expected {
			t.Errorf("input %q: expected %s, got %s", tt.input, tt.expected, got.Inspect())
		}
	}
}

func TestEvalBooleanExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"true", "true"},
		{"false", "false"},
		{"!true", "false"},
		{"!false", "true"},
		{"!!true", "true"},
		{"!5", "false"},
		{"1 < 2", "true"},
		{"1 > 2", "false"},
		{"1 == 1", "true"},
		{"1 != 1", "false"},
		{"1 <= 1", "true"},
		{"2 >= 1", "true"},
		{"true == true", "true"},
		{"true != false", "true"},
		{"(1 < 2) == true", "true"},
		{"(1 > 2) == false", "true"},
		{"1.5 < 2", "true"},
		{"2 == 2.0", "true"},
	}
	for _, tt := range tests {
		got := testEval(t, tt.input)
		if got.Inspect() != tt.expected {
			t.Errorf("input %q: expected %s, got %s", tt.input, tt.expected, got.Inspect())
		}
	}
}

func TestEvalStringExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"foo"`, "foo"},
		{`"foo" + "bar"`, "foobar"},
		{`"foo" == "foo"`, "true"},
		{`"foo" != "bar"`, "true"},
	}
	for _, tt := range tests {
		got := testEval(t, tt.input)
		if got.Inspect() != tt.expected {
			t.Errorf("input %q: expected %s, got %s", tt.input, tt.expected, got.Inspect())
		}
	}
}

func TestSingletonsNotMutated(t *testing.T) {
	_ = testEval(t, "!true")
	if evaluator.TRUE.Value != true {
		t.Fatalf("TRUE singleton was mutated: Value=%v", evaluator.TRUE.Value)
	}
	if evaluator.FALSE.Value != false {
		t.Fatalf("FALSE singleton was mutated: Value=%v", evaluator.FALSE.Value)
	}
	got := testEval(t, "true")
	if got != evaluator.TRUE {
		t.Fatalf("`true` did not evaluate to the TRUE singleton")
	}
}

func TestErrorHandling(t *testing.T) {
	tests := []struct {
		input       string
		expectedMsg string
	}{
		{"5 + true", "type mismatch: int + boolean"},
		{`"foo" - "bar"`, "unknown operator: string - string"},
		{"true + false", "unknown operator: boolean + boolean"},
		{"-true", "unknown operator: -boolean"},
		{"1 / 0", "division by zero"},
		{"1.0 / 0", "division by zero"},
		{"foobar", "identifier not found: foobar"},
	}
	for _, tt := range tests {
		got := testEval(t, tt.input)
		errObj, ok := got.(*object.Error)
		if !ok {
			t.Errorf("input %q: expected *object.Error, got %T (%s)", tt.input, got, got.Inspect())
			continue
		}
		if errObj.Message != tt.expectedMsg {
			t.Errorf("input %q: expected error %q, got %q", tt.input, tt.expectedMsg, errObj.Message)
		}
	}
}

func TestLetAndConstStatements(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"let x = 5; x", "5"},
		{"let x = 5 * 5; x", "25"},
		{"let x = 5; let y = x; y", "5"},
		{"let a = 5; let b = a; let c = a + b + 5; c", "15"},
		{"let x = 5; let y = x * 2; y", "10"},
		{"const k = 42; k", "42"},
		{"const a = 1; const b = 2; a + b", "3"},
		{`const greeting = "hello"; greeting + " world"`, "hello world"},
	}
	for _, tt := range tests {
		got := testEval(t, tt.input)
		if got.Inspect() != tt.expected {
			t.Errorf("input %q: expected %s, got %s", tt.input, tt.expected, got.Inspect())
		}
	}
}

func TestIfElseExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"if (true) { 10 }", "10"},
		{"if (false) { 10 }", "null"},
		{"if (1) { 10 }", "10"},
		{"if (1 < 2) { 10 }", "10"},
		{"if (1 > 2) { 10 }", "null"},
		{"if (1 < 2) { 10 } else { 20 }", "10"},
		{"if (1 > 2) { 10 } else { 20 }", "20"},
		{`if (1) { "yes" }`, "yes"},
		{`if (1 < 2) { "small" } else { "big" }`, "small"},
		{"let x = 10; if (x > 5) { x * 2 } else { x }", "20"},
		{"let x = 3; if (x > 5) { x * 2 } else { x }", "3"},
	}
	for _, tt := range tests {
		got := testEval(t, tt.input)
		if got.Inspect() != tt.expected {
			t.Errorf("input %q: expected %s, got %s", tt.input, tt.expected, got.Inspect())
		}
	}
}

func TestNestedScopeAccess(t *testing.T) {
	input := `
		let x = 1;
		let y = 2;
		if (x < y) {
			let z = x + y;
			z * 10
		} else {
			0
		}
	`
	got := testEval(t, input)
	if got.Inspect() != "30" {
		t.Errorf("expected 30, got %s", got.Inspect())
	}
}

func TestErrorPropagatesThroughLet(t *testing.T) {
	input := "let x = 5 + true; x"
	got := testEval(t, input)
	errObj, ok := got.(*object.Error)
	if !ok {
		t.Fatalf("expected *object.Error, got %T (%s)", got, got.Inspect())
	}
	if errObj.Message != "type mismatch: int + boolean" {
		t.Errorf("expected propagated type-mismatch error, got %q", errObj.Message)
	}
}
