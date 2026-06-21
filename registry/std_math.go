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
