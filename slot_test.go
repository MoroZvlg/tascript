package tascript_test

import (
	"errors"
	"testing"

	"github.com/MoroZvlg/tascript"
	"github.com/MoroZvlg/tascript/registry"
)

const settingsSrc = `setting period: Integer = 14
setting scale: Float = 1.0
output out: Integer
state n: Integer = 0
function Run() {
state.n = state.n + setting.period
emit(out, state.n)
}`

func settingsProgram(t *testing.T) (*tascript.Executable, *recorder) {
	t.Helper()

	builder := newBuilder(t)
	reg := builder.Registry()
	err := reg.RegisterDeclKind(registry.DeclKind{
		Word:        "setting",
		Initializer: registry.InitializerOptional,
		Namespaced:  true,
	})
	if err != nil {
		t.Fatalf("register decl kind: %v", err)
	}

	program, diags, err := builder.Compile(settingsSrc)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(diags) > 0 {
		t.Fatalf("compile diagnostics: %v", diags)
	}

	out := &recorder{accepts: registry.IntegerID}
	if err := program.BindOutput("out", out); err != nil {
		t.Fatalf("bind output: %v", err)
	}
	return program, out
}

func mustSlot(t *testing.T, program *tascript.Executable, kind, name string) tascript.Slot {
	t.Helper()

	slot, ok := program.Slot(kind, name)
	if !ok {
		t.Fatalf("slot %s.%s not found", kind, name)
	}
	return slot
}

func TestExecutable_SlotEnumeration(t *testing.T) {
	program, _ := settingsProgram(t)

	type entry struct {
		kind string
		name string
		t    registry.TypeID
	}
	want := []entry{
		{"setting", "period", registry.IntegerID},
		{"setting", "scale", registry.FloatID},
		{"state", "n", registry.IntegerID},
	}

	slots := program.Slots()
	if len(slots) != len(want) {
		t.Fatalf("expected %d slots, got %d", len(want), len(slots))
	}
	for i, w := range want {
		got := entry{slots[i].Kind(), slots[i].Name(), slots[i].Type()}
		if got != w {
			t.Errorf("slot %d: got %+v, want %+v", i, got, w)
		}
	}

	if _, ok := program.Slot("setting", "nope"); ok {
		t.Error("lookup of an undeclared slot succeeded")
	}
	if _, ok := program.Slot("indicator", "period"); ok {
		t.Error("lookup under the wrong kind succeeded")
	}
}

func TestExecutable_SlotFillBeforeInitWins(t *testing.T) {
	program, out := settingsProgram(t)

	if err := mustSlot(t, program, "setting", "period").Set(registry.Integer(3)); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := program.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := program.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	if out.last() != registry.Integer(3) {
		t.Errorf("host fill must win over the initializer: got %v, want 3", out.last())
	}
}

func TestExecutable_SlotDefaultsToItsInitializer(t *testing.T) {
	program, out := settingsProgram(t)

	if err := program.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := program.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	if out.last() != registry.Integer(14) {
		t.Errorf("got %v, want 14", out.last())
	}
}

func TestExecutable_SlotSetBetweenTurns(t *testing.T) {
	program, out := settingsProgram(t)
	if err := program.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	// the quiet turn that opens Live: a restore lands after Init, before the first tick
	period := mustSlot(t, program, "setting", "period")
	if err := period.Set(registry.Integer(2)); err != nil {
		t.Fatalf("set before the first tick: %v", err)
	}
	if err := program.Run(); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if out.last() != registry.Integer(2) {
		t.Fatalf("first tick: got %v, want 2", out.last())
	}

	if err := period.Set(registry.Integer(10)); err != nil {
		t.Fatalf("set between ticks: %v", err)
	}
	if err := program.Run(); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if out.last() != registry.Integer(12) {
		t.Errorf("second tick: got %v, want 12", out.last())
	}
}

func TestExecutable_SlotGet(t *testing.T) {
	program, _ := settingsProgram(t)
	n := mustSlot(t, program, "state", "n")

	if _, err := n.Get(); !errors.Is(err, tascript.ErrSlotEmpty) {
		t.Errorf("before Init: got %v, want ErrSlotEmpty", err)
	}

	if err := program.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := program.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	value, err := n.Get()
	if err != nil {
		t.Fatalf("get after a tick: %v", err)
	}
	if value != registry.Integer(14) {
		t.Errorf("got %v, want 14", value)
	}
}

