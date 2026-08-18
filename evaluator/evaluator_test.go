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
	registerStateKind(reg)
	resolv := resolver.New(prog, reg)
	resolvedProg := resolv.Resolve()
	if len(resolv.Diagnostics()) > 0 {
		t.Fatalf("resolver diagnostics: %v", resolv.Diagnostics())
	}

	ev := evaluator.New(resolvedProg, reg)
	alerts, sigs := bindRecorder(t, ev, "alert", registry.StringID), bindRecorder(t, ev, "sig", registry.NewTypeID("output.sig"))
	if _, err := ev.EvalRun(); err != nil {
		t.Fatalf("eval error: %v", err)
	}

	if len(alerts.values) != 1 || alerts.values[0] != registry.String("breakout") {
		t.Errorf("alert: got %v, want [breakout]", alerts.values)
	}

	if len(sigs.values) != 1 {
		t.Fatalf("sig: expected 1 emission, got %v", sigs.values)
	}
	rec, ok := sigs.values[0].(registry.Record)
	if !ok {
		t.Fatalf("sig: expected registry.Record, got %T", sigs.values[0])
	}
	if rec.TypeID().String() != "output.sig" {
		t.Errorf("sig: expected type output.sig, got %s", rec.TypeID())
	}
	if rec.Fields["dir"] != registry.String("up") {
		t.Errorf("sig: expected dir \"up\", got %v", rec.Fields["dir"])
	}
	if rec.Fields["price"] != registry.Float(1.2) {
		t.Errorf("sig: expected price 1.2, got %v", rec.Fields["price"])
	}

	if _, err := ev.EvalRun(); err != nil {
		t.Fatalf("second eval error: %v", err)
	}
	if len(alerts.values) != 2 || len(sigs.values) != 2 {
		t.Errorf("second run: got %d alerts and %d sigs, want 2 each", len(alerts.values), len(sigs.values))
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
		{"assigned value is coerced", "let x = 1.5\nx = 1\nx", registry.Float(1)},
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
		{"branch let stays in its scope", "let x = 1\nif (true) {\nlet y = 2\nx = x + y\n}\nx", registry.Integer(3)},
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
			if failure.At.Line != 2 {
				t.Errorf("position = %s, want line 2", failure.At.String())
			}
		})
	}
}

