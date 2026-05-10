package evaluator

import (
	"context"
	"fmt"

	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/object"
)

var (
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
	NULL  = &object.Null{}
)

// Op-budget state. A package-level counter is fine for a teaching interpreter
// but makes Eval non-reentrant — two scripts can't run concurrently in one process.
const opLimit = 10_000

var currentLiveBytes = 0

var currentOpCount = 0

func Eval(ctx context.Context, node ast.Node, env *object.Environment) object.Object {
	// Program is always the entry node — reset the counter and skip the budget check.
	if prog, ok := node.(*ast.Program); ok {
		currentOpCount = 0
		currentLiveBytes = 0
		return evalProgram(ctx, prog, env)
	}
	currentOpCount++
	if currentOpCount >= opLimit {
		return newError("operations limit exceeded (%d)", opLimit)
	}
	if ctx.Err() != nil {
		return newError("deadline exceeded: %s", ctx.Err())
	}
	switch n := node.(type) {
	case *ast.ExpressionStatement:
		return Eval(ctx, n.Expression, env)

	case *ast.IntegerLiteral:
		return &object.Integer{Value: n.Value}

	case *ast.FloatLiteral:
		return &object.Float{Value: n.Value}

	case *ast.StringLiteral:
		if err := enforceStringLength(env, len(n.Value)); err != nil {
			return err
		}
		return accountFor(&object.String{Value: n.Value})

	case *ast.Boolean:
		return nativeBool(n.Value)

	case *ast.PrefixExpression:
		right := Eval(ctx, n.Right, env)
		if object.IsError(right) {
			return right
		}
		return evalPrefix(n.Operator, right)

	case *ast.InfixExpression:
		left := Eval(ctx, n.Left, env)
		if object.IsError(left) {
			return left
		}
		right := Eval(ctx, n.Right, env)
		if object.IsError(right) {
			return right
		}
		return evalInfix(env, n.Operator, left, right)

	case *ast.LetStatement:
		val := Eval(ctx, n.Value, env)
		if object.IsError(val) {
			return val
		}
		env.Set(n.Name.Value, val)
		return val

	case *ast.ConstStatement:
		val := Eval(ctx, n.Value, env)
		if object.IsError(val) {
			return val
		}
		env.Set(n.Name.Value, val)
		return val

	case *ast.Identifier:
		if val, ok := env.Get(n.Value); ok {
			return val
		}
		return newError("identifier not found: %s", n.Value)

	case *ast.IfExpression:
		cond := Eval(ctx, n.Condition, env)
		if object.IsError(cond) {
			return cond
		}
		if isTruthy(cond) {
			return Eval(ctx, n.Consequence, env)
		} else if n.Alternative != nil {
			return Eval(ctx, n.Alternative, env)
		}
		return NULL

	case *ast.BlockStatement:
		return evalBlock(ctx, n, env)

	case *ast.FunctionLiteral:
		return &object.Function{Parameters: n.Parameters, Body: n.Body, Env: env}

	case *ast.ReturnStatement:
		result := Eval(ctx, n.Value, env)
		if object.IsError(result) {
			return result
		}
		return &object.Return{Value: result}

	case *ast.FunctionCall:
		fnObj := Eval(ctx, n.Function, env)
		if object.IsError(fnObj) {
			return fnObj
		}
		switch funcValue := fnObj.(type) {
		case *object.Function:
			return evalFunc(ctx, n, funcValue, env)
		case *object.Builtin:
			return evalBuiltin(ctx, n, funcValue, env)
		default:
			return newError("not a function: %s", n.Function.String())
		}

	case *ast.IndexExpression:
		leftVal := Eval(ctx, n.Left, env)
		if object.IsError(leftVal) {
			return leftVal
		}
		index := Eval(ctx, n.Index, env)
		if object.IsError(index) {
			return index
		}
		indexInt, ok := index.(*object.Integer)
		if !ok {
			return newError("index should be an integer, got %s", n.Index.TokenLiteral())
		}
		if indexInt.Value < 0 {
			return newError("index should be a positive integer, got %d", indexInt.Value)
		}

		switch leftObject := leftVal.(type) {
		case *object.Series:
			if int(indexInt.Value) > len(leftObject.Value)-1 {
				return newError("index out of range: %d", indexInt.Value)
			}
			pos := len(leftObject.Value) - int(indexInt.Value) - 1
			return &object.Float{Value: leftObject.Value[pos]}
		case *object.CandleSeries:
			if int(indexInt.Value) > len(leftObject.Value)-1 {
				return newError("index out of range: %d", indexInt.Value)
			}
			pos := len(leftObject.Value) - int(indexInt.Value) - 1
			return &leftObject.Value[pos]
		default:
			return newError("not an indexable object: %s", n.Left.String())
		}

	case *ast.MemberExpression:
		obj := Eval(ctx, n.Object, env)
		if object.IsError(obj) {
			return obj
		}
		switch ot := obj.(type) {
		case *object.String:
			switch n.Property.Value {
			case "length":
				return &object.Integer{Value: int64(len(ot.Value))}
			default:
				return newError("string has no property '%s'", n.Property.Value)
			}
		case *object.Series:
			switch n.Property.Value {
			case "length":
				return &object.Integer{Value: int64(len(ot.Value))}
			default:
				return newError("series has no property '%s'", n.Property.Value)
			}
		case *object.Candle:
			switch n.Property.Value {
			case "open":
				return &object.Float{Value: ot.Open}
			case "high":
				return &object.Float{Value: ot.High}
			case "low":
				return &object.Float{Value: ot.Low}
			case "close":
				return &object.Float{Value: ot.Close}
			case "volume":
				return &object.Float{Value: ot.Volume}
			default:
				return newError("Candle has no property '%s'", n.Property.Value)
			}
		case *object.CandleSeries:
			switch n.Property.Value {
			case "opens":
				return extractColumn(ot, func(c object.Candle) float64 { return c.Open })
			case "highs":
				return extractColumn(ot, func(c object.Candle) float64 { return c.High })
			case "lows":
				return extractColumn(ot, func(c object.Candle) float64 { return c.Low })
			case "closes":
				return extractColumn(ot, func(c object.Candle) float64 { return c.Close })
			case "volumes":
				return extractColumn(ot, func(c object.Candle) float64 { return c.Volume })
			default:
				return newError("CandleSeries has no property '%s'", n.Property.Value)
			}
		default:
			return newError("type %s has no properties", obj.Type())
		}
	}

	return newError("unknown node type: %T", node)
}

