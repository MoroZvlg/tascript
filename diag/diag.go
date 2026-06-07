package diag

import (
	"fmt"

	"github.com/MoroZvlg/tascript/token"
)

type Phase string

const (
	PhaseParse   Phase = "parse"
	PhaseLaunch  Phase = "launch"
	PhaseRuntime Phase = "runtime"
)

type Diagnostic interface {
	Error() string
}

type NotImplemented struct {
	Phase   Phase
	Pos     token.Pos
	Subject string
}

func (ni NotImplemented) Error() string {
	return fmt.Sprintf("%s [NOT_IMPLEMENTED] %s: %s not implemented", ni.Phase, ni.Pos.String(), ni.Subject)
}

type UnexpectedToken struct {
	Phase    Phase
	Pos      token.Pos
	Expected token.TokenType
	Got      token.TokenType
}

func (ut UnexpectedToken) Error() string {
	return fmt.Sprintf("%s [UNEXPECTED_TOKEN] %s: expected %s got %s", ut.Phase, ut.Pos.String(), ut.Expected, ut.Got)
}

type ExpressionExpected struct {
	Phase Phase
	Pos   token.Pos
	Got   token.TokenType
}

func (ee ExpressionExpected) Error() string {
	return fmt.Sprintf("%s [EXPRESSION_EXPECTED] %s: expected expression got %s", ee.Phase, ee.Pos.String(), ee.Got)
}

type ParseFailed struct {
	Phase  Phase
	Pos    token.Pos
	Target token.TokenType
}

func (pf ParseFailed) Error() string {
	return fmt.Sprintf("%s [PARSE_FAILED] %s: failed to parse %s", pf.Phase, pf.Pos.String(), pf.Target)
}

type TypeOrCustomTypeExpected struct {
	Phase Phase
	Pos   token.Pos
}

func (te TypeOrCustomTypeExpected) Error() string {
	return fmt.Sprintf("%s [TYPE_OR_CUSTOM_TYPE_EXPECTED] %s: type or custom type expected", te.Phase, te.Pos.String())
}

type EmptyCustomType struct {
	Phase Phase
	Pos   token.Pos
}

func (et EmptyCustomType) Error() string {
	return fmt.Sprintf("%s [EMPTY_CUSTOM_TYPE] %s: custom type should contain at least 1 field", et.Phase, et.Pos.String())
}
