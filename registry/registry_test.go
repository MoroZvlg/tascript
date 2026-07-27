package registry_test

import (
	"testing"

	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/token"
)

func TestRegistry_ErrorTypeIsReserved(t *testing.T) {
	errorType := registry.ErrorTypeID

	tests := []struct {
		name string
		call func(*registry.Registry) error
	}{
		{"RegisterType", func(r *registry.Registry) error {
			return r.RegisterType(registry.NewTypeID("Error"), registry.ScalarShape)
		}},
		{"RegisterModule", func(r *registry.Registry) error {
			_, err := r.RegisterModule("Error")
			return err
		}},
		{"RegisterScriptType by name", func(r *registry.Registry) error {
			_, err := r.RegisterScriptType("Error", nil)
			return err
		}},
		{"RegisterScriptType field type", func(r *registry.Registry) error {
			_, err := r.RegisterScriptType("rec", []registry.FieldDef{{Name: "f", Type: errorType}})
			return err
		}},
		{"RegisterCoercion from", func(r *registry.Registry) error {
			return r.RegisterCoercion(errorType, registry.FloatID, registry.CoerceRule{EvalType: registry.FloatID})
		}},
		{"RegisterCoercion to", func(r *registry.Registry) error {
			return r.RegisterCoercion(registry.IntegerID, errorType, registry.CoerceRule{EvalType: errorType})
		}},
		{"RegisterBinary operand", func(r *registry.Registry) error {
			return r.RegisterBinary(token.PLUS, errorType, registry.IntegerID, registry.BinaryRule{EvalType: registry.IntegerID})
		}},
		{"RegisterBinary result", func(r *registry.Registry) error {
			return r.RegisterBinary(token.PLUS, registry.IntegerID, registry.IntegerID, registry.BinaryRule{EvalType: errorType})
		}},
		{"RegisterUnary operand", func(r *registry.Registry) error {
			return r.RegisterUnary(token.MINUS, errorType, registry.UnaryRule{EvalType: registry.IntegerID})
		}},
		{"RegisterMemberAccess owner", func(r *registry.Registry) error {
			return r.RegisterMemberAccess(errorType, "field", registry.MemberAccessRule{EvalType: registry.IntegerID})
		}},
		{"RegisterMemberAccess result", func(r *registry.Registry) error {
			return r.RegisterMemberAccess(registry.IntegerID, "field", registry.MemberAccessRule{EvalType: errorType})
		}},
		{"RegisterCall result", func(r *registry.Registry) error {
			return r.RegisterCall(registry.IntegerID, "fn", registry.CallRule{EvalType: errorType})
		}},
		{"RegisterCall param", func(r *registry.Registry) error {
			return r.RegisterCall(registry.IntegerID, "fn", registry.CallRule{
				Args:     []registry.ParamRule{{Type: errorType, Name: "a"}},
				EvalType: registry.IntegerID,
			})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(registry.DefaultRegistry()); err == nil {
				t.Fatal("expected the reserved error type to be rejected, got nil error")
			}
		})
	}
}

func TestRegistry_ErrorTypeIsNotPreregistered(t *testing.T) {
	reg := registry.DefaultRegistry()
	if _, exists := reg.LookupType("Error"); exists {
		t.Error("the error type must not be resolvable as a script type name")
	}
}

