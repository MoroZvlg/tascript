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
	return fmt.Sprintf("%s [UNEXPECTED_TOP_DECL] %s: only const, input, output, state, Init and Run functions allowed at top level", ud.Phase, ud.Pos.String())
}

type StateUndeclared struct {
	Phase Phase
	Token token.Token
}

func (su StateUndeclared) Error() string {
	return fmt.Sprintf("%s [STATE_UNDECLARED] %s: state field %s is not declared", su.Phase, su.Token.Pos.String(), su.Token.Literal)
}

type StateUninitialized struct {
	Phase Phase
	Token token.Token
}

func (su StateUninitialized) Error() string {
	return fmt.Sprintf("%s [STATE_UNINITIALIZED] %s: state field %s has no initializer and is never assigned in Init()", su.Phase, su.Token.Pos.String(), su.Token.Literal)
}

type TopDeclInBody struct {
	Phase   Phase
	Pos     token.Pos
	Keyword token.TokenType
}

func (td TopDeclInBody) Error() string {
	return fmt.Sprintf("%s [TOP_DECL_IN_BODY] %s: %s declaration is only allowed at the top level", td.Phase, td.Pos.String(), td.Keyword)
}

type DuplicateDeclaration struct {
	Phase        Phase
	KeywordToken token.Token
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

type UndefinedVar struct {
	Phase Phase
	Token token.Token
}

func (uv UndefinedVar) Error() string {
	return fmt.Sprintf("%s [UNDEFINED_VAR] %s: unknown variable %s", uv.Phase, uv.Token.Pos.String(), uv.Token.String())
}

type OutputNotReadable struct {
	Phase Phase
	Token token.Token
}

func (or OutputNotReadable) Error() string {
	return fmt.Sprintf("%s [OUTPUT_NOT_READABLE] %s: output %s is emit-only and cannot be read", or.Phase, or.Token.Pos.String(), or.Token.String())
}

type InvalidEmitTarget struct {
	Phase Phase
	Token token.Token
}

func (ie InvalidEmitTarget) Error() string {
	return fmt.Sprintf("%s [INVALID_EMIT_TARGET] %s: emit target must be a declared output", ie.Phase, ie.Token.Pos.String())
}

type NotAssignable struct {
	Phase Phase
	Token token.Token // the target identifier
	Kind  string      // binding kind of the target: const, input, output, module
}

func (na NotAssignable) Error() string {
	return fmt.Sprintf("%s [NOT_ASSIGNABLE] %s: cannot assign to %s %s", na.Phase, na.Token.Pos.String(), na.Kind, na.Token.String())
}

type InvalidAssignTarget struct {
	Phase Phase
	Token token.Token
}

func (ia InvalidAssignTarget) Error() string {
	return fmt.Sprintf("%s [INVALID_ASSIGN_TARGET] %s: expression is not assignable", ia.Phase, ia.Token.Pos.String())
}

type UndefinedType struct {
	Phase Phase
	Token token.Token
}

func (ut UndefinedType) Error() string {
	return fmt.Sprintf("%s [UNDEFINED_TYPE] %s: unknown type %s", ut.Phase, ut.Token.Pos.String(), ut.Token.String())
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

type NotIndexable struct {
	Phase Phase
	Pos   token.Pos
	Left  registry.TypeID
}

func (ni NotIndexable) Error() string {
	return fmt.Sprintf("%s [NOT_INDEXABLE] %s: %s is not indexable", ni.Phase, ni.Pos.String(), ni.Left)
}

type UndefinedFunc struct {
	Phase Phase
	Token token.Token
}

func (uf UndefinedFunc) Error() string {
	return fmt.Sprintf("%s [UNDEFINED_FUNC] %s: unknown function %s", uf.Phase, uf.Token.Pos.String(), uf.Token.Literal)
}

type NotCallable struct {
	Phase Phase
	Pos   token.Pos
}

func (nc NotCallable) Error() string {
	return fmt.Sprintf("%s [NOT_CALLABLE] %s: expression is not callable", nc.Phase, nc.Pos.String())
}

type EmitNotExpression struct {
	Phase Phase
	Token token.Token
}

func (en EmitNotExpression) Error() string {
	return fmt.Sprintf("%s [EMIT_NOT_EXPRESSION] %s: emit is a statement and cannot be used as a value", en.Phase, en.Token.Pos.String())
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

type TypeMissmatch struct {
	Phase    Phase
	Token    token.Token
	Expected registry.TypeID
	Got      registry.TypeID
}

func (tm TypeMissmatch) Error() string {
	return fmt.Sprintf("%s [TYPE_MISSMATCH] %s: expected %s type got %s", tm.Phase, tm.Token.Pos.String(), tm.Expected, tm.Got)
}

// RuntimeFailure is a script trap: a runtime failure a correct interpreter hit on legal input (e.g. division by zero)
type RuntimeFailure struct {
	Phase   Phase
	Pos     token.Pos
	Kind    registry.ErrorKind
	Message string
	EntryFn string
}

func (rf RuntimeFailure) Error() string {
	return fmt.Sprintf("%s [%s] %s: %s (in %s)", rf.Phase, rf.Kind, rf.Pos.String(), rf.Message, rf.EntryFn)
}

// InternalFailure is a recovered panic below an entry point:
// either core interpreter code hitting a resolver-guaranteed-impossible state,
// or a registered rule that panicked instead of returning an error.
type InternalFailure struct {
	Phase   Phase
	EntryFn string
	Panic   any
	Stack   []byte
}

func (in InternalFailure) Error() string {
	return fmt.Sprintf("%s [INTERNAL] unrecovered panic in %s: %v\n%s", in.Phase, in.EntryFn, in.Panic, in.Stack)
}
