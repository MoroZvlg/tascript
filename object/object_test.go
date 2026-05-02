package object_test

import (
	"testing"

	"github.com/MoroZvlg/tascript/object"
)

func TestInterface(t *testing.T) {
	tests := []struct {
		input           object.Object
		expectedType    object.ObjectType
		expectedInspect string
	}{
		{&object.Integer{1}, object.IntKind, "1"},
		{&object.Integer{-1}, object.IntKind, "-1"},
		{&object.Float{1.01}, object.FloatKind, "1.01"},
		{&object.Boolean{true}, object.BooleanKind, "true"},
		{&object.String{"foo"}, object.StringKind, "foo"},
		{&object.Null{}, object.NullKind, "null"},
		{&object.Series{}, object.SeriesKind, "[0]"},
	}

	for _, tt := range tests {
		if tt.input.Inspect() != tt.expectedInspect {
			t.Errorf("expected inspect: %s, got: %s", tt.expectedInspect, tt.input.Inspect())
		}

		if tt.input.Type() != tt.expectedType {
			t.Errorf("expected type: %s, got: %s", tt.expectedType, tt.input.Type())
		}
	}
}
