package tascript_test

import (
	"testing"

	"github.com/MoroZvlg/tascript"
	"github.com/MoroZvlg/tascript/registry"
)

const counterSrc = "state n: Integer = 0\noutput tick: Integer\nfunction Run() {\nstate.n = state.n + 1\nemit(tick, state.n)\n}"

type recorder struct {
	values []registry.Value
}

func (r *recorder) Emit(value registry.Value) {
	r.values = append(r.values, value)
}

func (r *recorder) last() registry.Value {
	if len(r.values) == 0 {
		return nil
	}
	return r.values[len(r.values)-1]
}

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

func counterProgram(t *testing.T) (*tascript.Executable, *recorder) {
	t.Helper()

	program := compile(t, counterSrc)
	ticks := &recorder{}
	if err := program.BindOutput("tick", ticks); err != nil {
		t.Fatalf("bind output: %v", err)
	}
	return program, ticks
}

type box struct {
	id registry.TypeID
	v  float64
}

func (b *box) TypeID() registry.TypeID { return b.id }

func boxProgram(t *testing.T) (*tascript.Executable, registry.TypeID, *recorder) {
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

	program, diags, err := builder.Compile("input b: Box\noutput seen: Float\nfunction Run() {\nemit(seen, b.v)\n}")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(diags) > 0 {
		t.Fatalf("compile diagnostics: %v", diags)
	}

	seen := &recorder{}
	if err := program.BindOutput("seen", seen); err != nil {
		t.Fatalf("bind output: %v", err)
	}
	return program, boxID, seen
}
