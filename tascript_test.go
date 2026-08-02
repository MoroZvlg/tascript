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

func TestExecutable_RunBeforeInit(t *testing.T) {
	program := compile(t, counterSrc)

	if program.Stage() != tascript.StageCreated {
		t.Errorf("fresh executable stage: got %s, want created", program.Stage())
	}

	_, err := program.Run(nil)
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
		got, err := program.Run(nil)
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

	got, err := program.Run(nil)
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

	_, err := program.Run(nil)
	if !errors.Is(err, tascript.ErrNotInitialized) {
		t.Errorf("Run after failed Init: got %v, want ErrNotInitialized", err)
	}

	err = program.Init()
	if !errors.Is(err, tascript.ErrInitRepeated) {
		t.Errorf("retrying a failed Init: got %v, want ErrInitRepeated", err)
	}
}
