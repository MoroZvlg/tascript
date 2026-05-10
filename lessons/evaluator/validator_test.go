package evaluator_test

import (
	"testing"

	"github.com/MoroZvlg/tascript/evaluator"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/object"
	"github.com/MoroZvlg/tascript/parser"
)

// rootScope returns a scope pre-seeded with `candles` as a CandleSeries —
// matches what the host (REPL/script-runner) will do before calling Validate.
func rootScope() *object.Scope {
	sc := object.NewScope()
	sc.Set("candles", object.CandleSeriesKind)
	return sc
}

func runValidate(t *testing.T, input string) []string {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser errors: %v", errs)
	}
	v := evaluator.NewValidator()
	v.Validate(prog, rootScope())
	return v.Errors()
}

func TestValidate_DirectCandlesArg(t *testing.T) {
	errs := runValidate(t, `sma(candles, 14)`)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidate_AliasedCandles(t *testing.T) {
	errs := runValidate(t, `let cs = candles; sma(cs, 14)`)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidate_TransitiveAlias(t *testing.T) {
	errs := runValidate(t, `let a = candles; let b = a; sma(b, 14)`)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidate_LiteralFirstArg(t *testing.T) {
	errs := runValidate(t, `sma(5, 14)`)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d (%v)", len(errs), errs)
	}
	want := "builtin func sma expects candle series as first argument, got any"
	if errs[0] != want {
		t.Errorf("expected %q, got %q", want, errs[0])
	}
}

func TestValidate_NonCandlesIdentifier(t *testing.T) {
	errs := runValidate(t, `let cs = 42; sma(cs, 14)`)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d (%v)", len(errs), errs)
	}
	want := "builtin func sma expects candle series as first argument, got any"
	if errs[0] != want {
		t.Errorf("expected %q, got %q", want, errs[0])
	}
}

func TestValidate_MissingArg(t *testing.T) {
	errs := runValidate(t, `sma()`)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d (%v)", len(errs), errs)
	}
	want := "builtin func sma expects arguments but got none"
	if errs[0] != want {
		t.Errorf("expected %q, got %q", want, errs[0])
	}
}

func TestValidate_ScopedBindingInsideBlock(t *testing.T) {
	// cs is defined inside the if-block and used inside the same block — ok.
	errs := runValidate(t, `if (true) { let cs = candles; sma(cs, 14) }`)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidate_ScopedBindingLeaksOut(t *testing.T) {
	// cs is scoped to the if-block; outside the block it's gone, so this should error.
	errs := runValidate(t, `if (true) { let cs = candles }; sma(cs, 14)`)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d (%v)", len(errs), errs)
	}
	want := "builtin func sma expects candle series as first argument, got any"
	if errs[0] != want {
		t.Errorf("expected %q, got %q", want, errs[0])
	}
}

func TestValidate_BuriedInsideInfix(t *testing.T) {
	// Indicator call is buried inside an arithmetic expression — still must be checked.
	errs := runValidate(t, `let x = sma(5, 14) + 1`)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d (%v)", len(errs), errs)
	}
	want := "builtin func sma expects candle series as first argument, got any"
	if errs[0] != want {
		t.Errorf("expected %q, got %q", want, errs[0])
	}
}

func TestValidate_BuriedInsideIndex(t *testing.T) {
	errs := runValidate(t, `let x = sma(5, 14)[0]`)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d (%v)", len(errs), errs)
	}
	want := "builtin func sma expects candle series as first argument, got any"
	if errs[0] != want {
		t.Errorf("expected %q, got %q", want, errs[0])
	}
}

func TestValidate_FunctionParamFalsePositive(t *testing.T) {
	// Known limitation: function parameters have unknown kind. Even though
	// every caller passes `candles`, the validator can't see that — so it
	// reports a violation inside the body. Pinned here to keep the trade-off visible.
	errs := runValidate(t, `let f = function(x) { sma(x, 14) }`)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d (%v)", len(errs), errs)
	}
	want := "builtin func sma expects candle series as first argument, got any"
	if errs[0] != want {
		t.Errorf("expected %q, got %q", want, errs[0])
	}
}

func TestValidate_NonIndicatorCall(t *testing.T) {
	// signal() isn't an indicator — validator shouldn't complain about its args.
	errs := runValidate(t, `signal("buy")`)
	if len(errs) != 0 {
		t.Errorf("expected no errors for non-indicator call, got %v", errs)
	}
}

func TestValidate_MultipleViolationsReportedAtOnce(t *testing.T) {
	// Both calls are wrong; both should be reported, not just the first.
	errs := runValidate(t, `sma(5, 14); rsi("nope", 14)`)
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d (%v)", len(errs), errs)
	}
	wantSma := "builtin func sma expects candle series as first argument, got any"
	wantRsi := "builtin func rsi expects candle series as first argument, got any"
	if errs[0] != wantSma {
		t.Errorf("expected %q, got %q", wantSma, errs[0])
	}
	if errs[1] != wantRsi {
		t.Errorf("expected %q, got %q", wantRsi, errs[1])
	}
}

func TestValidate_EmptyProgram(t *testing.T) {
	errs := runValidate(t, ``)
	if len(errs) != 0 {
		t.Errorf("expected no errors on empty program, got %v", errs)
	}
}
