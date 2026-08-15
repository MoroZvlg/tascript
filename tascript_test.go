package tascript_test

import (
	"errors"
	"testing"

	"github.com/MoroZvlg/tascript"
	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/registry"
)

func TestExecutable_Binding(t *testing.T) {
	t.Run("a bound pointer stays live across ticks", func(t *testing.T) {
		program, boxID, seen := boxProgram(t)

		bound := &box{id: boxID, v: 1}
		if err := program.BindInput("b", bound); err != nil {
			t.Fatalf("bind: %v", err)
		}
		if err := program.Init(); err != nil {
			t.Fatalf("init: %v", err)
		}

		if err := program.Run(); err != nil {
			t.Fatalf("first tick: %v", err)
		}
		if seen.last() != registry.Float(1) {
			t.Fatalf("first tick: got %v, want 1", seen.last())
		}

		bound.v = 2
		if err := program.Run(); err != nil {
			t.Fatalf("second tick: %v", err)
		}
		if seen.last() != registry.Float(2) {
			t.Errorf("host mutation between ticks: got %v, want 2", seen.last())
		}
	})

	t.Run("binding after Init is rejected", func(t *testing.T) {
		program, boxID, _ := boxProgram(t)

		if err := program.BindInput("b", &box{id: boxID}); err != nil {
			t.Fatalf("bind: %v", err)
		}
		if err := program.Init(); err != nil {
			t.Fatalf("init: %v", err)
		}

		err := program.BindInput("b", &box{id: boxID, v: 9})
		if !errors.Is(err, tascript.ErrBindTooLate) {
			t.Errorf("bind input after Init: got %v, want ErrBindTooLate", err)
		}

		err = program.BindOutput("seen", &recorder{})
		if !errors.Is(err, tascript.ErrBindTooLate) {
			t.Errorf("bind output after Init: got %v, want ErrBindTooLate", err)
		}
	})

	t.Run("binding an undeclared output errors", func(t *testing.T) {
		program := compile(t, counterSrc)

		err := program.BindOutput("nope", &recorder{})
		var unknown diag.OutputUnknown
		if !errors.As(err, &unknown) {
			t.Fatalf("got %T %v, want diag.OutputUnknown", err, err)
		}
		if unknown.Name != "nope" {
			t.Errorf("name = %s, want nope", unknown.Name)
		}
	})

	t.Run("an output left without a sink fails Init", func(t *testing.T) {
		program := compile(t, counterSrc)

		err := program.Init()
		var missing diag.OutputMissing
		if !errors.As(err, &missing) {
			t.Fatalf("Init with no sink: got %T %v, want diag.OutputMissing", err, err)
		}
		if missing.Name != "tick" {
			t.Errorf("name = %s, want tick", missing.Name)
		}
		if program.Stage() != tascript.StageFailed {
			t.Errorf("stage: got %s, want failed", program.Stage())
		}
	})
}

func TestExecutable_RunBeforeInit(t *testing.T) {
	program, _ := counterProgram(t)

	if program.Stage() != tascript.StageCreated {
		t.Errorf("fresh executable stage: got %s, want created", program.Stage())
	}

	err := program.Run()
	if !errors.Is(err, tascript.ErrNotInitialized) {
		t.Fatalf("Run before Init: got %v, want ErrNotInitialized", err)
	}
}

func TestExecutable_InitIsSingleShot(t *testing.T) {
	program, ticks := counterProgram(t)

	if err := program.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if program.Stage() != tascript.StageInitialized {
		t.Errorf("stage after Init: got %s, want initialized", program.Stage())
	}

	for tick := 1; tick <= 3; tick++ {
		if err := program.Run(); err != nil {
			t.Fatalf("tick %d: %v", tick, err)
		}
	}
	if ticks.last() != registry.Integer(3) {
		t.Fatalf("after 3 ticks: got %v, want 3", ticks.last())
	}

	err := program.Init()
	if !errors.Is(err, tascript.ErrInitRepeated) {
		t.Fatalf("second Init: got %v, want ErrInitRepeated", err)
	}

	if err := program.Run(); err != nil {
		t.Fatalf("tick after rejected Init: %v", err)
	}
	if ticks.last() != registry.Integer(4) {
		t.Errorf("a rejected Init must not reseed state: got %v, want 4", ticks.last())
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

	err := program.Run()
	if !errors.Is(err, tascript.ErrNotInitialized) {
		t.Errorf("Run after failed Init: got %v, want ErrNotInitialized", err)
	}

	err = program.Init()
	if !errors.Is(err, tascript.ErrInitRepeated) {
		t.Errorf("retrying a failed Init: got %v, want ErrInitRepeated", err)
	}
}

func TestBuilder_Compile(t *testing.T) {
	t.Run("script diagnostics are not errors", func(t *testing.T) {
		program, diags, err := tascript.NewBuilder().Compile("function Run() {\nnope\n}")
		if err != nil {
			t.Fatalf("a script problem must not be an error: %v", err)
		}
		if program != nil {
			t.Error("expected no executable when the script does not compile")
		}
		if len(diags) == 0 {
			t.Fatal("expected diagnostics for an undefined name")
		}
	})

	t.Run("a spent builder rejects a second Compile", func(t *testing.T) {
		builder := tascript.NewBuilder()
		if _, _, err := builder.Compile(counterSrc); err != nil {
			t.Fatalf("first compile: %v", err)
		}

		program, diags, err := builder.Compile(counterSrc)
		if !errors.Is(err, tascript.ErrBuilderSpent) {
			t.Fatalf("second compile: got %v, want ErrBuilderSpent", err)
		}
		if program != nil || diags != nil {
			t.Error("a rejected compile must return neither an executable nor diagnostics")
		}
	})

	t.Run("a rejected script still spends the builder", func(t *testing.T) {
		builder := tascript.NewBuilder()
		if _, diags, _ := builder.Compile("function Run() {\nnope\n}"); len(diags) == 0 {
			t.Fatal("expected diagnostics")
		}

		if _, _, err := builder.Compile(counterSrc); !errors.Is(err, tascript.ErrBuilderSpent) {
			t.Errorf("compile after a rejected script: got %v, want ErrBuilderSpent", err)
		}
	})

	t.Run("a spent builder rejects registration", func(t *testing.T) {
		builder := tascript.NewBuilder()
		if _, _, err := builder.Compile(counterSrc); err != nil {
			t.Fatalf("compile: %v", err)
		}

		if _, err := builder.RegisterType("Late"); !errors.Is(err, tascript.ErrBuilderSpent) {
			t.Errorf("RegisterType: got %v, want ErrBuilderSpent", err)
		}
		if _, err := builder.RegisterModule("late"); !errors.Is(err, tascript.ErrBuilderSpent) {
			t.Errorf("RegisterModule: got %v, want ErrBuilderSpent", err)
		}
		err := builder.RegisterMemberAccess(registry.FloatID, "late", registry.MemberAccessRule{})
		if !errors.Is(err, tascript.ErrBuilderSpent) {
			t.Errorf("RegisterMemberAccess: got %v, want ErrBuilderSpent", err)
		}
	})
}
