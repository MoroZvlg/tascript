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

type Code string

const (
	CodeUnexpectedToken      Code = "UNEXPECTED_TOKEN"
	CodeExpressionExpected   Code = "EXPRESSION_EXPECTED"
	CodeNumberOutOfRange     Code = "NUMBER_OUT_OF_RANGE"
	CodeTypeExpected         Code = "TYPE_EXPECTED"
	CodeEmptyCustomType      Code = "EMPTY_CUSTOM_TYPE"
	CodeForbiddenFunction    Code = "FORBIDDEN_FUNCTION"
	CodeEmptyFunction        Code = "EMPTY_FUNCTION"
	CodeArgOrderInvalid      Code = "ARG_ORDER_INVALID"
	CodeMissingRun           Code = "MISSING_RUN"
	CodeTopDeclUnexpected    Code = "TOP_DECL_UNEXPECTED"
	CodeSlotUndeclared       Code = "SLOT_UNDECLARED"
	CodeUnknownDeclKeyword   Code = "UNKNOWN_DECL_KEYWORD"
	CodeInitializerRequired  Code = "INITIALIZER_REQUIRED"
	CodeInitializerForbidden Code = "INITIALIZER_FORBIDDEN"
	CodeTypeRequired         Code = "TYPE_REQUIRED"
	CodeDeclTypeNotAllowed   Code = "DECL_TYPE_NOT_ALLOWED"
	CodeUseBeforeDeclaration Code = "USE_BEFORE_DECLARATION"
	CodeInputInInit          Code = "INPUT_IN_INIT"
	CodeTopDeclMisplaced     Code = "TOP_DECL_MISPLACED"
	CodeDuplicateDeclaration Code = "DUPLICATE_DECLARATION"
	CodeReservedName         Code = "RESERVED_NAME"
	CodeNestingTooDeep       Code = "NESTING_TOO_DEEP"
	CodeInvalidOperation     Code = "INVALID_OPERATION"
	CodeUndefinedIdent       Code = "UNDEFINED_IDENT"
	CodeUndefinedVar         Code = "UNDEFINED_VAR"
	CodeNotReadable          Code = "NOT_READABLE"
	CodeInvalidEmitTarget    Code = "INVALID_EMIT_TARGET"
	CodeNotAssignable        Code = "NOT_ASSIGNABLE"
	CodeInvalidAssignTarget  Code = "INVALID_ASSIGN_TARGET"
	CodeUndefinedType        Code = "UNDEFINED_TYPE"
	CodeUndefinedAttribute   Code = "UNDEFINED_ATTRIBUTE"
	CodeUndefinedMethod      Code = "UNDEFINED_METHOD"
	CodeNotIndexable         Code = "NOT_INDEXABLE"
	CodeUndefinedFunc        Code = "UNDEFINED_FUNC"
	CodeNotCallable          Code = "NOT_CALLABLE"
	CodeEmitNotExpression    Code = "EMIT_NOT_EXPRESSION"
	CodeEmitOutsideRun       Code = "EMIT_OUTSIDE_RUN"
	CodeArgCountMismatch     Code = "ARG_COUNT_MISMATCH"
	CodeArgMissing           Code = "ARG_MISSING"
	CodeArgDuplicate         Code = "ARG_DUPLICATE"
	CodeArgUnknown           Code = "ARG_UNKNOWN"
	CodeTypeMismatch         Code = "TYPE_MISMATCH"
	CodeInputUnknown         Code = "INPUT_UNKNOWN"
	CodeInputMissing         Code = "INPUT_MISSING"
	CodeOutputUnknown        Code = "OUTPUT_UNKNOWN"
	CodeOutputMissing        Code = "OUTPUT_MISSING"
	CodeInputTypeMismatch    Code = "INPUT_TYPE_MISMATCH"
	CodeOutputTypeMismatch   Code = "OUTPUT_TYPE_MISMATCH"
	CodeSlotTypeMismatch     Code = "SLOT_TYPE_MISMATCH"
	CodeTypeRegistrationFail Code = "TYPE_REGISTRATION_FAILED"
	CodeInternalFailure      Code = "INTERNAL_FAILURE"
)

