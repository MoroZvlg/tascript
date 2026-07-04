package registry_test

import (
	"math"
	"testing"

	"github.com/MoroZvlg/tascript/registry"
)

func TestMathModuleConstants(t *testing.T) {
	reg := registry.DefaultRegistry()
	mathModule := reg.Modules["math"]

	tests := []struct {
		name string
		want registry.Value
	}{
		{"PI", registry.Float(math.Pi)},
		{"E", registry.Float(math.E)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, ok := reg.LookupMemberAccess(mathModule.TypeID(), tt.name)
			if !ok {
				t.Fatal("constant is not registered")
			}
			if rule.EvalType != registry.FloatID {
				t.Fatalf("type = %v, want %v", rule.EvalType, registry.FloatID)
			}
			if got := rule.EvalFn(mathModule); got != tt.want {
				t.Fatalf("value = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMathModuleFunctions(t *testing.T) {
	reg := registry.DefaultRegistry()
	mathModule := reg.Modules["math"]

	tests := []struct {
		name string
		args map[string]registry.Value
		want registry.Value
	}{
		{"abs", map[string]registry.Value{"number": registry.Float(-2.5)}, registry.Float(2.5)},
		{"max", map[string]registry.Value{"a": registry.Float(1), "b": registry.Float(2.5)}, registry.Float(2.5)},
		{"min", map[string]registry.Value{"a": registry.Float(1), "b": registry.Float(2.5)}, registry.Float(1)},
		{"sqrt", map[string]registry.Value{"number": registry.Float(9)}, registry.Float(3)},
		{"pow", map[string]registry.Value{"base": registry.Float(2), "exponent": registry.Float(3)}, registry.Float(8)},
		{"floor", map[string]registry.Value{"number": registry.Float(1.9)}, registry.Float(1)},
		{"ceil", map[string]registry.Value{"number": registry.Float(1.1)}, registry.Float(2)},
		{"round", map[string]registry.Value{"number": registry.Float(1.5)}, registry.Float(2)},
		{"round", map[string]registry.Value{"number": registry.Float(-2.5)}, registry.Float(-2)},
		{"round", map[string]registry.Value{"number": registry.Float(-0.1)}, registry.Float(0)},
		{"trunc", map[string]registry.Value{"number": registry.Float(-1.9)}, registry.Float(-1)},
		{"sign", map[string]registry.Value{"number": registry.Float(-42)}, registry.Float(-1)},
		{"sign", map[string]registry.Value{"number": registry.Float(0)}, registry.Float(0)},
		{"log", map[string]registry.Value{"number": registry.Float(math.E)}, registry.Float(1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, ok := reg.LookupCall(mathModule.TypeID(), tt.name)
			if !ok {
				t.Fatal("function is not registered")
			}
			if rule.EvalType != registry.FloatID {
				t.Fatalf("type = %v, want %v", rule.EvalType, registry.FloatID)
			}
			got := rule.EvalFn(mathModule, tt.args)
			if got != tt.want {
				t.Fatalf("value = %v, want %v", got, tt.want)
			}
		})
	}
}