func TestRegistry_DuplicateRegistrationRejected(t *testing.T) {
	custom := registry.NewTypeID("Custom")

	tests := []struct {
		name  string
		first func(*registry.Registry) error
		again func(*registry.Registry) error
	}{
		{
			"RegisterType",
			func(r *registry.Registry) error { return r.RegisterType(custom, registry.ScalarShape) },
			func(r *registry.Registry) error { return r.RegisterType(custom, registry.ScalarShape) },
		},
		{
			"RegisterModule",
			func(r *registry.Registry) error { _, err := r.RegisterModule("mod"); return err },
			func(r *registry.Registry) error { _, err := r.RegisterModule("mod"); return err },
		},
		{
			"RegisterScriptType",
			func(r *registry.Registry) error { _, err := r.RegisterScriptType("rec", nil); return err },
			func(r *registry.Registry) error { _, err := r.RegisterScriptType("rec", nil); return err },
		},
		{
			"RegisterCoercion",
			func(r *registry.Registry) error {
				return r.RegisterCoercion(registry.BoolID, registry.StringID, registry.CoerceRule{EvalType: registry.StringID})
			},
			func(r *registry.Registry) error {
				return r.RegisterCoercion(registry.BoolID, registry.StringID, registry.CoerceRule{EvalType: registry.StringID})
			},
		},
		{
			"RegisterBinary",
			func(r *registry.Registry) error {
				return r.RegisterBinary(token.PLUS, registry.BoolID, registry.BoolID, registry.BinaryRule{EvalType: registry.BoolID})
			},
			func(r *registry.Registry) error {
				return r.RegisterBinary(token.PLUS, registry.BoolID, registry.BoolID, registry.BinaryRule{EvalType: registry.BoolID})
			},
		},
		{
			"RegisterUnary",
			func(r *registry.Registry) error {
				return r.RegisterUnary(token.MINUS, registry.StringID, registry.UnaryRule{EvalType: registry.StringID})
			},
			func(r *registry.Registry) error {
				return r.RegisterUnary(token.MINUS, registry.StringID, registry.UnaryRule{EvalType: registry.StringID})
			},
		},
		{
			"RegisterMemberAccess",
			func(r *registry.Registry) error {
				return r.RegisterMemberAccess(registry.IntegerID, "attr", registry.MemberAccessRule{EvalType: registry.IntegerID})
			},
			func(r *registry.Registry) error {
				return r.RegisterMemberAccess(registry.IntegerID, "attr", registry.MemberAccessRule{EvalType: registry.IntegerID})
			},
		},
		{
			"RegisterCall",
			func(r *registry.Registry) error {
				return r.RegisterCall(registry.IntegerID, "fn", registry.CallRule{EvalType: registry.IntegerID})
			},
			func(r *registry.Registry) error {
				return r.RegisterCall(registry.IntegerID, "fn", registry.CallRule{EvalType: registry.IntegerID})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := registry.DefaultRegistry()
			if err := tt.first(reg); err != nil {
				t.Fatalf("first registration failed: %v", err)
			}
			if err := tt.again(reg); err == nil {
				t.Fatal("second registration should have been rejected, got nil error")
			}
		})
	}
}

func TestRegistry_FirstRegistrationWins(t *testing.T) {
	reg := registry.DefaultRegistry()
	rule := registry.BinaryRule{
		EvalType: registry.BoolID,
		EvalFn:   func(_, _ registry.Value) (registry.Value, error) { return registry.Bool(true), nil },
	}
	if err := reg.RegisterBinary(token.PLUS, registry.BoolID, registry.BoolID, rule); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	shadow := registry.BinaryRule{
		EvalType: registry.StringID,
		EvalFn:   func(_, _ registry.Value) (registry.Value, error) { return registry.String("clobbered"), nil },
	}
	if err := reg.RegisterBinary(token.PLUS, registry.BoolID, registry.BoolID, shadow); err == nil {
		t.Fatal("shadowing registration should have been rejected")
	}

	got, ok := reg.LookupBinary(token.PLUS, registry.BoolID, registry.BoolID)
	if !ok {
		t.Fatal("rule disappeared after a rejected duplicate")
	}
	if got.EvalType != registry.BoolID {
		t.Errorf("EvalType = %s, want %s: the rejected duplicate overwrote the original", got.EvalType, registry.BoolID)
	}
	value, _ := got.EvalFn(registry.Bool(true), registry.Bool(true))
	if value != registry.Bool(true) {
		t.Errorf("EvalFn returned %v: the rejected duplicate replaced the original", value)
	}
}