type Diagnostic interface {
	Error() string
	Code() Code
	Phase() Phase
	Pos() token.Pos
}

func render(code Code, pos token.Pos, msg string, args ...any) string {
	return fmt.Sprintf("error[%s] %s: %s", code, pos.String(), fmt.Sprintf(msg, args...))
}

type UnexpectedToken struct {
	At       token.Pos
	Expected token.TokenType
	Got      token.TokenType
}

func (ut UnexpectedToken) Code() Code     { return CodeUnexpectedToken }
func (ut UnexpectedToken) Phase() Phase   { return PhaseParse }
func (ut UnexpectedToken) Pos() token.Pos { return ut.At }

func (ut UnexpectedToken) Error() string {
	return render(ut.Code(), ut.At, "expected %s, found %s", ut.Expected, ut.Got)
}

type ExpressionExpected struct {
	At  token.Pos
	Got token.TokenType
}

func (ee ExpressionExpected) Code() Code     { return CodeExpressionExpected }
func (ee ExpressionExpected) Phase() Phase   { return PhaseParse }
func (ee ExpressionExpected) Pos() token.Pos { return ee.At }

func (ee ExpressionExpected) Error() string {
	return render(ee.Code(), ee.At, "expected expression, found %s", ee.Got)
}

type NumberOutOfRange struct {
	At      token.Pos
	Target  token.TokenType
	Literal string
}

func (nr NumberOutOfRange) Code() Code     { return CodeNumberOutOfRange }
func (nr NumberOutOfRange) Phase() Phase   { return PhaseParse }
func (nr NumberOutOfRange) Pos() token.Pos { return nr.At }

func (nr NumberOutOfRange) Error() string {
	return render(nr.Code(), nr.At, "%s literal %s is out of range", nr.Target, nr.Literal)
}

type TypeExpected struct {
	At token.Pos
}

func (te TypeExpected) Code() Code     { return CodeTypeExpected }
func (te TypeExpected) Phase() Phase   { return PhaseParse }
func (te TypeExpected) Pos() token.Pos { return te.At }

func (te TypeExpected) Error() string {
	return render(te.Code(), te.At, "type or custom type expected")
}

type EmptyCustomType struct {
	At token.Pos
}

func (et EmptyCustomType) Code() Code     { return CodeEmptyCustomType }
func (et EmptyCustomType) Phase() Phase   { return PhaseParse }
func (et EmptyCustomType) Pos() token.Pos { return et.At }

func (et EmptyCustomType) Error() string {
	return render(et.Code(), et.At, "custom type should contain at least 1 field")
}

type ForbiddenFunction struct {
	At token.Pos
}

func (ff ForbiddenFunction) Code() Code     { return CodeForbiddenFunction }
func (ff ForbiddenFunction) Phase() Phase   { return PhaseParse }
func (ff ForbiddenFunction) Pos() token.Pos { return ff.At }

func (ff ForbiddenFunction) Error() string {
	return render(ff.Code(), ff.At, "only Init and Run functions are allowed")
}

type EmptyFunction struct {
	At token.Pos
}

func (ef EmptyFunction) Code() Code     { return CodeEmptyFunction }
func (ef EmptyFunction) Phase() Phase   { return PhaseParse }
func (ef EmptyFunction) Pos() token.Pos { return ef.At }

func (ef EmptyFunction) Error() string {
	return render(ef.Code(), ef.At, "Run function is empty")
}

type ArgOrderInvalid struct {
	At token.Pos
}

func (ao ArgOrderInvalid) Code() Code     { return CodeArgOrderInvalid }
func (ao ArgOrderInvalid) Phase() Phase   { return PhaseParse }
func (ao ArgOrderInvalid) Pos() token.Pos { return ao.At }

func (ao ArgOrderInvalid) Error() string {
	return render(ao.Code(), ao.At, "args after kwargs not allowed")
}

type MissingRun struct {
	At token.Pos
}

func (mr MissingRun) Code() Code     { return CodeMissingRun }
func (mr MissingRun) Phase() Phase   { return PhaseParse }
func (mr MissingRun) Pos() token.Pos { return mr.At }

func (mr MissingRun) Error() string {
	return render(mr.Code(), mr.At, "Run function is required")
}

