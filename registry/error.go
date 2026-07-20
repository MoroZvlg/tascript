package registry

import "fmt"

// ErrorKind classifies runtime failures a correct interpreter can hit
type ErrorKind string

const (
	DivisionByZero  ErrorKind = "DIVISION_BY_ZERO"
	InvalidArgument ErrorKind = "INVALID_ARGUMENT"
	OutOfRange      ErrorKind = "OUT_OF_RANGE"
	UnassignedState ErrorKind = "UNASSIGNED_STATE"
	// UnknownKind marks a non-registry error. host-written rules may return arbitrary errors.
	UnknownKind ErrorKind = "UNKNOWN"
)

// Error is what rule EvalFns return to trap the current tick.
type Error struct {
	Kind    ErrorKind
	Message string
}

func (e Error) Error() string {
	return fmt.Sprintf("[%s] %s", e.Kind, e.Message)
}
