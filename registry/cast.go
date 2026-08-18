package registry

import "fmt"

// Accessors for rule bodies, where the resolver has already established the operand types.
// A mismatch is an engine or host-registration bug, not a script error: the panic is
// recovered by the evaluator and surfaces as a diagnostic.
//
// They stay monomorphic and keep the panic behind a noinline call, so the check itself costs
// nothing — a generic version pays a dictionary, and formatting the message inline blows the
// inline budget even on the branch never taken.

//go:noinline
func panicWrongType(want string, got Value) {
	panic(fmt.Sprintf("expected %s, got %T: nothing guarded this position", want, got))
}

func asInteger(value Value) Integer {
	typed, ok := value.(Integer)
	if !ok {
		panicWrongType("Integer", value)
	}
	return typed
}

func asString(value Value) String {
	typed, ok := value.(String)
	if !ok {
		panicWrongType("String", value)
	}
	return typed
}

func asBool(value Value) Bool {
	typed, ok := value.(Bool)
	if !ok {
		panicWrongType("Bool", value)
	}
	return typed
}

func asRecord(value Value) Record {
	typed, ok := value.(Record)
	if !ok {
		panicWrongType("Record", value)
	}
	return typed
}
