package tascript_test

import (
	"errors"
	"testing"

	"github.com/MoroZvlg/tascript"
	"github.com/MoroZvlg/tascript/registry"
)

const counterSrc = "state n: Integer = 0\nfunction Run() {\nstate.n = state.n + 1\nstate.n\n}"

func compile(t *testing.T, src string) *tascript.Executable {
	t.Helper()

	program, diags, err := tascript.NewBuilder().Compile(src)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(diags) > 0 {
		t.Fatalf("compile diagnostics: %v", diags)
	}
	return program
}

type box struct {
	id registry.TypeID
	v  float64
}

func (b *box) TypeID() registry.TypeID { return b.id }

func boxProgram(t *testing.T) (*tascript.Executable, registry.TypeID) {
	t.Helper()

	builder := tascript.NewBuilder()
	boxID, err := builder.RegisterType("Box")
	if err != nil {
		t.Fatalf("register type: %v", err)
	}

	err = builder.RegisterMemberAccess(boxID, "v", registry.MemberAccessRule{
		EvalType: registry.FloatID,
		EvalFn: func(receiver registry.Value) (registry.Value, error) {
			return registry.Float(receiver.(*box).v), nil
		},
	})
	if err != nil {
		t.Fatalf("register member: %v", err)
	}

	program, diags, err := builder.Compile("input b: Box\nfunction Run() {\nb.v\n}")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(diags) > 0 {
		t.Fatalf("compile diagnostics: %v", diags)
	}
	return program, boxID
}

func TestExecutable_BoundPointerStaysLive(t *testing.T) {
	program, boxID := boxProgram(t)

	bound := &box{id: boxID, v: 1}
	if err := program.BindInput("b", bound); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if err := program.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	got, err := program.Run()
	if err != nil {
		t.Fatalf("first tick: %v", err)
	}
	if got != registry.Float(1) {
		t.Fatalf("first tick: got %v, want 1", got)
	}

	bound.v = 2
	got, err = program.Run()
	if err != nil {
		t.Fatalf("second tick: %v", err)
	}
	if got != registry.Float(2) {
		t.Errorf("host mutation between ticks: got %v, want 2", got)
	}
}

func TestExecutable_BindAfterInit(t *testing.T) {
	program, boxID := boxProgram(t)

	if err := program.BindInput("b", &box{id: boxID}); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if err := program.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	err := program.BindInput("b", &box{id: boxID, v: 9})
	if !errors.Is(err, tascript.ErrBindTooLate) {
		t.Fatalf("bind after Init: got %v, want ErrBindTooLate", err)
	}
}

func TestExecutable_RunBeforeInit(t *testing.T) {
	program := compile(t, counterSrc)

	if program.Stage() != tascript.StageCreated {
		t.Errorf("fresh executable stage: got %s, want created", program.Stage())
	}

	_, err := program.Run()
	if !errors.Is(err, tascript.ErrNotInitialized) {
		t.Fatalf("Run before Init: got %v, want ErrNotInitialized", err)
	}
}

func TestExecutable_InitIsSingleShot(t *testing.T) {
	program := compile(t, counterSrc)

	if err := program.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if program.Stage() != tascript.StageInitialized {
		t.Errorf("stage after Init: got %s, want initialized", program.Stage())
	}

	var last registry.Value
	for tick := 1; tick <= 3; tick++ {
		got, err := program.Run()
		if err != nil {
			t.Fatalf("tick %d: %v", tick, err)
		}
		last = got
	}
	if last != registry.Integer(3) {
		t.Fatalf("after 3 ticks: got %v, want 3", last)
	}

	err := program.Init()
	if !errors.Is(err, tascript.ErrInitRepeated) {
		t.Fatalf("second Init: got %v, want ErrInitRepeated", err)
	}

	got, err := program.Run()
	if err != nil {
		t.Fatalf("tick after rejected Init: %v", err)
	}
	if got != registry.Integer(4) {
		t.Errorf("a rejected Init must not reseed state: got %v, want 4", got)
	}
}

func TestExecutable_FailedInitIsTerminal(t *testing.T) {
	program := compile(t, "state x: Float = math.sqrt(-1.0)\nfunction Run() {\nstate.x\n}")

	if err := program.Init(); err == nil {
		t.Fatal("expected Init to fail on a trapping state initializer")
	}
	if program.Stage() != tascript.StageFailed {
		t.Errorf("stage after failed Init: got %s, want failed", program.Stage())
	}

	_, err := program.Run()
	if !errors.Is(err, tascript.ErrNotInitialized) {
		t.Errorf("Run after failed Init: got %v, want ErrNotInitialized", err)
	}

	err = program.Init()
	if !errors.Is(err, tascript.ErrInitRepeated) {
		t.Errorf("retrying a failed Init: got %v, want ErrInitRepeated", err)
	}
}
