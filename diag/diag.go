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