func evalProgram(ctx context.Context, p *ast.Program, env *object.Environment) object.Object {
	var result object.Object = NULL
	for _, stmt := range p.Statements {
		result = Eval(ctx, stmt, env)
		switch rTyped := result.(type) {
		case *object.Error:
			return rTyped
		case *object.Return:
			return rTyped.Value
		}
	}
	return result
}

func evalBuiltin(ctx context.Context, funcCall *ast.FunctionCall, funcVal *object.Builtin, env *object.Environment) object.Object {
	args := make([]object.Object, len(funcCall.Arguments))
	for i, arg := range funcCall.Arguments {
		argVal := Eval(ctx, arg, env)
		if object.IsError(argVal) {
			return argVal
		}
		args[i] = argVal
	}
	return funcVal.Fn(env, args)
}

func evalFunc(ctx context.Context, funcCall *ast.FunctionCall, funcVal *object.Function, env *object.Environment) object.Object {
	if len(funcCall.Arguments) != len(funcVal.Parameters) {
		return newError("argument(s) number mismatch. expected %d got %d", len(funcVal.Parameters), len(funcCall.Arguments))
	}
	funcEnv := object.NewEnclosedEnvironment(funcVal.Env)
	for i, arg := range funcCall.Arguments {
		argVal := Eval(ctx, arg, env)
		if object.IsError(argVal) {
			return argVal
		}
		funcEnv.Set(funcVal.Parameters[i].Value, argVal)
	}
	result := Eval(ctx, funcVal.Body, funcEnv)
	if rv, ok := result.(*object.Return); ok {
		return rv.Value
	}
	return result
}

func evalBlock(ctx context.Context, p *ast.BlockStatement, env *object.Environment) object.Object {
	var result object.Object = NULL
	for _, stmt := range p.Statements {
		result = Eval(ctx, stmt, env)
		if result != nil {
			rType := result.Type()
			if rType == object.ErrorKind || rType == object.ReturnKind {
				return result
			}
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

func evalInfix(env *object.Environment, op string, left, right object.Object) object.Object {
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
		return evalStringInfix(env, op, left, right)
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

func evalStringInfix(env *object.Environment, op string, left, right object.Object) object.Object {
	l := left.(*object.String).Value
	r := right.(*object.String).Value
	switch op {
	case "+":
		if err := enforceStringLength(env, len(l)+len(r)); err != nil {
			return err
		}
		return accountFor(&object.String{Value: l + r})
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
	case "&&":
		return nativeBool(left == TRUE && right == TRUE)
	case "||":
		return nativeBool(left == TRUE || right == TRUE)
	}
	return newError("unknown operator: boolean %s boolean", op)
}

func newError(format string, args ...any) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, args...)}
}

func extractColumn(cs *object.CandleSeries, pick func(object.Candle) float64) object.Object {
	out := make([]float64, len(cs.Value))
	for i, c := range cs.Value {
		out[i] = pick(c)
	}
	return accountFor(&object.Series{Value: out})
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

func enforceStringLength(env *object.Environment, n int) *object.Error {
	if n > env.Limits().MaxStringLength {
		return newError("string length %d exceeds limits %d.", n, env.Limits().MaxStringLength)
	}
	return nil
}

func enforceSeriesLength(env *object.Environment, n int) *object.Error {
	if n > env.Limits().MaxSeriesLength {
		return newError("series length %d exceeds limits %d.", n, env.Limits().MaxSeriesLength)
	}
	return nil
}

func accountFor(obj object.Object) object.Object {
	var size int
	switch v := obj.(type) {
	case *object.String:
		size = len(v.Value)
	case *object.Series:
		size = len(v.Value) * 8 // float64
	case *object.CandleSeries:
		size = len(v.Value) * 5 * 8 // 5 float64 fields per candle
	default:
		return obj
	}
	currentLiveBytes += size
	if currentLiveBytes > object.DefaultLimits.MaxLiveBytes {
		return newError("memory limit exceeded: %d bytes (limit %d)",
			currentLiveBytes, object.DefaultLimits.MaxLiveBytes)
	}
	return obj
}