// An aborted tick is unfinished, not undone: slot writes before the trap persist.
func TestEvaluator_TrapLeavesEarlierSlotWrites(t *testing.T) {
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
	out := bindRecorder(t, ev, "out", registry.IntegerID)
	if _, err := ev.EvalInit(); err != nil {
		t.Fatalf("init error: %v", err)
	}

	// tick 1 commits: n 0 -> 1, emits 0 and 1
	if _, err := ev.EvalRun(); err != nil {
		t.Fatalf("first run error: %v", err)
	}
	if len(out.values) != 2 || out.values[0] != registry.Integer(0) || out.values[1] != registry.Integer(1) {
		t.Fatalf("first run: expected emissions [0 1], got %v", out.values)
	}

	// tick 2 traps after its first emit, which the sink keeps
	if _, err := ev.EvalRun(); err == nil {
		t.Fatal("second run: expected trap")
	}
	if len(out.values) != 3 || out.values[2] != registry.Integer(1) {
		t.Errorf("second run: expected the pre-trap emit to survive, got %v", out.values)
	}

	// tick 3 sees n = 2, the value the aborted tick wrote before trapping, and commits
	if _, err := ev.EvalRun(); err != nil {
		t.Fatalf("third run error: %v", err)
	}
	if len(out.values) != 5 || out.values[3] != registry.Integer(2) || out.values[4] != registry.Integer(3) {
		t.Errorf("third run: expected emissions [2 3] on top, got %v", out.values)
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
	registerStateKind(reg)
	resolv := resolver.New(prog, reg)
	resolvedProg := resolv.Resolve()
	if len(resolv.Diagnostics()) > 0 {
		t.Fatalf("resolver diagnostics: %v", resolv.Diagnostics())
	}

	ev := evaluator.New(resolvedProg, reg)
	out := bindRecorder(t, ev, "out", registry.IntegerID)
	if _, err := ev.EvalInit(); err != nil {
		t.Fatalf("init error: %v", err)
	}

	// bar 1: cooldown 3 -> 2, emits 2 + 10
	if _, err := ev.EvalRun(); err != nil {
		t.Fatalf("first eval error: %v", err)
	}
	if len(out.values) != 1 || out.values[0] != registry.Integer(12) {
		t.Fatalf("first run: expected [12], got %v", out.values)
	}

	// bar 2: cooldown persists, 2 -> 1, emits 1 + 10
	if _, err := ev.EvalRun(); err != nil {
		t.Fatalf("second eval error: %v", err)
	}
	if len(out.values) != 2 || out.values[1] != registry.Integer(11) {
		t.Errorf("second run: expected [12 11], got %v", out.values)
	}
}

func TestEvaluator_LoadPhase(t *testing.T) {
	var calls int
	withCounter := func(reg *registry.Registry) {
		module, _ := reg.RegisterModule("load")
		reg.RegisterMemberAccess(module.TypeID(), "next", registry.MemberAccessRule{
			EvalType: registry.IntegerID,
			EvalFn: func(registry.Value) (registry.Value, error) {
				calls++
				return registry.Integer(calls), nil
			},
		})
	}

	t.Run("Init writes persist into Run", func(t *testing.T) {
		src := "state seeded: Integer\n" +
			"function Init() {\nstate.seeded = 10\n}\n" +
			"function Run() {\nstate.seeded = state.seeded + 1\nstate.seeded\n}"
		ev := initedEval(t, src, nil)

		if got := mustRun(t, ev); got != registry.Integer(11) {
			t.Errorf("first run: got %v, want 11", got)
		}
		if got := mustRun(t, ev); got != registry.Integer(12) {
			t.Errorf("second run: got %v, want 12", got)
		}
	})

	t.Run("state initializer is evaluated at load", func(t *testing.T) {
		src := "state size: Integer = 2 * 3\nfunction Run() {\nstate.size\n}"
		if got := mustRun(t, initedEval(t, src, nil)); got != registry.Integer(6) {
			t.Errorf("got %v, want 6", got)
		}
	})

	t.Run("consts are evaluated once, not per tick", func(t *testing.T) {
		calls = 0
		src := "const K = load.next\nfunction Run() {\nK\n}"
		ev := initedEval(t, src, withCounter)

		for tick := 1; tick <= 3; tick++ {
			if got := mustRun(t, ev); got != registry.Integer(1) {
				t.Fatalf("tick %d: got %v, want 1", tick, got)
			}
		}
		if calls != 1 {
			t.Errorf("load.next evaluated %d times, want 1", calls)
		}
	})

	t.Run("Init body runs once", func(t *testing.T) {
		calls = 0
		src := "state stamp: Integer\n" +
			"function Init() {\nstate.stamp = load.next\n}\n" +
			"function Run() {\nstate.stamp\n}"
		ev := initedEval(t, src, withCounter)

		for tick := 1; tick <= 3; tick++ {
			if got := mustRun(t, ev); got != registry.Integer(1) {
				t.Fatalf("tick %d: got %v, want 1", tick, got)
			}
		}
		if calls != 1 {
			t.Errorf("load.next evaluated %d times, want 1", calls)
		}
	})
}

func TestEvaluator_UninitializedSlotFailsInit(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			"no initializer and no Init",
			"state x: Integer\nfunction Run() {\nstate.x\n}",
		},
		{
			"Init does not assign it",
			"state x: Integer\nstate y: Integer = 0\nfunction Init() {\nstate.y = 1\n}\nfunction Run() {\nstate.x\n}",
		},
		{
			"assigned only under a branch that does not run",
			"state x: Integer\nfunction Init() {\nif (false) {\nstate.x = 1\n}\n}\nfunction Run() {\nstate.x\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := compileSrc(t, tt.src, nil)
			_, err := ev.EvalInit()
			if err == nil {
				t.Fatal("expected Init to fail on the unfilled slot")
			}
			failure, ok := err.(diag.RuntimeFailure)
			if !ok {
				t.Fatalf("expected diag.RuntimeFailure, got %T: %v", err, err)
			}
			if failure.Kind != registry.UninitializedSlot {
				t.Errorf("kind = %s, want %s", failure.Kind, registry.UninitializedSlot)
			}
			if !strings.Contains(failure.Message, "state.x") {
				t.Errorf("message %q does not name the slot", failure.Message)
			}
		})
	}
}