type TopDeclUnexpected struct {
	At token.Pos
}

func (ud TopDeclUnexpected) Code() Code     { return CodeTopDeclUnexpected }
func (ud TopDeclUnexpected) Phase() Phase   { return PhaseParse }
func (ud TopDeclUnexpected) Pos() token.Pos { return ud.At }

func (ud TopDeclUnexpected) Error() string {
	return render(ud.Code(), ud.At, "only declarations and Init/Run functions are allowed at the top level")
}

type TopDeclMisplaced struct {
	At      token.Pos
	Keyword token.TokenType
}

func (td TopDeclMisplaced) Code() Code     { return CodeTopDeclMisplaced }
func (td TopDeclMisplaced) Phase() Phase   { return PhaseParse }
func (td TopDeclMisplaced) Pos() token.Pos { return td.At }

func (td TopDeclMisplaced) Error() string {
	return render(td.Code(), td.At, "%s declaration is only allowed at the top level", td.Keyword)
}

type NestingTooDeep struct {
	At token.Pos
}

func (dl NestingTooDeep) Code() Code     { return CodeNestingTooDeep }
func (dl NestingTooDeep) Phase() Phase   { return PhaseParse }
func (dl NestingTooDeep) Pos() token.Pos { return dl.At }

func (dl NestingTooDeep) Error() string {
	return render(dl.Code(), dl.At, "expression nested too deep")
}

type DuplicateDeclaration struct {
	At      token.Pos
	Keyword string
	Name    string
}

func (dd DuplicateDeclaration) Code() Code     { return CodeDuplicateDeclaration }
func (dd DuplicateDeclaration) Phase() Phase   { return PhaseCheck }
func (dd DuplicateDeclaration) Pos() token.Pos { return dd.At }

func (dd DuplicateDeclaration) Error() string {
	return render(dd.Code(), dd.At, "duplicate declaration of %s %s", dd.Keyword, dd.Name)
}

type ReservedName struct {
	At   token.Pos
	Name string
	Kind string
}

func (rn ReservedName) Code() Code     { return CodeReservedName }
func (rn ReservedName) Phase() Phase   { return PhaseCheck }
func (rn ReservedName) Pos() token.Pos { return rn.At }

func (rn ReservedName) Error() string {
	return render(rn.Code(), rn.At, "%s is a reserved %s name", rn.Name, rn.Kind)
}

type SlotUndeclared struct {
	At   token.Pos
	Kind string
	Name string
}

func (su SlotUndeclared) Code() Code     { return CodeSlotUndeclared }
func (su SlotUndeclared) Phase() Phase   { return PhaseCheck }
func (su SlotUndeclared) Pos() token.Pos { return su.At }

func (su SlotUndeclared) Error() string {
	return render(su.Code(), su.At, "%s.%s is not declared", su.Kind, su.Name)
}

type UnknownDeclKeyword struct {
	At   token.Pos
	Word string
}

func (ud UnknownDeclKeyword) Code() Code     { return CodeUnknownDeclKeyword }
func (ud UnknownDeclKeyword) Phase() Phase   { return PhaseCheck }
func (ud UnknownDeclKeyword) Pos() token.Pos { return ud.At }

func (ud UnknownDeclKeyword) Error() string {
	return render(ud.Code(), ud.At, "unknown declaration keyword %s", ud.Word)
}

type InitializerRequired struct {
	At   token.Pos
	Kind string
	Name string
}

func (ir InitializerRequired) Code() Code     { return CodeInitializerRequired }
func (ir InitializerRequired) Phase() Phase   { return PhaseCheck }
func (ir InitializerRequired) Pos() token.Pos { return ir.At }

func (ir InitializerRequired) Error() string {
	return render(ir.Code(), ir.At, "%s %s requires an initializer", ir.Kind, ir.Name)
}

type InitializerForbidden struct {
	At   token.Pos
	Kind string
	Name string
}

func (id InitializerForbidden) Code() Code     { return CodeInitializerForbidden }
func (id InitializerForbidden) Phase() Phase   { return PhaseCheck }
func (id InitializerForbidden) Pos() token.Pos { return id.At }

