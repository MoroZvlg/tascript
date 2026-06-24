package registry

import (
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
		reg.Types[id] = TypeDef{Shape: ScalarShape}
	}

	for _, operator := range arithmeticOperators {
		for _, pair := range numericTypePairs {
			resultType := pair.result
			if operator == token.SLASH {
				resultType = FloatID
			}
			reg.RegisterBinary(operator, pair.left, pair.right, BinaryRule{
				EvalFn:   evalNumericBinary,
				EvalType: resultType,
			})
		}
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

func evalStringConcat(operator token.TokenType, left, right Value) Value {
	return left.(String) + right.(String)
}

func evalNumericNegate(operator token.TokenType, operand Value) Value {
	switch operand := operand.(type) {
	case Integer:
		return -operand
	case Float:
		return -operand
	default:
		panic("operand is not numeric")
	}
}

func evalBoolNot(operator token.TokenType, operand Value) Value {
	return Bool(!operand.(Bool))
}

func evalNumericBinary(operator token.TokenType, left, right Value) Value {
	leftInt, leftIsInt := left.(Integer)
	rightInt, rightIsInt := right.(Integer)
	if leftIsInt && rightIsInt {
		switch operator {
		case token.PLUS:
			return leftInt + rightInt
		case token.MINUS:
			return leftInt - rightInt
		case token.ASTERISK:
			return leftInt * rightInt
		case token.SLASH:
			return Float(leftInt) / Float(rightInt)
		case token.PERCENT:
			return leftInt % rightInt
		}
	}

	leftFloat := asFloat(left)
	rightFloat := asFloat(right)
	switch operator {
	case token.PLUS:
		return Float(leftFloat + rightFloat)
	case token.MINUS:
		return Float(leftFloat - rightFloat)
	case token.ASTERISK:
		return Float(leftFloat * rightFloat)
	case token.SLASH:
		return Float(leftFloat / rightFloat)
	case token.PERCENT:
		return Float(math.Mod(leftFloat, rightFloat))
	default:
		panic("unsupported numeric operator: " + operator)
	}
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
