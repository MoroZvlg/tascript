package registry

import (
	"cmp"
	"math"

	"github.com/MoroZvlg/tascript/token"
)

var arithmeticOperators = []token.TokenType{
	token.PLUS,
	token.MINUS,
	token.ASTERISK,
	token.SLASH,
	token.PERCENT,
}

var comparisonOperators = []token.TokenType{
	token.EQ,
	token.NEQ,
	token.LT,
	token.GT,
	token.LTEQ,
	token.GTEQ,
}

var numericTypePairs = []struct {
	left, right TypeID
	result      TypeID
}{
	{IntegerID, IntegerID, IntegerID},
	{IntegerID, FloatID, FloatID},
	{FloatID, IntegerID, FloatID},
	{FloatID, FloatID, FloatID},
}

func RegisterStdMath(reg *Registry) {
	for _, id := range []TypeID{IntegerID, FloatID, StringID, BoolID} {
		reg.types[id] = TypeDef{Shape: ScalarShape}
	}

	for _, operator := range arithmeticOperators {
		for _, pair := range numericTypePairs {
			resultType := pair.result
			if operator == token.SLASH {
				resultType = FloatID
			}
			reg.RegisterBinary(operator, pair.left, pair.right, BinaryRule{
				EvalFn:   makeNumericBinary(operator),
				EvalType: resultType,
			})
		}
	}

	for _, operator := range comparisonOperators {
		for _, pair := range numericTypePairs {
			reg.RegisterBinary(operator, pair.left, pair.right, BinaryRule{
				EvalFn:   makeNumericCompare(operator),
				EvalType: BoolID,
			})
		}
	}

	for _, operand := range []TypeID{StringID, BoolID} {
		reg.RegisterBinary(token.EQ, operand, operand, BinaryRule{
			EvalFn:   evalScalarEq,
			EvalType: BoolID,
		})
		reg.RegisterBinary(token.NEQ, operand, operand, BinaryRule{
			EvalFn:   evalScalarNeq,
			EvalType: BoolID,
		})
	}

	// String concatenation: "a" + "b" -> "ab". No other operator is defined
	// for strings (e.g. `*` repetition), so they fall through to InvalidBinaryOp.
	reg.RegisterBinary(token.PLUS, StringID, StringID, BinaryRule{
		EvalFn:   evalStringConcat,
		EvalType: StringID,
	})

	// Numeric negation: -Integer -> Integer, -Float -> Float.
	for _, operand := range []TypeID{IntegerID, FloatID} {
		reg.RegisterUnary(token.MINUS, operand, UnaryRule{
			EvalFn:   evalNumericNegate,
			EvalType: operand,
		})
	}

	// Logical not: !Bool -> Bool.
	reg.RegisterUnary(token.BANG, BoolID, UnaryRule{
		EvalFn:   evalBoolNot,
		EvalType: BoolID,
	})
}

func evalStringConcat(left, right Value) (Value, error) {
	return left.(String) + right.(String), nil
}

func evalNumericNegate(operand Value) (Value, error) {
	switch operand := operand.(type) {
	case Integer:
		return -operand, nil
	case Float:
		return -operand, nil
	default:
		panic("operand is not numeric")
	}
}

func evalBoolNot(operand Value) (Value, error) {
	return !operand.(Bool), nil
}

func makeNumericBinary(operator token.TokenType) func(left, right Value) (Value, error) {
	return func(left, right Value) (Value, error) {
		leftInt, leftIsInt := left.(Integer)
		rightInt, rightIsInt := right.(Integer)
		if leftIsInt && rightIsInt {
			switch operator {
			case token.PLUS:
				return leftInt + rightInt, nil
			case token.MINUS:
				return leftInt - rightInt, nil
			case token.ASTERISK:
				return leftInt * rightInt, nil
			case token.SLASH:
				if rightInt == 0 {
					return nil, Error{Kind: DivisionByZero, Message: "integer division by zero"}
				}
				return Float(leftInt) / Float(rightInt), nil
			case token.PERCENT:
				if rightInt == 0 {
					return nil, Error{Kind: DivisionByZero, Message: "integer modulo by zero"}
				}
				return leftInt % rightInt, nil
			}
		}

		leftFloat := asFloat(left)
		rightFloat := asFloat(right)
		switch operator {
		case token.PLUS:
			return Float(leftFloat + rightFloat), nil
		case token.MINUS:
			return Float(leftFloat - rightFloat), nil
		case token.ASTERISK:
			return Float(leftFloat * rightFloat), nil
		case token.SLASH:
			if rightFloat == 0 {
				return nil, Error{Kind: DivisionByZero, Message: "division by zero"}
			}
			return Float(leftFloat / rightFloat), nil
		case token.PERCENT:
			if rightFloat == 0 {
				return nil, Error{Kind: DivisionByZero, Message: "modulo by zero"}
			}
			return Float(math.Mod(leftFloat, rightFloat)), nil
		default:
			panic("unsupported numeric operator: " + operator)
		}
	}
}

func makeNumericCompare(operator token.TokenType) func(left, right Value) (Value, error) {
	return func(left, right Value) (Value, error) {
		leftInt, leftIsInt := left.(Integer)
		rightInt, rightIsInt := right.(Integer)
		if leftIsInt && rightIsInt {
			return evalCompare(operator, leftInt, rightInt), nil
		}
		return evalCompare(operator, asFloat(left), asFloat(right)), nil
	}
}

func evalCompare[T cmp.Ordered](operator token.TokenType, left, right T) Bool {
	switch operator {
	case token.EQ:
		return left == right
	case token.NEQ:
		return left != right
	case token.LT:
		return left < right
	case token.GT:
		return left > right
	case token.LTEQ:
		return left <= right
	case token.GTEQ:
		return left >= right
	default:
		panic("unsupported comparison operator: " + operator)
	}
}

func evalScalarEq(left, right Value) (Value, error) {
	return Bool(left == right), nil
}

func evalScalarNeq(left, right Value) (Value, error) {
	return Bool(left != right), nil
}

func asFloat(value Value) float64 {
	switch value := value.(type) {
	case Integer:
		return float64(value)
	case Float:
		return float64(value)
	default:
		panic("value is not numeric")
	}
}