func (id InitializerForbidden) Error() string {
	return render(id.Code(), id.At, "%s %s cannot have an initializer", id.Kind, id.Name)
}

type TypeRequired struct {
	At   token.Pos
	Kind string
	Name string
}

func (tr TypeRequired) Code() Code     { return CodeTypeRequired }
func (tr TypeRequired) Phase() Phase   { return PhaseCheck }
func (tr TypeRequired) Pos() token.Pos { return tr.At }

func (tr TypeRequired) Error() string {
	return render(tr.Code(), tr.At, "%s %s needs a type annotation or an initializer", tr.Kind, tr.Name)
}

type DeclTypeNotAllowed struct {
	At   token.Pos
	Kind string
	T    registry.TypeID
}

func (dt DeclTypeNotAllowed) Code() Code     { return CodeDeclTypeNotAllowed }
func (dt DeclTypeNotAllowed) Phase() Phase   { return PhaseCheck }
func (dt DeclTypeNotAllowed) Pos() token.Pos { return dt.At }

func (dt DeclTypeNotAllowed) Error() string {
	return render(dt.Code(), dt.At, "%s does not accept type %s", dt.Kind, dt.T)
}

type UseBeforeDeclaration struct {
	At   token.Pos
	Name string
}

func (ub UseBeforeDeclaration) Code() Code     { return CodeUseBeforeDeclaration }
func (ub UseBeforeDeclaration) Phase() Phase   { return PhaseCheck }
func (ub UseBeforeDeclaration) Pos() token.Pos { return ub.At }

func (ub UseBeforeDeclaration) Error() string {
	return render(ub.Code(), ub.At, "%s is referenced before its declaration", ub.Name)
}

type InputInInit struct {
	At   token.Pos
	Name string
}

func (ii InputInInit) Code() Code     { return CodeInputInInit }
func (ii InputInInit) Phase() Phase   { return PhaseCheck }
func (ii InputInInit) Pos() token.Pos { return ii.At }

func (ii InputInInit) Error() string {
	return render(ii.Code(), ii.At, "input %s is not available before Run", ii.Name)
}

type InvalidBinaryOperation struct {
	At    token.Pos
	Op    string
	Left  registry.TypeID
	Right registry.TypeID
}

func (ib InvalidBinaryOperation) Code() Code     { return CodeInvalidOperation }
func (ib InvalidBinaryOperation) Phase() Phase   { return PhaseCheck }
func (ib InvalidBinaryOperation) Pos() token.Pos { return ib.At }

func (ib InvalidBinaryOperation) Error() string {
	return render(ib.Code(), ib.At, "cannot apply %s to %s and %s", ib.Op, ib.Left, ib.Right)
}

type InvalidUnaryOperation struct {
	At    token.Pos
	Op    string
	Right registry.TypeID
}

func (iu InvalidUnaryOperation) Code() Code     { return CodeInvalidOperation }
func (iu InvalidUnaryOperation) Phase() Phase   { return PhaseCheck }
func (iu InvalidUnaryOperation) Pos() token.Pos { return iu.At }

func (iu InvalidUnaryOperation) Error() string {
	return render(iu.Code(), iu.At, "cannot apply %s to %s", iu.Op, iu.Right)
}

type UndefinedIdent struct {
	At   token.Pos
	Name string
}

func (ui UndefinedIdent) Code() Code     { return CodeUndefinedIdent }
func (ui UndefinedIdent) Phase() Phase   { return PhaseCheck }
func (ui UndefinedIdent) Pos() token.Pos { return ui.At }

func (ui UndefinedIdent) Error() string {
	return render(ui.Code(), ui.At, "unknown identifier %s", ui.Name)
}

type UndefinedVar struct {
	At   token.Pos
	Name string
}

func (uv UndefinedVar) Code() Code     { return CodeUndefinedVar }
func (uv UndefinedVar) Phase() Phase   { return PhaseCheck }
func (uv UndefinedVar) Pos() token.Pos { return uv.At }

func (uv UndefinedVar) Error() string {
	return render(uv.Code(), uv.At, "unknown variable %s", uv.Name)
}

type NotReadable struct {
	At   token.Pos
	Name string
	Kind string
}

