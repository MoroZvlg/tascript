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
	return evaluator.Eval(prog)
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
