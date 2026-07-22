package evaluator_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/evaluator"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/parser"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/resolver"
)

func testRegistry() *registry.Registry {
	reg := registry.DefaultRegistry()
	module, _ := reg.RegisterModule("t")
	reg.RegisterMemberAccess(module.TypeID(), "answer", registry.MemberAccessRule{
		EvalType: registry.IntegerID,
		EvalFn:   func(registry.Value) (registry.Value, error) { return registry.Integer(42), nil },
	})
	reg.RegisterCall(module.TypeID(), "double", registry.CallRule{
		Args:     []registry.ParamRule{{Type: registry.FloatID, Name: "n"}},
		EvalType: registry.FloatID,
		EvalFn: func(_ registry.Value, args map[string]registry.Value) (registry.Value, error) {
			return args["n"].(registry.Float) * 2, nil
		},
	})
	return reg
}

// compileSrc parses and resolves src into a fresh evaluator;
// customize (may be nil) tweaks the registry before resolution, e.g. to add test modules.
func compileSrc(t *testing.T, src string, customize func(*registry.Registry)) *evaluator.Evaluator {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.Parse()
	if len(p.Diagnostics()) > 0 {
		t.Fatalf("parser diagnostics: %v", p.Diagnostics())
	}
	reg := testRegistry()
	if customize != nil {
		customize(reg)
	}
	resolv := resolver.New(prog, reg)
	resolvedProg := resolv.Resolve()
	if len(resolv.Diagnostics()) > 0 {
		t.Fatalf("resolver diagnostics: %v", resolv.Diagnostics())
	}
	return evaluator.New(resolvedProg, reg)
}

func evalSrc(t *testing.T, src string) registry.Value {
	t.Helper()
	ev := compileSrc(t, src, nil)
	if _, err := ev.EvalInit(); err != nil {
		t.Fatalf("init error: %v", err)
	}
	got, err := ev.EvalRun()
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	return got
}

func evalRunBody(t *testing.T, body string) registry.Value {
	t.Helper()
	return evalSrc(t, "function Run() {\n"+body+"\n}")
}

func TestEvaluator_Emit(t *testing.T) {
	src := `output alert: String
output sig: {dir: String, price: Float}
function Run() {
let threshold = 1.0
emit(alert, "breakout")
emit(sig, dir="up", price=threshold + 0.2)
1
}`
	p := parser.New(lexer.New(src))
	prog := p.Parse()
	if len(p.Diagnostics()) > 0 {
		t.Fatalf("parser diagnostics: %v", p.Diagnostics())
	}
	reg := registry.DefaultRegistry()
	resolv := resolver.New(prog, reg)
	resolvedProg := resolv.Resolve()
	if len(resolv.Diagnostics()) > 0 {
		t.Fatalf("resolver diagnostics: %v", resolv.Diagnostics())
	}

	ev := evaluator.New(resolvedProg, reg)
	if _, err := ev.EvalRun(); err != nil {
		t.Fatalf("eval error: %v", err)
	}

	emitted := ev.Emitted()
	if len(emitted) != 2 {
		t.Fatalf("expected 2 emissions, got %d: %v", len(emitted), emitted)
	}

	if emitted[0].Name != "alert" {
		t.Errorf("emission[0]: expected output alert, got %s", emitted[0].Name)
	}
	if emitted[0].Value != registry.String("breakout") {
		t.Errorf("emission[0]: expected \"breakout\", got %v", emitted[0].Value)
	}

	if emitted[1].Name != "sig" {
		t.Errorf("emission[1]: expected output sig, got %s", emitted[1].Name)
	}
	rec, ok := emitted[1].Value.(registry.Record)
	if !ok {
		t.Fatalf("emission[1]: expected registry.Record, got %T", emitted[1].Value)
	}
	if rec.TypeID().String() != "output.sig" {
		t.Errorf("emission[1]: expected type output.sig, got %s", rec.TypeID())
	}
	if rec.Fields["dir"] != registry.String("up") {
		t.Errorf("emission[1]: expected dir \"up\", got %v", rec.Fields["dir"])
	}
	if rec.Fields["price"] != registry.Float(1.2) {
		t.Errorf("emission[1]: expected price 1.2, got %v", rec.Fields["price"])
	}

	// emissions must not accumulate across runs
	if _, err := ev.EvalRun(); err != nil {
		t.Fatalf("second eval error: %v", err)
	}
	if got := len(ev.Emitted()); got != 2 {
		t.Errorf("expected 2 emissions after second run, got %d", got)
	}
}

