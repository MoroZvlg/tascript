package diag

import (
	"fmt"

	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/token"
)

type Phase string

const (
	PhaseParse   Phase = "parse"
	PhaseCheck   Phase = "check"
	PhaseRuntime Phase = "runtime"
)

type Diagnostic interface {
	Error() string
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

type ForbiddenFunc struct {
	Phase Phase
	Pos   token.Pos
}

func (ff ForbiddenFunc) Error() string {
	return fmt.Sprintf("%s [FORBIDDEN_FUNCTION] %s: only Init and Run functions are allowed", ff.Phase, ff.Pos.String())
}

type EmptyFunctionBody struct {
	Phase Phase
	Pos   token.Pos
}

func (ef EmptyFunctionBody) Error() string {
	return fmt.Sprintf("%s [EMPTY_FUNCTION] %s: Run function is empty", ef.Phase, ef.Pos.String())
}

type ArgsOrder struct {
	Phase Phase
	Pos   token.Pos
}

func (ao ArgsOrder) Error() string {
	return fmt.Sprintf("%s [ARGS_ORDER] %s: args after kwargs not allowed", ao.Phase, ao.Pos.String())
}

type MissingRunFunc struct {
	Phase Phase
	Pos   token.Pos
}

func (mr MissingRunFunc) Error() string {
	return fmt.Sprintf("%s [MISSING_REQUIRED_FN] %s: Run function is required", mr.Phase, mr.Pos.String())
}

type UnexpectedTopDecl struct {
	Phase Phase
	Pos   token.Pos
}

func (ud UnexpectedTopDecl) Error() string {
	return fmt.Sprintf("%s [UNEXPECTED_TOP_DECL] %s: only const, input, output, Init and Run functions allowed at top level", ud.Phase, ud.Pos.String())
}

type DuplicateDeclaration struct {
	Phase        Phase
	KeywordToken token.Token // keyword token
	IdentToken   token.Token
}

func (dd DuplicateDeclaration) Error() string {
	return fmt.Sprintf("%s [DUPLICATE_DECLARATION] %s: duplicate declaration of %s %s", dd.Phase, dd.KeywordToken.Pos.String(), dd.KeywordToken.Literal, dd.IdentToken.Literal)
}

type NestingTooDeep struct {
	Phase Phase
	Pos   token.Pos
}

func (nt NestingTooDeep) Error() string {
	return fmt.Sprintf("%s [DEPTH_LIMIT] %s: expression nested too deep", nt.Phase, nt.Pos.String())
}

type InvalidBinaryOperation struct {
	Phase Phase
	Token token.Token
	Left  registry.TypeID
	Right registry.TypeID
}

func (ib InvalidBinaryOperation) Error() string {
	return fmt.Sprintf("%s [INVALID_OPERATION] %s: %s can't be %s with %s", ib.Phase, ib.Token.Pos.String(), ib.Left, ib.Token.String(), ib.Right)
}

type UnaryBinaryOperation struct {
	Phase Phase
	Token token.Token
	Right registry.TypeID
}

func (iu UnaryBinaryOperation) Error() string {
	return fmt.Sprintf("%s [INVALID_OPERATION] %s: %s can't be %s", iu.Phase, iu.Token.Pos.String(), iu.Right, iu.Token.String())
}

type UndefinedIdent struct {
	Phase Phase
	Token token.Token
}

func (ui UndefinedIdent) Error() string {
	return fmt.Sprintf("%s [UNDEFINED_IDENT] %s: unknown identifier %s", ui.Phase, ui.Token.Pos.String(), ui.Token.String())
}

type UndefinedAttribute struct {
	Phase  Phase
	Member token.Token
}

func (ua UndefinedAttribute) Error() string {
	return fmt.Sprintf("%s [UNDEFINED_ATTRIBUTE] %s: unknown attribute %s", ua.Phase, ua.Member.Pos.String(), ua.Member.String())
}

type UndefinedMethod struct {
	Phase  Phase
	Method token.Token
}

func (um UndefinedMethod) Error() string {
	return fmt.Sprintf("%s [UNDEFINED_MEETHOD] %s: unknown method %s", um.Phase, um.Method.Pos.String(), um.Method.String())
}

type ArgsNumberMissmatch struct {
	Phase    Phase
	Token    token.Token
	Expected int
	Got      int
}

func (am ArgsNumberMissmatch) Error() string {
	return fmt.Sprintf("%s [ARGS_NUMBER_MISSMATCH] %s: expected %d args but got %d", am.Phase, am.Token.Pos.String(), am.Expected, am.Got)
}

type MissingArg struct {
	Phase    Phase
	Token    token.Token
	Expected string
}

func (am MissingArg) Error() string {
	return fmt.Sprintf("%s [ARG_MISSING] %s: missing %s arg", am.Phase, am.Token.Pos.String(), am.Expected)
}

type ArgTypeMissmatch struct {
	Phase    Phase
	Token    token.Token
	Expected registry.TypeID
	Got      registry.TypeID
}

func (am ArgTypeMissmatch) Error() string {
	return fmt.Sprintf("%s [ARG_TYPE_MISSMATCH] %s: expected %s type got %s", am.Phase, am.Token.Pos.String(), am.Expected, am.Got)
}
