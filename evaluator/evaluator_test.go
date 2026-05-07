package evaluator_test

import (
	"bytes"
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
		{"let id = function(x) { x }; id(5)", "5"},
		{"const makeAdder = function(x) { function(y) { x + y } }; const addFive = makeAdder(5); addFive(10)", "15"},
		{"let fact = function(n) { if (n < 2) { 1 } else { n * fact(n - 1) } }; fact(5)", "120"},
		{"5(1,2)", "ERROR: not a function: 5"},
		{"let f = function(x) { x }; f(1, 2)", "ERROR: argument(s) number mismatch. expected 1 got 2"},
		{"const make = function(x) { function(y) { x + y } }; const a = make(5); const b = make(10); a(1)", "6"},
		{`"hi".length`, "2"},
		{`"".length`, "0"},
		{`"".wrong`, "ERROR: string has no property 'wrong'"},
		{`(5).length`, "ERROR: type int has no properties"},
		{"let f = function(x) { return x; }; f(5)", "5"},
		{"let f = function(x) { return x; x * 2 }; f(5)", "5"},
		{"let f = function(x) { if (x > 0) { return 1; } return -1; }; f(5)", "1"},
		{"let f = function(x) { if (x > 0) { if (x > 10) { return 1; } return 2; } return 3; }; f(5)", "2"},
		{"return 10;", "10"},
		{"let f = function() { return 5 + true; }; f()", "ERROR: type mismatch: int + boolean"},
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
		{"true && false", "false"},
		{"true || false", "true"},
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

func TestCandleSeriesMemberAccess(t *testing.T) {
	mk := func() *object.Environment {
		env := object.NewEnvironment()
		env.Set("candles", &object.CandleSeries{
			Value: []object.Candle{
				{Open: 1, High: 1.5, Low: 0.5, Close: 1.2, Volume: 100},
				{Open: 2, High: 2.5, Low: 1.5, Close: 2.2, Volume: 200},
				{Open: 3, High: 3.5, Low: 2.5, Close: 3.2, Volume: 300},
			},
		})
		return env
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"candles.opens.length", "3"},
		{"candles.highs.length", "3"},
		{"candles.lows.length", "3"},
		{"candles.closes.length", "3"},
		{"candles.volumes.length", "3"},
		{"candles.closes", "[3]"},
		{"candles.bogus", "ERROR: CandleSeries has no property 'bogus'"},
	}
	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := parser.New(l)
		prog := p.ParseProgram()
		if errs := p.Errors(); len(errs) > 0 {
			t.Fatalf("parser errors for %q: %v", tt.input, errs)
		}
		got := evaluator.Eval(prog, mk())
		if got.Inspect() != tt.expected {
			t.Errorf("input %q: expected %s, got %s", tt.input, tt.expected, got.Inspect())
		}
	}
}

func TestCandleMemberAccess(t *testing.T) {
	mk := func() *object.Environment {
		env := object.NewEnvironment()
		env.Set("c", &object.Candle{Open: 1, High: 1.5, Low: 0.5, Close: 1.2, Volume: 100})
		return env
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"c.open", "1"},
		{"c.high", "1.5"},
		{"c.low", "0.5"},
		{"c.close", "1.2"},
		{"c.volume", "100"},
		{"c.bogus", "ERROR: Candle has no property 'bogus'"},
	}
	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := parser.New(l)
		prog := p.ParseProgram()
		if errs := p.Errors(); len(errs) > 0 {
			t.Fatalf("parser errors for %q: %v", tt.input, errs)
		}
		got := evaluator.Eval(prog, mk())
		if got.Inspect() != tt.expected {
			t.Errorf("input %q: expected %s, got %s", tt.input, tt.expected, got.Inspect())
		}
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

func TestBuiltinDispatch(t *testing.T) {
	mk := func() *object.Environment {
		env := object.NewEnvironment()
		env.Set("double", &object.Builtin{
			Name: "double",
			Fn: func(args []object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: "double: expected 1 arg"}
				}
				n, ok := args[0].(*object.Integer)
				if !ok {
					return &object.Error{Message: "double: expected int"}
				}
				return &object.Integer{Value: n.Value * 2}
			},
		})
		env.Set("addTwo", &object.Builtin{
			Name: "addTwo",
			Fn: func(args []object.Object) object.Object {
				if len(args) != 2 {
					return &object.Error{Message: "addTwo: expected 2 args"}
				}
				a, ok := args[0].(*object.Integer)
				if !ok {
					return &object.Error{Message: "addTwo: arg 0 not int"}
				}
				b, ok := args[1].(*object.Integer)
				if !ok {
					return &object.Error{Message: "addTwo: arg 1 not int"}
				}
				return &object.Integer{Value: a.Value + b.Value}
			},
		})
		return env
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"double(5)", "10"},
		{"addTwo(3, 4)", "7"},
		{"double(double(3))", "12"},
		{"addTwo(double(2), double(3))", "10"},
		{"let f = double; f(7)", "14"},
		{`double`, "double"},
		{"double()", "ERROR: double: expected 1 arg"},
		{"double(1, 2)", "ERROR: double: expected 1 arg"},
		{"double(true)", "ERROR: double: expected int"},
		{"addTwo(1)", "ERROR: addTwo: expected 2 args"},
		{"let x = 5; x(1)", "ERROR: not a function: x"},
		{"double(unknownVar)", "ERROR: identifier not found: unknownVar"},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := parser.New(l)
		prog := p.ParseProgram()
		if errs := p.Errors(); len(errs) > 0 {
			t.Fatalf("parser errors for %q: %v", tt.input, errs)
		}
		got := evaluator.Eval(prog, mk())
		if got.Inspect() != tt.expected {
			t.Errorf("input %q: expected %s, got %s", tt.input, tt.expected, got.Inspect())
		}
	}
}