func TestEvaluator_StdOps(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want registry.Value
	}{
		{"string concat", `"foo" + "bar"`, registry.String("foobar")},
		{"int negation", `-5`, registry.Integer(-5)},
		{"float negation", `-2.5`, registry.Float(-2.5)},
		{"bool not", `!true`, registry.Bool(false)},
		{"double negation", `--5`, registry.Integer(5)},
		{"member access", `t.answer`, registry.Integer(42)},
		{"method call", `t.double(1.5)`, registry.Float(3)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalRunBody(t, tt.expr)
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluator_Arithmetic(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want registry.Value
	}{
		{"int addition", `1 + 2`, registry.Integer(3)},
		{"int subtraction", `1 - 2`, registry.Integer(-1)},
		{"int multiplication", `6 * 7`, registry.Integer(42)},
		{"int modulo", `7 % 3`, registry.Integer(1)},
		{"int division is float", `1 / 2`, registry.Float(0.5)},
		{"mixed int float", `1 + 2.5`, registry.Float(3.5)},
		{"mixed float int", `2.5 * 2`, registry.Float(5)},
		{"float modulo", `7.5 % 2`, registry.Float(1.5)},
		{"precedence", `2 + 3 * 4`, registry.Integer(14)},
		{"grouping", `(2 + 3) * 4`, registry.Integer(20)},
		{"negation in expression", `-2 * -3`, registry.Integer(6)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalRunBody(t, tt.expr)
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluator_Comparisons(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want registry.Value
	}{
		{"int less", `1 < 2`, registry.Bool(true)},
		{"int greater false", `1 > 2`, registry.Bool(false)},
		{"int equal", `2 == 2`, registry.Bool(true)},
		{"int not equal", `2 != 2`, registry.Bool(false)},
		{"lteq boundary", `2 <= 2`, registry.Bool(true)},
		{"gteq boundary", `3 >= 4`, registry.Bool(false)},
		// int/float mix must compare numerically, not by dynamic type
		{"int equals float", `1 == 1.0`, registry.Bool(true)},
		{"float less than int", `1.5 < 2`, registry.Bool(true)},
		{"string equality", `"a" == "a"`, registry.Bool(true)},
		{"string inequality", `"a" != "b"`, registry.Bool(true)},
		{"bool equality", `true == true`, registry.Bool(true)},
		{"comparison chain via bool", `true == (2 > 1)`, registry.Bool(true)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalRunBody(t, tt.expr)
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluator_Logical(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want registry.Value
	}{
		{"and both true", `true && true`, registry.Bool(true)},
		{"and short-circuits false", `false && true`, registry.Bool(false)},
		{"and right false", `true && false`, registry.Bool(false)},
		{"or both false", `false || false`, registry.Bool(false)},
		{"or short-circuits true", `true || false`, registry.Bool(true)},
		{"or right true", `false || true`, registry.Bool(true)},
		{"chained and", `true && true && false`, registry.Bool(false)},
		{"mixed precedence", `1 > 0 && 2 > 3`, registry.Bool(false)},
		{"or binds looser than and", `false && true || true`, registry.Bool(true)},
		{"grouping overrides precedence", `false && (true || true)`, registry.Bool(false)},
		{"and skips trapping rhs", `false && (1 / 0 > 0)`, registry.Bool(false)},
		{"or skips trapping rhs", `true || (1 / 0 > 0)`, registry.Bool(true)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalRunBody(t, tt.expr)
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluator_Statements(t *testing.T) {
	tests := []struct {
		name string
		body string
		want registry.Value
	}{
		{"let then read", "let x = 2\nx * 3", registry.Integer(6)},
		{"assign overwrites", "let x = 1\nx = x + 41\nx", registry.Integer(42)},
		{"two lets", "let a = 2\nlet b = 3\na * b", registry.Integer(6)},
		{"let from expression", "let x = t.double(1.0) + 1\nx", registry.Float(3)},
		{"let value is coerced", "let x = 1\nx + 0.5", registry.Float(1.5)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalRunBody(t, tt.body)
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluator_IfStmt(t *testing.T) {
	tests := []struct {
		name string
		body string
		want registry.Value
	}{
		{"true takes consequence", "let x = 1\nif (x > 0) {\nx = 2\n}\nx", registry.Integer(2)},
		{"false skips consequence", "let x = 1\nif (x < 0) {\nx = 2\n}\nx", registry.Integer(1)},
		{"false takes else", "let x = 1\nif (x < 0) {\nx = 2\n} else {\nx = 3\n}\nx", registry.Integer(3)},
		{"else if chain", "let x = 0\nlet r = \"\"\nif (x > 0) {\nr = \"pos\"\n} else if (x == 0) {\nr = \"zero\"\n} else {\nr = \"neg\"\n}\nr", registry.String("zero")},
		{"final else in chain", "let x = -1\nlet r = \"\"\nif (x > 0) {\nr = \"pos\"\n} else if (x == 0) {\nr = \"zero\"\n} else {\nr = \"neg\"\n}\nr", registry.String("neg")},
		{"if yields branch value", "if (true) {\n42\n}", registry.Integer(42)},
		{"untaken if yields nothing", "if (false) {\n42\n}", nil},
		{"branch let shadows outer", "let x = 1\nif (true) {\nlet x = 2\nx = x + 1\n}\nx", registry.Integer(1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalRunBody(t, tt.body)
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluator_TrapDivisionByZero(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{"int div", "7 / 0"},
		{"int mod", "5 % 0"},
		{"float div", "7.5 / 0.0"},
		{"float mod", "7.5 % 0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := compileSrc(t, "function Run() {\n"+tt.expr+"\n}", nil)
			_, err := ev.EvalRun()

			var failure diag.RuntimeFailure
			if !errors.As(err, &failure) {
				t.Fatalf("expected diag.RuntimeFailure, got %T: %v", err, err)
			}
			if failure.Kind != registry.DivisionByZero {
				t.Errorf("kind = %s, want %s", failure.Kind, registry.DivisionByZero)
			}
			if failure.Pos.Line != 2 {
				t.Errorf("position = %s, want line 2", failure.Pos.String())
			}
			if failure.EntryFn != "Run" {
				t.Errorf("entry fn = %s, want Run", failure.EntryFn)
			}
		})
	}
}

// A trapped tick never happened: its state writes roll back and its emits are discarded.
func TestEvaluator_TrapRollsBackTick(t *testing.T) {
	src := `output out: Integer
state n: Integer = 0

function Run() {
emit(out, state.n)
state.n = state.n + 1
if (state.n == 2) {
5 % 0
}
emit(out, state.n)
}`
	ev := compileSrc(t, src, nil)
	if _, err := ev.EvalInit(); err != nil {
		t.Fatalf("init error: %v", err)
	}

	// tick 1 commits: n 0 -> 1, emits 0 and 1
	if _, err := ev.EvalRun(); err != nil {
		t.Fatalf("first run error: %v", err)
	}
	emitted := ev.Emitted()
	if len(emitted) != 2 || emitted[0].Value != registry.Integer(0) || emitted[1].Value != registry.Integer(1) {
		t.Fatalf("first run: expected emissions [0 1], got %v", emitted)
	}

	// tick 2 traps mid-way: the emit before the trap must not leak
	if _, err := ev.EvalRun(); err == nil {
		t.Fatal("second run: expected trap")
	}
	if emitted := ev.Emitted(); len(emitted) != 0 {
		t.Errorf("second run: expected no emissions after trap, got %v", emitted)
	}

	// tick 3 proves n rolled back to 1: it traps identically. Without rollback n would be 2 and this tick would commit.
	if _, err := ev.EvalRun(); err == nil {
		t.Error("third run: expected trap again — state write leaked from the aborted tick")
	}
}

// A rule that panics (here a host rule violating the "return errors, don't
// panic" contract) surfaces as InternalFailure via the recover net — never as
// a script trap, and never crashing the host.
func TestEvaluator_PanicReportsInternalFailure(t *testing.T) {
	src := "function Run() {\nboom.explode(number=1.0)\n}"
	ev := compileSrc(t, src, func(reg *registry.Registry) {
		module, _ := reg.RegisterModule("boom")
		reg.RegisterCall(module.TypeID(), "explode", registry.CallRule{
			Args:     []registry.ParamRule{{Type: registry.FloatID, Name: "number"}},
			EvalType: registry.FloatID,
			EvalFn: func(registry.Value, map[string]registry.Value) (registry.Value, error) {
				panic("rule contract violated")
			},
		})
	})
	_, err := ev.EvalRun()

	var internal diag.InternalFailure
	if !errors.As(err, &internal) {
		t.Fatalf("expected diag.InternalFailure, got %T: %v", err, err)
	}
	if internal.EntryFn != "Run" {
		t.Errorf("entry fn = %s, want Run", internal.EntryFn)
	}
	if !strings.Contains(err.Error(), "rule contract violated") {
		t.Errorf("expected panic message in diagnostic, got: %v", err)
	}
}

// Host-written rules may return plain errors; they trap with UnknownKind instead of being lost or panicking.
func TestEvaluator_HostRuleErrorBecomesTrap(t *testing.T) {
	src := "function Run() {\nhost.fail(number=1.0)\n}"
	ev := compileSrc(t, src, func(reg *registry.Registry) {
		module, _ := reg.RegisterModule("host")
		reg.RegisterCall(module.TypeID(), "fail", registry.CallRule{
			Args:     []registry.ParamRule{{Type: registry.FloatID, Name: "number"}},
			EvalType: registry.FloatID,
			EvalFn: func(registry.Value, map[string]registry.Value) (registry.Value, error) {
				return nil, errors.New("host storage unavailable")
			},
		})
	})
	_, err := ev.EvalRun()

	var failure diag.RuntimeFailure
	if !errors.As(err, &failure) {
		t.Fatalf("expected diag.RuntimeFailure, got %T: %v", err, err)
	}
	if failure.Kind != registry.UnknownKind {
		t.Errorf("kind = %s, want %s", failure.Kind, registry.UnknownKind)
	}
	if failure.Message != "host storage unavailable" {
		t.Errorf("message = %q, want the host error text", failure.Message)
	}
}

func TestEvaluator_Consts(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want registry.Value
	}{
		{
			"const read in run",
			"const K = 2\nfunction Run() {\nK + 1\n}",
			registry.Integer(3),
		},
		{
			"const from const",
			"const A = 2\nconst B = A * 3\nfunction Run() {\nB\n}",
			registry.Integer(6),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalSrc(t, tt.src)
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluator_State(t *testing.T) {
	src := `output out: Integer
state cooldown: Integer = 3
state seeded: Integer

function Init() {
state.seeded = 10
}

function Run() {
state.cooldown = state.cooldown - 1
emit(out, state.cooldown + state.seeded)
1
}`
	p := parser.New(lexer.New(src))
	prog := p.Parse()
	if len(p.Diagnostics()) > 0 {
		t.Fatalf("parser diagnostics: %v", p.Diagnostics())
	}
	reg := registry.DefaultRegistry()
	resolv := resolver.New(prog, reg)
	resolvedProg := resolv.Resolve()
	if len(resolv.Diagnostics()) > 0 {
		t.Fatalf("resolver diagnostics: %v", resolv.Diagnostics())
	}

	ev := evaluator.New(resolvedProg, reg)
	if _, err := ev.EvalInit(); err != nil {
		t.Fatalf("init error: %v", err)
	}

	// bar 1: cooldown 3 -> 2, emits 2 + 10
	if _, err := ev.EvalRun(); err != nil {
		t.Fatalf("first eval error: %v", err)
	}
	emitted := ev.Emitted()
	if len(emitted) != 1 {
		t.Fatalf("expected 1 emission, got %d: %v", len(emitted), emitted)
	}
	if emitted[0].Value != registry.Integer(12) {
		t.Errorf("first run: expected 12, got %v", emitted[0].Value)
	}

	// bar 2: cooldown persists, 2 -> 1, emits 1 + 10
	if _, err := ev.EvalRun(); err != nil {
		t.Fatalf("second eval error: %v", err)
	}
	emitted = ev.Emitted()
	if len(emitted) != 1 {
		t.Fatalf("expected 1 emission on second run, got %d", len(emitted))
	}
	if emitted[0].Value != registry.Integer(11) {
		t.Errorf("second run: expected 11, got %v", emitted[0].Value)
	}
}
