package evaluator_test

import (
	"testing"

	"github.com/MoroZvlg/tascript/evaluator"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/parser"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/resolver"
)

func registerStateKind(reg *registry.Registry) {
	reg.RegisterDeclKind(registry.DeclKind{
		Word:        "state",
		Initializer: registry.InitializerOptional,
		Assignable:  true,
		Namespaced:  true,
	})
}

func testRegistry() *registry.Registry {
	reg := registry.DefaultRegistry()
	registerStateKind(reg)
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

type recorder struct {
	values []registry.Value
}

func (r *recorder) Emit(value registry.Value) {
	r.values = append(r.values, value)
}

func bindRecorder(t *testing.T, ev *evaluator.Evaluator, output string) *recorder {
	t.Helper()

	rec := &recorder{}
	if err := ev.BindOutput(output, rec); err != nil {
		t.Fatalf("bind output %s: %v", output, err)
	}
	return rec
}

func initedEval(t *testing.T, src string, customize func(*registry.Registry)) *evaluator.Evaluator {
	t.Helper()
	ev := compileSrc(t, src, customize)
	if _, err := ev.EvalInit(); err != nil {
		t.Fatalf("init error: %v", err)
	}
	return ev
}

func mustRun(t *testing.T, ev *evaluator.Evaluator) registry.Value {
	t.Helper()
	got, err := ev.EvalRun()
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	return got
}