func TestRegistry_LookupRoundTrips(t *testing.T) {
	reg := registry.DefaultRegistry()
	owner := registry.NewTypeID("Widget")
	if err := reg.RegisterType(owner, registry.ScalarShape); err != nil {
		t.Fatalf("RegisterType: %v", err)
	}

	t.Run("type by name", func(t *testing.T) {
		id, ok := reg.LookupType("Widget")
		if !ok || id != owner {
			t.Fatalf("LookupType = (%v, %v), want (%v, true)", id, ok, owner)
		}
	})

	t.Run("unregistered type is not found", func(t *testing.T) {
		if _, ok := reg.LookupType("Nope"); ok {
			t.Error("LookupType found a type that was never registered")
		}
	})

	t.Run("member access", func(t *testing.T) {
		reg.RegisterMemberAccess(owner, "size", registry.MemberAccessRule{EvalType: registry.IntegerID})
		rule, ok := reg.LookupMemberAccess(owner, "size")
		if !ok || rule.EvalType != registry.IntegerID {
			t.Fatalf("LookupMemberAccess = (%v, %v)", rule.EvalType, ok)
		}
		if _, ok := reg.LookupMemberAccess(owner, "missing"); ok {
			t.Error("LookupMemberAccess found an unregistered member")
		}
	})

	t.Run("call", func(t *testing.T) {
		reg.RegisterCall(owner, "grow", registry.CallRule{
			Args:     []registry.ParamRule{{Type: registry.IntegerID, Name: "by"}},
			EvalType: owner,
		})
		rule, ok := reg.LookupCall(owner, "grow")
		if !ok || len(rule.Args) != 1 || rule.Args[0].Name != "by" {
			t.Fatalf("LookupCall = (%+v, %v)", rule, ok)
		}
	})

	t.Run("binary is keyed on operand order", func(t *testing.T) {
		reg.RegisterBinary(token.ASTERISK, owner, registry.IntegerID, registry.BinaryRule{EvalType: owner})
		if _, ok := reg.LookupBinary(token.ASTERISK, owner, registry.IntegerID); !ok {
			t.Error("registered operand order not found")
		}
		if _, ok := reg.LookupBinary(token.ASTERISK, registry.IntegerID, owner); ok {
			t.Error("reversed operand order must not resolve to the same rule")
		}
	})
}

func TestRegistry_Coercion(t *testing.T) {
	reg := registry.DefaultRegistry()

	t.Run("builtin int to float", func(t *testing.T) {
		rule, ok := reg.LookupCoerce(registry.IntegerID, registry.FloatID)
		if !ok {
			t.Fatal("int -> float coercion missing")
		}
		if got := rule.EvalFn(registry.Integer(3)); got != registry.Float(3) {
			t.Errorf("coerced to %v, want 3", got)
		}
	})

	t.Run("no reverse edge", func(t *testing.T) {
		if _, ok := reg.LookupCoerce(registry.FloatID, registry.IntegerID); ok {
			t.Error("float -> int must not exist: it is lossy and was never registered")
		}
	})
}

func TestRegistry_EmitRule(t *testing.T) {
	reg := registry.DefaultRegistry()

	t.Run("scalar output takes one value param", func(t *testing.T) {
		rule := reg.EmitRule(registry.IntegerID)
		if len(rule.Args) != 1 {
			t.Fatalf("got %d params, want 1", len(rule.Args))
		}
		if rule.Args[0].Name != "value" || rule.Args[0].Type != registry.IntegerID {
			t.Errorf("param = %+v, want {value, Integer}", rule.Args[0])
		}
		if rule.EvalType != registry.IntegerID {
			t.Errorf("EvalType = %s, want Integer", rule.EvalType)
		}
	})

	t.Run("structural output takes one param per field", func(t *testing.T) {
		id, err := reg.RegisterScriptType("output.sig", []registry.FieldDef{
			{Name: "dir", Type: registry.StringID},
			{Name: "price", Type: registry.FloatID},
		})
		if err != nil {
			t.Fatalf("RegisterScriptType: %v", err)
		}
		rule := reg.EmitRule(id)
		if len(rule.Args) != 2 {
			t.Fatalf("got %d params, want 2", len(rule.Args))
		}
		if rule.Args[0].Name != "dir" || rule.Args[0].Type != registry.StringID {
			t.Errorf("param[0] = %+v, want {dir, String}", rule.Args[0])
		}
		if rule.Args[1].Name != "price" || rule.Args[1].Type != registry.FloatID {
			t.Errorf("param[1] = %+v, want {price, Float}", rule.Args[1])
		}
	})

	t.Run("unregistered output type falls through to the scalar shape", func(t *testing.T) {
		ghost := registry.NewTypeID("NeverRegistered")
		if _, exists := reg.LookupTypeDef(ghost); exists {
			t.Fatal("precondition: type must not be registered")
		}
		rule := reg.EmitRule(ghost)
		if len(rule.Args) != 1 || rule.Args[0].Name != "value" {
			t.Errorf("got %+v, want the one-value scalar shape", rule.Args)
		}
	})
}
