package evaluator

import (
	"fmt"

	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/object"
)

var (
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
	NULL  = &object.Null{}
)

func Eval(node ast.Node, env *object.Environment) object.Object {
	switch n := node.(type) {
	case *ast.Program:
		return evalProgram(n, env)

	case *ast.ExpressionStatement:
		return Eval(n.Expression, env)

	case *ast.IntegerLiteral:
		return &object.Integer{Value: n.Value}

	case *ast.FloatLiteral:
		return &object.Float{Value: n.Value}

	case *ast.StringLiteral:
		return &object.String{Value: n.Value}

	case *ast.Boolean:
		return nativeBool(n.Value)

	case *ast.PrefixExpression:
		right := Eval(n.Right, env)
		if object.IsError(right) {
			return right
		}
		return evalPrefix(n.Operator, right)

	case *ast.InfixExpression:
		left := Eval(n.Left, env)
		if object.IsError(left) {
			return left
		}
		right := Eval(n.Right, env)
		if object.IsError(right) {
			return right
		}
		return evalInfix(n.Operator, left, right)

	case *ast.LetStatement:
		val := Eval(n.Value, env)
		if object.IsError(val) {
			return val
		}
		env.Set(n.Name.Value, val)
		return NULL

	case *ast.ConstStatement:
		val := Eval(n.Value, env)
		if object.IsError(val) {
			return val
		}
		env.Set(n.Name.Value, val)
		return NULL

	case *ast.Identifier:
		if val, ok := env.Get(n.Value); ok {
			return val
		}
		return newError("identifier not found: %s", n.Value)

	case *ast.IfExpression:
		cond := Eval(n.Condition, env)
		if object.IsError(cond) {
			return cond
		}
		if isTruthy(cond) {
			return Eval(n.Consequence, env)
		} else if n.Alternative != nil {
			return Eval(n.Alternative, env)
		}
		return NULL

	case *ast.BlockStatement:
		return evalBlock(n, env)
	}

	return newError("unknown node type: %T", node)
}

func evalProgram(p *ast.Program, env *object.Environment) object.Object {
	var result object.Object = NULL
	for _, stmt := range p.Statements {
		result = Eval(stmt, env)
		if object.IsError(result) {
			return result
		}
	}
	return result
}

func evalBlock(p *ast.BlockStatement, env *object.Environment) object.Object {
	var result object.Object = NULL
	for _, stmt := range p.Statements {
		result = Eval(stmt, env)
		if object.IsError(result) {
			return result
		}
	}
	return result
}

func nativeBool(b bool) *object.Boolean {
	if b {
		return TRUE
	}
	return FALSE
}

func evalPrefix(op string, right object.Object) object.Object {
	switch op {
	case "!":
		return evalBang(right)
	case "-":
		return evalMinusPrefix(right)
	}
	return newError("unknown operator: %s%s", op, right.Type())
}

func evalBang(o object.Object) object.Object {
	switch o {
	case TRUE:
		return FALSE
	case FALSE:
		return TRUE
	case NULL:
		return TRUE
	}
	return FALSE
}

func evalMinusPrefix(o object.Object) object.Object {
	switch v := o.(type) {
	case *object.Integer:
		return &object.Integer{Value: -v.Value}
	case *object.Float:
		return &object.Float{Value: -v.Value}
	}
	return newError("unknown operator: -%s", o.Type())
}

func evalInfix(op string, left, right object.Object) object.Object {
	if left.Type() == object.IntKind && right.Type() == object.FloatKind {
		promoted := &object.Float{Value: float64(left.(*object.Integer).Value)}
		return evalFloatInfix(op, promoted, right)
	}
	if left.Type() == object.FloatKind && right.Type() == object.IntKind {
		promoted := &object.Float{Value: float64(right.(*object.Integer).Value)}
		return evalFloatInfix(op, left, promoted)
	}

	switch {
	case left.Type() == object.IntKind && right.Type() == object.IntKind:
		return evalIntInfix(op, left, right)
	case left.Type() == object.FloatKind && right.Type() == object.FloatKind:
		return evalFloatInfix(op, left, right)
	case left.Type() == object.StringKind && right.Type() == object.StringKind:
		return evalStringInfix(op, left, right)
	case left.Type() == object.BooleanKind && right.Type() == object.BooleanKind:
		return evalBoolInfix(op, left, right)
	}

	if left.Type() != right.Type() {
		return newError("type mismatch: %s %s %s", left.Type(), op, right.Type())
	}
	return newError("unknown operator: %s %s %s", left.Type(), op, right.Type())
}

func evalIntInfix(op string, left, right object.Object) object.Object {
	l := left.(*object.Integer).Value
	r := right.(*object.Integer).Value
	switch op {
	case "+":
		return &object.Integer{Value: l + r}
	case "-":
		return &object.Integer{Value: l - r}
	case "*":
		return &object.Integer{Value: l * r}
	case "/":
		if r == 0 {
			return newError("division by zero")
		}
		return &object.Integer{Value: l / r}
	case "<":
		return nativeBool(l < r)
	case ">":
		return nativeBool(l > r)
	case "<=":
		return nativeBool(l <= r)
	case ">=":
		return nativeBool(l >= r)
	case "==":
		return nativeBool(l == r)
	case "!=":
		return nativeBool(l != r)
	}
	return newError("unknown operator: int %s int", op)
}

func evalFloatInfix(op string, left, right object.Object) object.Object {
	l := left.(*object.Float).Value
	r := right.(*object.Float).Value
	switch op {
	case "+":
		return &object.Float{Value: l + r}
	case "-":
		return &object.Float{Value: l - r}
	case "*":
		return &object.Float{Value: l * r}
	case "/":
		if r == 0 {
			return newError("division by zero")
		}
		return &object.Float{Value: l / r}
	case "<":
		return nativeBool(l < r)
	case ">":
		return nativeBool(l > r)
	case "<=":
		return nativeBool(l <= r)
	case ">=":
		return nativeBool(l >= r)
	case "==":
		return nativeBool(l == r)
	case "!=":
		return nativeBool(l != r)
	}
	return newError("unknown operator: float %s float", op)
}

func evalStringInfix(op string, left, right object.Object) object.Object {
	l := left.(*object.String).Value
	r := right.(*object.String).Value
	switch op {
	case "+":
		return &object.String{Value: l + r}
	case "==":
		return nativeBool(l == r)
	case "!=":
		return nativeBool(l != r)
	}
	return newError("unknown operator: string %s string", op)
}

func evalBoolInfix(op string, left, right object.Object) object.Object {
	switch op {
	case "==":
		return nativeBool(left == right)
	case "!=":
		return nativeBool(left != right)
	}
	return newError("unknown operator: boolean %s boolean", op)
}

func newError(format string, args ...any) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, args...)}
}

func isTruthy(o object.Object) bool {
	switch o {
	case TRUE:
		return true
	case FALSE:
		return false
	case NULL:
		return false
	}
	return true // ints, floats, strings, etc all truthy
}