func (nr NotReadable) Code() Code     { return CodeNotReadable }
func (nr NotReadable) Phase() Phase   { return PhaseCheck }
func (nr NotReadable) Pos() token.Pos { return nr.At }

func (nr NotReadable) Error() string {
	return render(nr.Code(), nr.At, "cannot read %s %s", nr.Kind, nr.Name)
}

type InvalidEmitTarget struct {
	At token.Pos
}

func (iet InvalidEmitTarget) Code() Code     { return CodeInvalidEmitTarget }
func (iet InvalidEmitTarget) Phase() Phase   { return PhaseCheck }
func (iet InvalidEmitTarget) Pos() token.Pos { return iet.At }

func (iet InvalidEmitTarget) Error() string {
	return render(iet.Code(), iet.At, "emit target must be a declared output")
}

type NotAssignable struct {
	At   token.Pos
	Name string
	Kind string
}

func (na NotAssignable) Code() Code     { return CodeNotAssignable }
func (na NotAssignable) Phase() Phase   { return PhaseCheck }
func (na NotAssignable) Pos() token.Pos { return na.At }

func (na NotAssignable) Error() string {
	return render(na.Code(), na.At, "cannot assign to %s %s", na.Kind, na.Name)
}

type InvalidAssignTarget struct {
	At token.Pos
}

func (ia InvalidAssignTarget) Code() Code     { return CodeInvalidAssignTarget }
func (ia InvalidAssignTarget) Phase() Phase   { return PhaseCheck }
func (ia InvalidAssignTarget) Pos() token.Pos { return ia.At }

func (ia InvalidAssignTarget) Error() string {
	return render(ia.Code(), ia.At, "expression is not assignable")
}

type UndefinedType struct {
	At   token.Pos
	Name string
}

func (ut2 UndefinedType) Code() Code     { return CodeUndefinedType }
func (ut2 UndefinedType) Phase() Phase   { return PhaseCheck }
func (ut2 UndefinedType) Pos() token.Pos { return ut2.At }

func (ut2 UndefinedType) Error() string {
	return render(ut2.Code(), ut2.At, "unknown type %s", ut2.Name)
}

type UndefinedAttribute struct {
	At   token.Pos
	Name string
}

func (ua UndefinedAttribute) Code() Code     { return CodeUndefinedAttribute }
func (ua UndefinedAttribute) Phase() Phase   { return PhaseCheck }
func (ua UndefinedAttribute) Pos() token.Pos { return ua.At }

func (ua UndefinedAttribute) Error() string {
	return render(ua.Code(), ua.At, "unknown attribute %s", ua.Name)
}

type UndefinedMethod struct {
	At   token.Pos
	Name string
}

func (um UndefinedMethod) Code() Code     { return CodeUndefinedMethod }
func (um UndefinedMethod) Phase() Phase   { return PhaseCheck }
func (um UndefinedMethod) Pos() token.Pos { return um.At }

func (um UndefinedMethod) Error() string {
	return render(um.Code(), um.At, "unknown method %s", um.Name)
}

type NotIndexable struct {
	At   token.Pos
	Left registry.TypeID
}

func (ni NotIndexable) Code() Code     { return CodeNotIndexable }
func (ni NotIndexable) Phase() Phase   { return PhaseCheck }
func (ni NotIndexable) Pos() token.Pos { return ni.At }

func (ni NotIndexable) Error() string {
	return render(ni.Code(), ni.At, "%s is not indexable", ni.Left)
}

type UndefinedFunc struct {
	At   token.Pos
	Name string
}

func (uf UndefinedFunc) Code() Code     { return CodeUndefinedFunc }
func (uf UndefinedFunc) Phase() Phase   { return PhaseCheck }
func (uf UndefinedFunc) Pos() token.Pos { return uf.At }

func (uf UndefinedFunc) Error() string {
	return render(uf.Code(), uf.At, "unknown function %s", uf.Name)
}

type NotCallable struct {
	At token.Pos
}

func (nc NotCallable) Code() Code     { return CodeNotCallable }
func (nc NotCallable) Phase() Phase   { return PhaseCheck }
func (nc NotCallable) Pos() token.Pos { return nc.At }

