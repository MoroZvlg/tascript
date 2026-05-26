// Package diag holds the stable diagnostic types surfaced from compilation,
// launch, and runtime phases. Category codes are stable across language
// revisions; messages are not.
package diag

import "github.com/MoroZvlg/tascript/token"

type Phase string

const (
	PhaseParse   Phase = "parse"
	PhaseLaunch  Phase = "launch"
	PhaseRuntime Phase = "runtime"
)

type Category string

const (
	CatTopLevelForm      Category = "TOP_LEVEL_FORM"
	CatMissingRequiredFn Category = "MISSING_REQUIRED_FN"
	CatEmitReservedKwarg Category = "EMIT_RESERVED_KWARG"
	CatPortDuplicate     Category = "PORT_DUPLICATE"
	CatUnknownOutput     Category = "UNKNOWN_OUTPUT"
	CatEmitOutsideRun    Category = "EMIT_OUTSIDE_RUN"
	CatEmitPayload       Category = "EMIT_PAYLOAD"
	CatInputNotWired     Category = "INPUT_NOT_WIRED"
	CatOutputNotWired    Category = "OUTPUT_NOT_WIRED"
	CatTypeMismatch      Category = "TYPE_MISMATCH"
	CatNotImplemented    Category = "NOT_IMPLEMENTED"
)

type Diagnostic struct {
	Phase    Phase
	Category Category
	Pos      token.Pos
	Msg      string
}

func (d Diagnostic) Error() string {
	return string(d.Phase) + "[" + string(d.Category) + "] " + d.Pos.String() + ": " + d.Msg
}
