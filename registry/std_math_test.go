package registry_test

import (
	"testing"

	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/token"
)

func TestDefaultNumericOperations(t *testing.T) {
	tests := []struct {
		name     string
		operator token.TokenType
		left     registry.Value
		right    registry.Value
		want     registry.Value
		wantType registry.TypeID
	}{
		{"integer addition", token.PLUS, registry.Integer(7), registry.Integer(2), registry.Integer(9), registry.IntegerID},
		{"float addition", token.PLUS, registry.Float(7.5), registry.Float(2.5), registry.Float(10), registry.FloatID},
		{"mixed addition", token.PLUS, registry.Integer(7), registry.Float(2.5), registry.Float(9.5), registry.FloatID},
		{"reverse mixed addition", token.PLUS, registry.Float(7.5), registry.Integer(2), registry.Float(9.5), registry.FloatID},
		{"integer subtraction", token.MINUS, registry.Integer(7), registry.Integer(2), registry.Integer(5), registry.IntegerID},
		{"float subtraction", token.MINUS, registry.Float(7.5), registry.Float(2.5), registry.Float(5), registry.FloatID},
		{"mixed subtraction", token.MINUS, registry.Integer(7), registry.Float(2.5), registry.Float(4.5), registry.FloatID},
		{"reverse mixed subtraction", token.MINUS, registry.Float(7.5), registry.Integer(2), registry.Float(5.5), registry.FloatID},
		{"integer multiplication", token.ASTERISK, registry.Integer(7), registry.Integer(2), registry.Integer(14), registry.IntegerID},
		{"float multiplication", token.ASTERISK, registry.Float(7.5), registry.Float(2), registry.Float(15), registry.FloatID},
		{"mixed multiplication", token.ASTERISK, registry.Integer(7), registry.Float(2.5), registry.Float(17.5), registry.FloatID},
		{"reverse mixed multiplication", token.ASTERISK, registry.Float(7.5), registry.Integer(2), registry.Float(15), registry.FloatID},
		{"integer division", token.SLASH, registry.Integer(7), registry.Integer(2), registry.Float(3.5), registry.FloatID},
		{"float division", token.SLASH, registry.Float(7.5), registry.Float(2.5), registry.Float(3), registry.FloatID},
		{"mixed division", token.SLASH, registry.Integer(7), registry.Float(2), registry.Float(3.5), registry.FloatID},
		{"reverse mixed division", token.SLASH, registry.Float(7.5), registry.Integer(2), registry.Float(3.75), registry.FloatID},
		{"integer remainder", token.PERCENT, registry.Integer(7), registry.Integer(3), registry.Integer(1), registry.IntegerID},
		{"float remainder", token.PERCENT, registry.Float(7.5), registry.Float(2), registry.Float(1.5), registry.FloatID},
		{"mixed remainder", token.PERCENT, registry.Integer(7), registry.Float(2.5), registry.Float(2), registry.FloatID},
		{"reverse mixed remainder", token.PERCENT, registry.Float(7.5), registry.Integer(2), registry.Float(1.5), registry.FloatID},
	}

	reg := registry.DefaultRegistry()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rule, ok := reg.LookupBinary(test.operator, test.left.TypeID(), test.right.TypeID())
			if !ok {
				t.Fatal("operation is not registered")
			}
			if rule.EvalType != test.wantType {
				t.Errorf("result type = %v, want %v", rule.EvalType, test.wantType)
			}
			if got := rule.EvalFn(test.operator, test.left, test.right); got != test.want {
				t.Errorf("result = %v, want %v", got, test.want)
			}
		})
	}
}