func (nc NotCallable) Error() string {
	return render(nc.Code(), nc.At, "expression is not callable")
}

type EmitNotExpression struct {
	At token.Pos
}

func (en EmitNotExpression) Code() Code     { return CodeEmitNotExpression }
func (en EmitNotExpression) Phase() Phase   { return PhaseCheck }
func (en EmitNotExpression) Pos() token.Pos { return en.At }

func (en EmitNotExpression) Error() string {
	return render(en.Code(), en.At, "emit is a statement and cannot be used as a value")
}

type EmitOutsideRun struct {
	At token.Pos
}

func (eo EmitOutsideRun) Code() Code     { return CodeEmitOutsideRun }
func (eo EmitOutsideRun) Phase() Phase   { return PhaseCheck }
func (eo EmitOutsideRun) Pos() token.Pos { return eo.At }

func (eo EmitOutsideRun) Error() string {
	return render(eo.Code(), eo.At, "emit is only allowed inside Run()")
}

type ArgCountMismatch struct {
	At       token.Pos
	Expected int
	Got      int
}

func (ac ArgCountMismatch) Code() Code     { return CodeArgCountMismatch }
func (ac ArgCountMismatch) Phase() Phase   { return PhaseCheck }
func (ac ArgCountMismatch) Pos() token.Pos { return ac.At }

func (ac ArgCountMismatch) Error() string {
	return render(ac.Code(), ac.At, "expected %d args, found %d", ac.Expected, ac.Got)
}

type ArgMissing struct {
	At       token.Pos
	Expected string
}

func (am ArgMissing) Code() Code     { return CodeArgMissing }
func (am ArgMissing) Phase() Phase   { return PhaseCheck }
func (am ArgMissing) Pos() token.Pos { return am.At }

func (am ArgMissing) Error() string {
	return render(am.Code(), am.At, "missing %s arg", am.Expected)
}

type ArgDuplicate struct {
	At   token.Pos
	Name string
}

func (da ArgDuplicate) Code() Code     { return CodeArgDuplicate }
func (da ArgDuplicate) Phase() Phase   { return PhaseCheck }
func (da ArgDuplicate) Pos() token.Pos { return da.At }

func (da ArgDuplicate) Error() string {
	return render(da.Code(), da.At, "%s arg passed more than once", da.Name)
}

type ArgUnknown struct {
	At   token.Pos
	Name string
}

func (uk ArgUnknown) Code() Code     { return CodeArgUnknown }
func (uk ArgUnknown) Phase() Phase   { return PhaseCheck }
func (uk ArgUnknown) Pos() token.Pos { return uk.At }

func (uk ArgUnknown) Error() string {
	return render(uk.Code(), uk.At, "unknown keyword argument %s", uk.Name)
}

type TypeMismatch struct {
	At       token.Pos
	Expected registry.TypeID
	Got      registry.TypeID
}

func (tm TypeMismatch) Code() Code     { return CodeTypeMismatch }
func (tm TypeMismatch) Phase() Phase   { return PhaseCheck }
func (tm TypeMismatch) Pos() token.Pos { return tm.At }

func (tm TypeMismatch) Error() string {
	return render(tm.Code(), tm.At, "expected %s, found %s", tm.Expected, tm.Got)
}

type RuntimeFailure struct {
	At      token.Pos
	Kind    registry.ErrorKind
	Message string
}

func (rf RuntimeFailure) Code() Code     { return Code(rf.Kind) }
func (rf RuntimeFailure) Phase() Phase   { return PhaseRuntime }
func (rf RuntimeFailure) Pos() token.Pos { return rf.At }

func (rf RuntimeFailure) Error() string {
	return render(rf.Code(), rf.At, "%s", rf.Message)
}

// Not a script problem: the registry was already written by another compile.
type TypeRegistrationFail struct {
	At     token.Pos
	Name   string
	Reason string
}

func (tr TypeRegistrationFail) Code() Code     { return CodeTypeRegistrationFail }
func (tr TypeRegistrationFail) Phase() Phase   { return PhaseCheck }
func (tr TypeRegistrationFail) Pos() token.Pos { return tr.At }

func (tr TypeRegistrationFail) Error() string {
	return render(tr.Code(), tr.At, "could not register type %s: %s", tr.Name, tr.Reason)
}