func TestIndicatorBuiltinsEndToEnd(t *testing.T) {
	mk := func() *object.Environment {
		env := object.NewEnvironment()
		evaluator.RegisterBuiltins(env)
		env.Set("candles", &object.CandleSeries{
			Value: []object.Candle{
				{Open: 1, High: 1, Low: 1, Close: 10, Volume: 1},
				{Open: 1, High: 1, Low: 1, Close: 20, Volume: 1},
				{Open: 1, High: 1, Low: 1, Close: 30, Volume: 1},
				{Open: 1, High: 1, Low: 1, Close: 40, Volume: 1},
				{Open: 1, High: 1, Low: 1, Close: 50, Volume: 1},
			},
		})
		return env
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"sma(candles, 3).length", "5"},
		{"ema(candles, 3).length", "5"},
		{"rsi(candles, 3).length", "5"},
		{"sma(candles, 3)", "[5]"},
		{"sma()", "ERROR: sma: wrong number of arguments. got=0, want=2"},
		{"sma(candles)", "ERROR: sma: wrong number of arguments. got=1, want=2"},
		{"sma(5, 3)", "ERROR: sma: first argument must be CandleSeries, got int"},
		{"sma(candles, candles)", "ERROR: sma: second argument must be Integer, got CandleSeries"},
	}
	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := parser.New(l)
		prog := p.ParseProgram()
		if errs := p.Errors(); len(errs) > 0 {
			t.Fatalf("parser errors for %q: %v", tt.input, errs)
		}
		got := evaluator.Eval(prog, mk())
		if got.Inspect() != tt.expected {
			t.Errorf("input %q: expected %s, got %s", tt.input, tt.expected, got.Inspect())
		}
	}
}

func TestSmaMathRoundTrip(t *testing.T) {
	env := object.NewEnvironment()
	evaluator.RegisterBuiltins(env)
	env.Set("candles", &object.CandleSeries{
		Value: []object.Candle{
			{Close: 10}, {Close: 20}, {Close: 30}, {Close: 40}, {Close: 50},
		},
	})
	l := lexer.New("sma(candles, 3)")
	p := parser.New(l)
	prog := p.ParseProgram()
	got := evaluator.Eval(prog, env)
	series, ok := got.(*object.Series)
	if !ok {
		t.Fatalf("expected *object.Series, got %T (%s)", got, got.Inspect())
	}
	// SMA(3) over [10,20,30,40,50]: warmup zeros for first 2 slots, then 20, 30, 40.
	want := []float64{0, 0, 20, 30, 40}
	if len(series.Value) != len(want) {
		t.Fatalf("expected len %d, got %d", len(want), len(series.Value))
	}
	for i, w := range want {
		if series.Value[i] != w {
			t.Errorf("slot %d: expected %g, got %g", i, w, series.Value[i])
		}
	}
}

func TestIndexExpressionOnSeries(t *testing.T) {
	mk := func() *object.Environment {
		env := object.NewEnvironment()
		env.Set("closes", &object.Series{Value: []float64{10, 20, 30}})
		env.Set("i", &object.Integer{Value: 1})
		return env
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"closes[0]", "30"},
		{"closes[1]", "20"},
		{"closes[2]", "10"},
		{"closes[i]", "20"},
		{"closes[i + 1]", "10"},
		{"closes[0] + closes[2]", "40"},
		{"-closes[0]", "-30"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			prog := p.ParseProgram()
			if errs := p.Errors(); len(errs) > 0 {
				t.Fatalf("parser errors: %v", errs)
			}
			got := evaluator.Eval(prog, mk())
			if got.Inspect() != tt.expected {
				t.Errorf("input %q: expected %s, got %s", tt.input, tt.expected, got.Inspect())
			}
		})
	}
}