func TestEvaluator_SlotHandles(t *testing.T) {
	const src = "state x: Integer = 1\nstate y: Integer\nfunction Init() {\nstate.y = 2\n}\nfunction Run() {\nstate.x + state.y\n}"

	t.Run("a host fill before Init wins over the initializer", func(t *testing.T) {
		ev := compileSrc(t, src, nil)
		if err := ev.SlotSet(0, registry.Integer(10)); err != nil {
			t.Fatalf("SlotSet: %v", err)
		}
		if _, err := ev.EvalInit(); err != nil {
			t.Fatalf("init error: %v", err)
		}
		if got := mustRun(t, ev); got != registry.Integer(12) {
			t.Errorf("got %v, want 12", got)
		}
	})

	t.Run("a host fill before Init satisfies Rule B", func(t *testing.T) {
		ev := compileSrc(t, "state x: Integer\nfunction Run() {\nstate.x\n}", nil)
		if err := ev.SlotSet(0, registry.Integer(7)); err != nil {
			t.Fatalf("SlotSet: %v", err)
		}
		if _, err := ev.EvalInit(); err != nil {
			t.Fatalf("init error: %v", err)
		}
		if got := mustRun(t, ev); got != registry.Integer(7) {
			t.Errorf("got %v, want 7", got)
		}
	})

	t.Run("Get reports an empty slot before Init", func(t *testing.T) {
		ev := compileSrc(t, src, nil)
		if _, err := ev.SlotGet(0); !errors.Is(err, evaluator.ErrSlotEmpty) {
			t.Errorf("SlotGet: got %v, want ErrSlotEmpty", err)
		}
	})

	t.Run("Set coerces into the declared type", func(t *testing.T) {
		ev := compileSrc(t, "state f: Float = 0.0\nfunction Run() {\nstate.f\n}", nil)
		if err := ev.SlotSet(0, registry.Integer(3)); err != nil {
			t.Fatalf("SlotSet: %v", err)
		}
		if _, err := ev.EvalInit(); err != nil {
			t.Fatalf("init error: %v", err)
		}
		if got := mustRun(t, ev); got != registry.Float(3) {
			t.Errorf("got %v, want 3", got)
		}
	})

	t.Run("Set rejects an unrelated type", func(t *testing.T) {
		ev := compileSrc(t, src, nil)
		err := ev.SlotSet(0, registry.String("nope"))
		if err == nil {
			t.Fatal("expected a type error")
		}
		if _, ok := err.(diag.SlotTypeMismatch); !ok {
			t.Fatalf("expected diag.SlotTypeMismatch, got %T: %v", err, err)
		}
	})

	t.Run("Set between ticks is visible to the next tick", func(t *testing.T) {
		ev := initedEval(t, src, nil)
		if got := mustRun(t, ev); got != registry.Integer(3) {
			t.Fatalf("first run: got %v, want 3", got)
		}
		if err := ev.SlotSet(0, registry.Integer(5)); err != nil {
			t.Fatalf("SlotSet: %v", err)
		}
		if got := mustRun(t, ev); got != registry.Integer(7) {
			t.Errorf("second run: got %v, want 7", got)
		}
	})

	t.Run("a sink calling back mid-tick is rejected", func(t *testing.T) {
		ev := compileSrc(t, "output out: Integer\nstate x: Integer = 1\nfunction Run() {\nemit(out, state.x)\n}", nil)
		var setErr error
		ev.BindOutput("out", sinkFn{
			accepts: registry.IntegerID,
			emit:    func(registry.Value) { setErr = ev.SlotSet(0, registry.Integer(9)) },
		})
		if _, err := ev.EvalInit(); err != nil {
			t.Fatalf("init error: %v", err)
		}
		mustRun(t, ev)
		if !errors.Is(setErr, evaluator.ErrMidActivation) {
			t.Errorf("SlotSet inside Emit: got %v, want ErrMidActivation", setErr)
		}
	})
}