func TestExecutable_SlotSetTypeCheck(t *testing.T) {
	program, _ := settingsProgram(t)

	if err := mustSlot(t, program, "setting", "scale").Set(registry.Integer(3)); err != nil {
		t.Errorf("Integer into a Float slot must coerce: %v", err)
	}
	if err := mustSlot(t, program, "setting", "period").Set(registry.String("x")); err == nil {
		t.Error("String into an Integer slot must fail")
	}
}

func TestExecutable_SlotAccessAfterFailedInit(t *testing.T) {
	program := compile(t, "state y: Integer = 1\nstate x: Float = math.sqrt(-1.0)\nfunction Run() {\nstate.x\n}")
	if err := program.Init(); err == nil {
		t.Fatal("expected Init to fail on a trapping initializer")
	}

	value, err := mustSlot(t, program, "state", "y").Get()
	if err != nil {
		t.Fatalf("get after a failed Init: %v", err)
	}
	if value != registry.Integer(1) {
		t.Errorf("got %v, want 1", value)
	}

	if err := mustSlot(t, program, "state", "y").Set(registry.Integer(2)); !errors.Is(err, tascript.ErrSetTooLate) {
		t.Errorf("set after a failed Init: got %v, want ErrSetTooLate", err)
	}
}

type callbackSink struct {
	onEmit func()
}

func (s *callbackSink) Emit(registry.Value)     { s.onEmit() }
func (s *callbackSink) TypeID() registry.TypeID { return registry.IntegerID }

func TestExecutable_SlotSetIsRejectedMidActivation(t *testing.T) {
	program, _ := settingsProgram(t)
	slot := mustSlot(t, program, "setting", "period")

	var setErr, getErr error
	if err := program.BindOutput("out", &callbackSink{onEmit: func() {
		setErr = slot.Set(registry.Integer(99))
		_, getErr = slot.Get()
	}}); err != nil {
		t.Fatalf("bind output: %v", err)
	}

	if err := program.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := program.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !errors.Is(setErr, tascript.ErrMidActivation) {
		t.Errorf("Set inside Emit: got %v, want ErrMidActivation", setErr)
	}
	if !errors.Is(getErr, tascript.ErrMidActivation) {
		t.Errorf("Get inside Emit: got %v, want ErrMidActivation", getErr)
	}
}

func TestExecutable_RunIsRejectedMidActivation(t *testing.T) {
	program, _ := settingsProgram(t)

	var runErr error
	if err := program.BindOutput("out", &callbackSink{onEmit: func() {
		runErr = program.Run()
	}}); err != nil {
		t.Fatalf("bind output: %v", err)
	}

	if err := program.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := program.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !errors.Is(runErr, tascript.ErrMidActivation) {
		t.Errorf("Run inside Emit: got %v, want ErrMidActivation", runErr)
	}
	value, err := mustSlot(t, program, "state", "n").Get()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if value != registry.Integer(14) {
		t.Errorf("a rejected nested tick must not touch state: got %v, want 14", value)
	}
}

func TestExecutable_InitIsRejectedMidActivation(t *testing.T) {
	builder := newBuilder(t)
	reg := builder.Registry()
	host, err := reg.RegisterModule("host")
	if err != nil {
		t.Fatalf("register module: %v", err)
	}

	var program *tascript.Executable
	var initErr error
	err = reg.RegisterCall(host.TypeID(), "poke", registry.CallRule{
		EvalType: registry.IntegerID,
		EvalFn: func(registry.Value, map[string]registry.Value) (registry.Value, error) {
			initErr = program.Init()
			return registry.Integer(1), nil
		},
	})
	if err != nil {
		t.Fatalf("register call: %v", err)
	}

	prog, diags, err := builder.Compile("state n: Integer = host.poke()\noutput out: Integer\nfunction Run() {\nemit(out, state.n)\n}")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(diags) > 0 {
		t.Fatalf("compile diagnostics: %v", diags)
	}
	program = prog
	if err := program.BindOutput("out", &recorder{accepts: registry.IntegerID}); err != nil {
		t.Fatalf("bind output: %v", err)
	}

	if err := program.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if !errors.Is(initErr, tascript.ErrMidActivation) {
		t.Errorf("Init inside a host rule: got %v, want ErrMidActivation", initErr)
	}
}

func TestBuilder_RegisterDeclKindAfterCompile(t *testing.T) {
	builder := newBuilder(t)
	reg := builder.Registry()
	if _, _, err := builder.Compile(counterSrc); err != nil {
		t.Fatalf("compile: %v", err)
	}

	err := reg.RegisterDeclKind(registry.DeclKind{Word: "setting"})
	if !errors.Is(err, registry.ErrSealed) {
		t.Errorf("got %v, want ErrSealed", err)
	}
}
