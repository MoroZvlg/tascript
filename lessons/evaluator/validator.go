package evaluator

import (
	"fmt"

	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/object"
)

type Validator struct {
	builtins map[string]string
	errors   []string
}

func NewValidator() *Validator {
	v := &Validator{
		builtins: map[string]string{
			"sma": "sma",
			"ema": "ema",
			"rsi": "rsi",
		},
	}
	return v
}

func (v *Validator) Errors() []string {
	return v.errors
}

// Validate walks the AST and returns the statically-inferred kind of the node.
// Most nodes don't have a meaningful kind — they return KindAny.
// Identifiers and let/const bindings are the interesting cases that propagate
// CandleSeriesKind through the scope.
func (v *Validator) Validate(node ast.Node, scope *object.Scope) object.ObjectType {
	switch n := node.(type) {

	case *ast.Program:
		for _, stmt := range n.Statements {
			v.Validate(stmt, scope)
		}
		return object.KindAny

	case *ast.ExpressionStatement:
		return v.Validate(n.Expression, scope)

	case *ast.LetStatement:
		rhsKind := v.Validate(n.Value, scope)
		scope.Set(n.Name.Value, rhsKind)
		return object.KindAny

	case *ast.ConstStatement:
		rhsKind := v.Validate(n.Value, scope)
		scope.Set(n.Name.Value, rhsKind)
		return object.KindAny

	case *ast.ReturnStatement:
		v.Validate(n.Value, scope)
		return object.KindAny

	case *ast.Identifier:
		if kind, ok := scope.Get(n.Value); ok {
			return kind
		}
		return object.KindAny

	case *ast.BlockStatement:
		inner := object.NewEnclosedScope(scope)
		for _, stmt := range n.Statements {
			v.Validate(stmt, inner)
		}
		return object.KindAny

	case *ast.IfExpression:
		v.Validate(n.Condition, scope)
		v.Validate(n.Consequence, scope)
		if n.Alternative != nil {
			v.Validate(n.Alternative, scope)
		}
		return object.KindAny

	case *ast.FunctionLiteral:
		// Walk the body at definition time so violations inside a function
		// that's never called still get reported. Params have unknown kinds
		// — callers could pass anything.
		inner := object.NewEnclosedScope(scope)
		for _, p := range n.Parameters {
			inner.Set(p.Value, object.KindAny)
		}
		v.Validate(n.Body, inner)
		return object.KindAny

	case *ast.FunctionCall:
		v.checkIndicatorCall(n, scope)
		v.Validate(n.Function, scope)
		for _, arg := range n.Arguments {
			v.Validate(arg, scope)
		}
		return object.KindAny

	// Recurse-only cases — no validation rules today, but children may contain
	// indicator calls that need checking.
	case *ast.PrefixExpression:
		v.Validate(n.Right, scope)
		return object.KindAny

	case *ast.InfixExpression:
		v.Validate(n.Left, scope)
		v.Validate(n.Right, scope)
		return object.KindAny

	case *ast.IndexExpression:
		v.Validate(n.Left, scope)
		v.Validate(n.Index, scope)
		return object.KindAny

	case *ast.MemberExpression:
		v.Validate(n.Object, scope)
		return object.KindAny

	// Leaves — no children, no kind to infer.
	case *ast.IntegerLiteral, *ast.FloatLiteral, *ast.StringLiteral, *ast.Boolean:
		return object.KindAny
	}

	return object.KindAny
}

// checkIndicatorCall is the place to enforce "first arg of sma/ema/rsi must be
// a CandleSeries". Stub for now — fill in.
func (v *Validator) checkIndicatorCall(call *ast.FunctionCall, scope *object.Scope) {
	ident, ok := call.Function.(*ast.Identifier)
	if !ok {
		return // not a function call. we don't care
	}
	_, exist := v.builtins[ident.Value]
	if !exist {
		return // not a builtin function. we don't care
	}
	if len(call.Arguments) < 1 {
		v.errors = append(v.errors, fmt.Sprintf("builtin func %s expects arguments but got none", ident.Value))
		return
	}
	if argType := v.Validate(call.Arguments[0], scope); argType != object.CandleSeriesKind {
		v.errors = append(v.errors, fmt.Sprintf("builtin func %s expects candle series as first argument, got %s", ident.Value, string(argType)))
	}
}