type sinkFn struct {
	accepts registry.TypeID
	emit    func(registry.Value)
}

func (f sinkFn) Emit(value registry.Value) { f.emit(value) }
func (f sinkFn) TypeID() registry.TypeID   { return f.accepts }

func TestEvaluator_InputBinding(t *testing.T) {
	src := "input price: Float\nfunction Run() {\nprice\n}"

	t.Run("bound value reaches the body", func(t *testing.T) {
		ev := initedEval(t, src, nil)
		if err := ev.BindInput("price", registry.Float(1.5)); err != nil {
			t.Fatalf("bind error: %v", err)
		}
		got := mustRun(t, ev)
		if got != registry.Float(1.5) {
			t.Errorf("got %v, want 1.5", got)
		}
	})

	t.Run("a bound value holds across ticks until rebound", func(t *testing.T) {
		ev := initedEval(t, src, nil)
		if err := ev.BindInput("price", registry.Float(1)); err != nil {
			t.Fatal(err)
		}
		if got := mustRun(t, ev); got != registry.Float(1) {
			t.Fatalf("first tick: got %v, want 1", got)
		}
		if got := mustRun(t, ev); got != registry.Float(1) {
			t.Errorf("second tick without rebinding: got %v, want 1", got)
		}

		if err := ev.BindInput("price", registry.Float(2)); err != nil {
			t.Fatal(err)
		}
		if got := mustRun(t, ev); got != registry.Float(2) {
			t.Errorf("after rebinding: got %v, want 2", got)
		}
	})

	t.Run("Integer coerces to declared Float", func(t *testing.T) {
		ev := initedEval(t, src, nil)
		if err := ev.BindInput("price", registry.Integer(3)); err != nil {
			t.Fatalf("bind error: %v", err)
		}
		if got := mustRun(t, ev); got != registry.Float(3) {
			t.Errorf("got %v, want 3.0", got)
		}
	})

	t.Run("unbound input errors on Run", func(t *testing.T) {
		ev := initedEval(t, src, nil)
		_, err := ev.EvalRun()
		var missing diag.InputMissing
		if !errors.As(err, &missing) {
			t.Fatalf("expected diag.InputMissing, got %T: %v", err, err)
		}
		if missing.Name != "price" {
			t.Errorf("name = %s, want price", missing.Name)
		}
	})

	t.Run("type mismatch errors on bind", func(t *testing.T) {
		ev := initedEval(t, src, nil)
		err := ev.BindInput("price", registry.String("x"))
		var mismatch diag.InputTypeMismatch
		if !errors.As(err, &mismatch) {
			t.Fatalf("expected diag.InputTypeMismatch, got %T: %v", err, err)
		}
		if mismatch.Expected != registry.FloatID || mismatch.Got != registry.StringID {
			t.Errorf("expected %s got %s, want Float/String", mismatch.Expected, mismatch.Got)
		}
	})

	t.Run("undeclared input errors on bind", func(t *testing.T) {
		ev := initedEval(t, src, nil)
		err := ev.BindInput("volume", registry.Float(2))
		var unknown diag.InputUnknown
		if !errors.As(err, &unknown) {
			t.Fatalf("expected diag.InputUnknown, got %T: %v", err, err)
		}
		if unknown.Name != "volume" {
			t.Errorf("name = %s, want volume", unknown.Name)
		}
	})

	t.Run("a rejected bind leaves the input unbound", func(t *testing.T) {
		ev := initedEval(t, src, nil)
		if err := ev.BindInput("price", registry.String("x")); err == nil {
			t.Fatal("expected a type mismatch")
		}
		_, err := ev.EvalRun()
		var missing diag.InputMissing
		if !errors.As(err, &missing) {
			t.Fatalf("expected diag.InputMissing, got %T: %v", err, err)
		}
	})
}