func TestIndexExpressionOnCandleSeries(t *testing.T) {
	mk := func() *object.Environment {
		env := object.NewEnvironment()
		env.Set("candles", &object.CandleSeries{
			Value: []object.Candle{
				{Open: 1, High: 1.5, Low: 0.5, Close: 1.2, Volume: 100},
				{Open: 2, High: 2.5, Low: 1.5, Close: 2.2, Volume: 200},
				{Open: 3, High: 3.5, Low: 2.5, Close: 3.2, Volume: 300},
			},
		})
		return env
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"candles[0].open", "3"},
		{"candles[1].close", "2.2"},
		{"candles[2].volume", "100"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			prog := p.ParseProgram()
			if errs := p.Errors(); len(errs) > 0 {
				t.Fatalf("parser errors: %v", errs)
			}
			got := evaluator.Eval(prog, mk())
			if got.Inspect() != tt.expected {
				t.Errorf("input %q: expected %s, got %s", tt.input, tt.expected, got.Inspect())
			}
		})
	}
}

func TestIndexExpressionErrors(t *testing.T) {
	mk := func() *object.Environment {
		env := object.NewEnvironment()
		env.Set("closes", &object.Series{Value: []float64{10, 20, 30}})
		env.Set("candles", &object.CandleSeries{
			Value: []object.Candle{
				{Open: 1, High: 1.5, Low: 0.5, Close: 1.2, Volume: 100},
			},
		})
		return env
	}

	tests := []struct {
		input       string
		wantErrPart string
	}{
		{"closes[3]", "out of range"},
		{"closes[100]", "out of range"},
		{"closes[-1]", "index should be a positive integer, got -1"},
		{"candles[5]", "out of range"},
		{`closes["foo"]`, "integer"},
		{"let x = 5; x[0]", "indexable"},
		{"closes[5 + true]", "type mismatch"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			prog := p.ParseProgram()
			if errs := p.Errors(); len(errs) > 0 {
				t.Fatalf("parser errors: %v", errs)
			}
			got := evaluator.Eval(prog, mk())
			errObj, ok := got.(*object.Error)
			if !ok {
				t.Fatalf("input %q: expected *object.Error, got %T (%s)", tt.input, got, got.Inspect())
			}
			if !contains(errObj.Message, tt.wantErrPart) {
				t.Errorf("input %q: expected error containing %q, got %q", tt.input, tt.wantErrPart, errObj.Message)
			}
		})
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestSignalBuiltin(t *testing.T) {
	var buf bytes.Buffer
	prev := evaluator.SignalOutput
	evaluator.SignalOutput = &buf
	defer func() { evaluator.SignalOutput = prev }()

	env := object.NewEnvironment()
	evaluator.RegisterBuiltins(env)

	input := `signal("buy")`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser errors: %v", errs)
	}
	got := evaluator.Eval(prog, env)
	if got.Type() != object.NullKind {
		t.Errorf("expected NULL return, got %s (%s)", got.Type(), got.Inspect())
	}
	if buf.String() != "received signal: buy\n" {
		t.Errorf("expected log %q, got %q", "received signal: buy\n", buf.String())
	}
}

func TestSignalBuiltinErrors(t *testing.T) {
	var buf bytes.Buffer
	prev := evaluator.SignalOutput
	evaluator.SignalOutput = &buf
	defer func() { evaluator.SignalOutput = prev }()

	tests := []struct {
		input       string
		wantErrPart string
	}{
		{`signal()`, "wrong number of arguments"},
		{`signal("a", "b")`, "wrong number of arguments"},
		{`signal(5)`, "must be String"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			env := object.NewEnvironment()
			evaluator.RegisterBuiltins(env)
			l := lexer.New(tt.input)
			p := parser.New(l)
			prog := p.ParseProgram()
			if errs := p.Errors(); len(errs) > 0 {
				t.Fatalf("parser errors: %v", errs)
			}
			got := evaluator.Eval(prog, env)
			errObj, ok := got.(*object.Error)
			if !ok {
				t.Fatalf("expected *object.Error, got %T (%s)", got, got.Inspect())
			}
			if !contains(errObj.Message, tt.wantErrPart) {
				t.Errorf("expected error containing %q, got %q", tt.wantErrPart, errObj.Message)
			}
		})
	}
}

func TestBuiltinAndUserFunctionInterop(t *testing.T) {
	env := object.NewEnvironment()
	env.Set("triple", &object.Builtin{
		Name: "triple",
		Fn: func(args []object.Object) object.Object {
			n := args[0].(*object.Integer)
			return &object.Integer{Value: n.Value * 3}
		},
	})

	input := `
		let wrap = function(x) { triple(x) + 1 };
		wrap(4)
	`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser errors: %v", errs)
	}
	got := evaluator.Eval(prog, env)
	if got.Inspect() != "13" {
		t.Errorf("expected 13, got %s", got.Inspect())
	}
}
