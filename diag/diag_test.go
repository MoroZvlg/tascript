package diag_test

import (
	"fmt"
	"testing"

	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/token"
)

func pos(line, col int) token.Pos {
	return token.Pos{Line: line, Col: col}
}

func TestDiagnostic_Render(t *testing.T) {
	tests := []struct {
		diag diag.Diagnostic
		want string
	}{
		{
			diag.UnexpectedToken{At: pos(2, 6), Expected: token.ASSIGN, Got: token.COLON},
			"error[UNEXPECTED_TOKEN] 2:6: expected =, found :",
		},
		{
			diag.ExpressionExpected{At: pos(3, 1), Got: token.RBRACE},
			"error[EXPRESSION_EXPECTED] 3:1: expected expression, found }",
		},
		{
			diag.NumberOutOfRange{At: pos(4, 2), Target: token.INTEGER, Literal: "99999999999999999999"},
			"error[NUMBER_OUT_OF_RANGE] 4:2: INTEGER literal 99999999999999999999 is out of range",
		},
		{
			diag.TypeExpected{At: pos(1, 12)},
			"error[TYPE_EXPECTED] 1:12: type or custom type expected",
		},
		{
			diag.EmptyCustomType{At: pos(1, 12)},
			"error[EMPTY_CUSTOM_TYPE] 1:12: custom type should contain at least 1 field",
		},
		{
			diag.ForbiddenFunction{At: pos(5, 1)},
			"error[FORBIDDEN_FUNCTION] 5:1: only Init and Run functions are allowed",
		},
		{
			diag.EmptyFunction{At: pos(5, 1)},
			"error[EMPTY_FUNCTION] 5:1: Run function is empty",
		},
		{
			diag.ArgOrderInvalid{At: pos(6, 14)},
			"error[ARG_ORDER_INVALID] 6:14: args after kwargs not allowed",
		},
		{
			diag.MissingRun{At: pos(1, 1)},
			"error[MISSING_RUN] 1:1: Run function is required",
		},
		{
			diag.TopDeclUnexpected{At: pos(1, 1)},
			"error[TOP_DECL_UNEXPECTED] 1:1: only declarations and Init/Run functions are allowed at the top level",
		},
		{
			diag.TopDeclMisplaced{At: pos(3, 3), Keyword: token.INPUT},
			"error[TOP_DECL_MISPLACED] 3:3: input declaration is only allowed at the top level",
		},
		{
			diag.NestingTooDeep{At: pos(2, 40)},
			"error[NESTING_TOO_DEEP] 2:40: expression nested too deep",
		},
		{
			diag.DuplicateDeclaration{At: pos(2, 1), Keyword: "input", Name: "btc"},
			"error[DUPLICATE_DECLARATION] 2:1: duplicate declaration of input btc",
		},
		{
			diag.ReservedName{At: pos(1, 7), Name: "math", Kind: "module"},
			"error[RESERVED_NAME] 1:7: math is a reserved module name",
		},
		{
			diag.SlotUndeclared{At: pos(4, 7), Kind: "state", Name: "cooldown"},
			"error[SLOT_UNDECLARED] 4:7: state.cooldown is not declared",
		},
		{
			diag.UnknownDeclKeyword{At: pos(1, 1), Word: "indicator"},
			"error[UNKNOWN_DECL_KEYWORD] 1:1: unknown declaration keyword indicator",
		},
		{
			diag.InitializerRequired{At: pos(1, 11), Kind: "indicator", Name: "fast"},
			"error[INITIALIZER_REQUIRED] 1:11: indicator fast requires an initializer",
		},
		{
			diag.InitializerForbidden{At: pos(1, 9), Kind: "setting", Name: "period"},
			"error[INITIALIZER_FORBIDDEN] 1:9: setting period cannot have an initializer",
		},
		{
			diag.TypeRequired{At: pos(1, 9), Kind: "setting", Name: "period"},
			"error[TYPE_REQUIRED] 1:9: setting period needs a type annotation or an initializer",
		},
		{
			diag.DeclTypeNotAllowed{At: pos(1, 11), Kind: "indicator", T: registry.StringID},
			"error[DECL_TYPE_NOT_ALLOWED] 1:11: indicator does not accept type String",
		},
		{
			diag.UseBeforeDeclaration{At: pos(2, 18), Name: "period"},
			"error[USE_BEFORE_DECLARATION] 2:18: period is referenced before its declaration",
		},
		{
			diag.InputInInit{At: pos(3, 12), Name: "btc"},
			"error[INPUT_IN_INIT] 3:12: input btc is not available before Run",
		},
		{
			diag.SlotTypeMismatch{At: pos(1, 9), Kind: "setting", Name: "period", Expected: registry.IntegerID, Got: registry.StringID},
			"error[SLOT_TYPE_MISMATCH] 1:9: setting period: expected Integer, found String",
		},
		{
			diag.InvalidBinaryOperation{At: pos(3, 4), Op: "+", Left: registry.IntegerID, Right: registry.StringID},
			"error[INVALID_OPERATION] 3:4: cannot apply + to Integer and String",
		},
		{
			diag.InvalidUnaryOperation{At: pos(3, 4), Op: "-", Right: registry.StringID},
			"error[INVALID_OPERATION] 3:4: cannot apply - to String",
		},
		{
			diag.UndefinedIdent{At: pos(2, 9), Name: "sig"},
			"error[UNDEFINED_IDENT] 2:9: unknown identifier sig",
		},
		{
			diag.UndefinedVar{At: pos(2, 1), Name: "x"},
			"error[UNDEFINED_VAR] 2:1: unknown variable x",
		},
		{
			diag.NotReadable{At: pos(3, 9), Name: "alert", Kind: "output"},
			"error[NOT_READABLE] 3:9: cannot read output alert",
		},
		{
			diag.InvalidEmitTarget{At: pos(3, 5)},
			"error[INVALID_EMIT_TARGET] 3:5: emit target must be a declared output",
		},
		{
			diag.NotAssignable{At: pos(3, 1), Name: "THRESHOLD", Kind: "const"},
			"error[NOT_ASSIGNABLE] 3:1: cannot assign to const THRESHOLD",
		},
		{
			diag.InvalidAssignTarget{At: pos(3, 1)},
			"error[INVALID_ASSIGN_TARGET] 3:1: expression is not assignable",
		},
		{
			diag.UndefinedType{At: pos(1, 12), Name: "Foo"},
			"error[UNDEFINED_TYPE] 1:12: unknown type Foo",
		},
		{
			diag.UndefinedAttribute{At: pos(2, 14), Name: "TAU"},
			"error[UNDEFINED_ATTRIBUTE] 2:14: unknown attribute TAU",
		},
		{
			diag.UndefinedMethod{At: pos(2, 14), Name: "cbrt"},
			"error[UNDEFINED_METHOD] 2:14: unknown method cbrt",
		},
		{
			diag.NotIndexable{At: pos(2, 10), Left: registry.IntegerID},
			"error[NOT_INDEXABLE] 2:10: Integer is not indexable",
		},
		{
			diag.UndefinedFunc{At: pos(2, 9), Name: "foo"},
			"error[UNDEFINED_FUNC] 2:9: unknown function foo",
		},
		{
			diag.NotCallable{At: pos(2, 5)},
			"error[NOT_CALLABLE] 2:5: expression is not callable",
		},
		{
			diag.EmitNotExpression{At: pos(2, 9)},
			"error[EMIT_NOT_EXPRESSION] 2:9: emit is a statement and cannot be used as a value",
		},
		{
			diag.EmitOutsideRun{At: pos(2, 5)},
			"error[EMIT_OUTSIDE_RUN] 2:5: emit is only allowed inside Run()",
		},
		{
			diag.ArgCountMismatch{At: pos(2, 18), Expected: 1, Got: 2},
			"error[ARG_COUNT_MISMATCH] 2:18: expected 1 args, found 2",
		},
		{
			diag.ArgMissing{At: pos(2, 18), Expected: "price"},
			"error[ARG_MISSING] 2:18: missing price arg",
		},
		{
			diag.ArgDuplicate{At: pos(2, 24), Name: "dir"},
			"error[ARG_DUPLICATE] 2:24: dir arg passed more than once",
		},
		{
			diag.ArgUnknown{At: pos(2, 24), Name: "colour"},
			"error[ARG_UNKNOWN] 2:24: unknown keyword argument colour",
		},
		{
			diag.TypeMismatch{At: pos(2, 19), Expected: registry.FloatID, Got: registry.StringID},
			"error[TYPE_MISMATCH] 2:19: expected Float, found String",
		},
		{
			diag.RuntimeFailure{At: pos(3, 9), Kind: registry.DivisionByZero, Message: "integer division by zero"},
			"error[DIVISION_BY_ZERO] 3:9: integer division by zero",
		},
		{
			diag.InputUnknown{Name: "volume"},
			"error[INPUT_UNKNOWN] input \"volume\" is not declared",
		},
		{
			diag.InputMissing{At: pos(1, 1), Name: "price"},
			"error[INPUT_MISSING] 1:1: input price was not supplied",
		},
		{
			diag.InputTypeMismatch{At: pos(1, 1), Name: "price", Expected: registry.FloatID, Got: registry.StringID},
			"error[INPUT_TYPE_MISMATCH] 1:1: input price: expected Float, found String",
		},
		{
			diag.InternalFailure{EntryFn: "Run", Panic: "boom"},
			"error[INTERNAL_FAILURE] unrecovered panic in Run: boom\n",
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%T", tt.diag), func(t *testing.T) {
			if got := tt.diag.Error(); got != tt.want {
				t.Errorf("\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}