// InternalFailure is a recovered panic below an entry point:
// either core interpreter code hitting a resolver-guaranteed-impossible state,
// or a registered rule that panicked instead of returning an error.
type InternalFailure struct {
	EntryFn string
	Panic   any
	Stack   []byte
}

func (in InternalFailure) Code() Code     { return CodeInternalFailure }
func (in InternalFailure) Phase() Phase   { return PhaseRuntime }
func (in InternalFailure) Pos() token.Pos { return token.Pos{} }

func (in InternalFailure) Error() string {
	return fmt.Sprintf("error[%s] unrecovered panic in %s: %v\n%s", in.Code(), in.EntryFn, in.Panic, in.Stack)
}

type OutputUnknown struct {
	Name string
}

func (ou OutputUnknown) Code() Code     { return CodeOutputUnknown }
func (ou OutputUnknown) Phase() Phase   { return PhaseRuntime }
func (ou OutputUnknown) Pos() token.Pos { return token.Pos{} }

func (ou OutputUnknown) Error() string {
	return fmt.Sprintf("error[%s] output %q is not declared", ou.Code(), ou.Name)
}

type OutputMissing struct {
	At   token.Pos
	Name string
}

func (om OutputMissing) Code() Code     { return CodeOutputMissing }
func (om OutputMissing) Phase() Phase   { return PhaseRuntime }
func (om OutputMissing) Pos() token.Pos { return om.At }

func (om OutputMissing) Error() string {
	return render(om.Code(), om.At, "output %s has no bound sink", om.Name)
}

type InputUnknown struct {
	Name string
}

func (iu InputUnknown) Code() Code     { return CodeInputUnknown }
func (iu InputUnknown) Phase() Phase   { return PhaseRuntime }
func (iu InputUnknown) Pos() token.Pos { return token.Pos{} }

func (iu InputUnknown) Error() string {
	return fmt.Sprintf("error[%s] input %q is not declared", iu.Code(), iu.Name)
}

type InputMissing struct {
	At   token.Pos
	Name string
}

func (im InputMissing) Code() Code     { return CodeInputMissing }
func (im InputMissing) Phase() Phase   { return PhaseRuntime }
func (im InputMissing) Pos() token.Pos { return im.At }

func (im InputMissing) Error() string {
	return render(im.Code(), im.At, "input %s was not supplied", im.Name)
}

type InputTypeMismatch struct {
	At       token.Pos
	Name     string
	Expected registry.TypeID
	Got      registry.TypeID
}

func (im InputTypeMismatch) Code() Code     { return CodeInputTypeMismatch }
func (im InputTypeMismatch) Phase() Phase   { return PhaseRuntime }
func (im InputTypeMismatch) Pos() token.Pos { return im.At }

func (im InputTypeMismatch) Error() string {
	return render(im.Code(), im.At, "input %s: expected %s, found %s", im.Name, im.Expected, im.Got)
}

type OutputTypeMismatch struct {
	At       token.Pos
	Name     string
	Expected registry.TypeID
	Got      registry.TypeID
}

func (om OutputTypeMismatch) Code() Code     { return CodeOutputTypeMismatch }
func (om OutputTypeMismatch) Phase() Phase   { return PhaseRuntime }
func (om OutputTypeMismatch) Pos() token.Pos { return om.At }

func (om OutputTypeMismatch) Error() string {
	return render(om.Code(), om.At, "output %s: sink accepts %s, port declares %s", om.Name, om.Got, om.Expected)
}

type SlotTypeMismatch struct {
	At       token.Pos
	Kind     string
	Name     string
	Expected registry.TypeID
	Got      registry.TypeID
}

func (sm SlotTypeMismatch) Code() Code     { return CodeSlotTypeMismatch }
func (sm SlotTypeMismatch) Phase() Phase   { return PhaseRuntime }
func (sm SlotTypeMismatch) Pos() token.Pos { return sm.At }

func (sm SlotTypeMismatch) Error() string {
	return render(sm.Code(), sm.At, "%s %s: expected %s, found %s", sm.Kind, sm.Name, sm.Expected, sm.Got)
}
