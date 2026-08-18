package stdlib

import (
	"fmt"

	"github.com/MoroZvlg/tascript/registry"
)

// Accessors for rule bodies, where the resolver has already established the operand types.
// Each unwraps to the underlying primitive, so call sites stay free of nested conversions.
//
// They stay monomorphic and keep the panic behind a noinline call. The ok check itself is
// free; what costs is routing through the generic registry.Cast (+75%) or letting the
// message formatting inline, which blows the inline budget even on the branch never taken.

//go:noinline
func panicWrongType(want string, got registry.Value) {
	panic(fmt.Sprintf("expected %s, got %T: nothing guarded this position", want, got))
}

func asTime(value registry.Value) int64 {
	typed, ok := value.(Time)
	if !ok {
		panicWrongType("Time", value)
	}
	return int64(typed)
}

func asDuration(value registry.Value) int64 {
	typed, ok := value.(Duration)
	if !ok {
		panicWrongType("Duration", value)
	}
	return int64(typed)
}

func asInteger(value registry.Value) int64 {
	typed, ok := value.(registry.Integer)
	if !ok {
		panicWrongType("Integer", value)
	}
	return int64(typed)
}

// asFloat also accepts an Integer: a numeric position admits both, and the resolver only
// coerces at operator, argument, and assignment boundaries.
func asFloat(value registry.Value) float64 {
	switch value := value.(type) {
	case registry.Integer:
		return float64(value)
	case registry.Float:
		return float64(value)
	default:
		panic("value is not numeric")
	}
}
