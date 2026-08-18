package stdlib

import (
	"math"

	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/token"
)

var comparisonOperators = []token.TokenType{
	token.EQ,
	token.NEQ,
	token.LT,
	token.GT,
	token.LTEQ,
	token.GTEQ,
}

func registerTimeOperators(reg *registry.Registry) {
	// Time <-> Time.
	_ = reg.RegisterBinary(token.MINUS, TimeID, TimeID, registry.BinaryRule{
		EvalType: DurationID,
		EvalFn: func(left, right registry.Value) (registry.Value, error) {
			return Duration(asTime(left) - asTime(right)), nil
		},
	})

	// Time <-> Duration. Addition is commutative; subtraction is Time - Duration only
	// (Duration - Time is meaningless).
	_ = reg.RegisterBinary(token.PLUS, TimeID, DurationID, registry.BinaryRule{
		EvalType: TimeID,
		EvalFn: func(left, right registry.Value) (registry.Value, error) {
			return Time(asTime(left) + asDuration(right)), nil
		},
	})
	_ = reg.RegisterBinary(token.PLUS, DurationID, TimeID, registry.BinaryRule{
		EvalType: TimeID,
		EvalFn: func(left, right registry.Value) (registry.Value, error) {
			return Time(asDuration(left) + asTime(right)), nil
		},
	})
	_ = reg.RegisterBinary(token.MINUS, TimeID, DurationID, registry.BinaryRule{
		EvalType: TimeID,
		EvalFn: func(left, right registry.Value) (registry.Value, error) {
			return Time(asTime(left) - asDuration(right)), nil
		},
	})

	// Duration <-> Duration.
	_ = reg.RegisterBinary(token.PLUS, DurationID, DurationID, registry.BinaryRule{
		EvalType: DurationID,
		EvalFn: func(left, right registry.Value) (registry.Value, error) {
			return Duration(asDuration(left) + asDuration(right)), nil
		},
	})
	_ = reg.RegisterBinary(token.MINUS, DurationID, DurationID, registry.BinaryRule{
		EvalType: DurationID,
		EvalFn: func(left, right registry.Value) (registry.Value, error) {
			return Duration(asDuration(left) - asDuration(right)), nil
		},
	})

	// Scaling: Number * Duration and Duration * Number, both directions.
	registerDurationScale(reg, registry.IntegerID, DurationID)
	registerDurationScale(reg, registry.FloatID, DurationID)
	registerDurationScale(reg, DurationID, registry.IntegerID)
	registerDurationScale(reg, DurationID, registry.FloatID)

	// Duration / Number -> Duration.
	_ = reg.RegisterBinary(token.SLASH, DurationID, registry.IntegerID, registry.BinaryRule{
		EvalType: DurationID,
		EvalFn: func(left, right registry.Value) (registry.Value, error) {
			divisor := asInteger(right)
			if divisor == 0 {
				return nil, registry.Error{Kind: registry.DivisionByZero, Message: "duration division by zero"}
			}
			return Duration(roundHalfAway(float64(asDuration(left)) / float64(divisor))), nil
		},
	})
	_ = reg.RegisterBinary(token.SLASH, DurationID, registry.FloatID, registry.BinaryRule{
		EvalType: DurationID,
		EvalFn: func(left, right registry.Value) (registry.Value, error) {
			divisor := asFloat(right)
			if divisor == 0 {
				return nil, registry.Error{Kind: registry.DivisionByZero, Message: "duration division by zero"}
			}
			return Duration(roundHalfAway(float64(asDuration(left)) / divisor)), nil
		},
	})

	// Duration / Duration -> Float ratio.
	_ = reg.RegisterBinary(token.SLASH, DurationID, DurationID, registry.BinaryRule{
		EvalType: registry.FloatID,
		EvalFn: func(left, right registry.Value) (registry.Value, error) {
			divisor := asDuration(right)
			if divisor == 0 {
				return nil, registry.Error{Kind: registry.DivisionByZero, Message: "duration division by zero"}
			}
			return registry.Float(float64(asDuration(left)) / float64(divisor)), nil
		},
	})

	// Comparisons on Time and Duration.
	for _, operator := range comparisonOperators {
		_ = reg.RegisterBinary(operator, TimeID, TimeID, registry.BinaryRule{
			EvalType: registry.BoolID,
			EvalFn:   makeInt64Compare(operator, asTime),
		})
		_ = reg.RegisterBinary(operator, DurationID, DurationID, registry.BinaryRule{
			EvalType: registry.BoolID,
			EvalFn:   makeInt64Compare(operator, asDuration),
		})
	}

	// -Duration.
	_ = reg.RegisterUnary(token.MINUS, DurationID, registry.UnaryRule{
		EvalType: DurationID,
		EvalFn: func(operand registry.Value) (registry.Value, error) {
			return Duration(-asDuration(operand)), nil
		},
	})
}

// registerDurationScale registers `left * right` where exactly one operand is a
// Duration and the other a number. The result is a Duration with milliseconds
// rounded half away from zero.
func registerDurationScale(reg *registry.Registry, left, right registry.TypeID) {
	_ = reg.RegisterBinary(token.ASTERISK, left, right, registry.BinaryRule{
		EvalType: DurationID,
		EvalFn: func(l, r registry.Value) (registry.Value, error) {
			durationMs, factor := scaleOperands(l, r)
			return Duration(roundHalfAway(durationMs * factor)), nil
		},
	})
}

func scaleOperands(l, r registry.Value) (durationMs, factor float64) {
	if d, ok := l.(Duration); ok {
		return float64(d), asFloat(r)
	}
	return float64(asDuration(r)), asFloat(l)
}

func makeInt64Compare(operator token.TokenType, get func(registry.Value) int64) func(left, right registry.Value) (registry.Value, error) {
	return func(left, right registry.Value) (registry.Value, error) {
		return registry.Bool(compareInt64(operator, get(left), get(right))), nil
	}
}

func compareInt64(operator token.TokenType, left, right int64) bool {
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

func roundHalfAway(x float64) float64 {
	if x < 0 {
		return math.Ceil(x - 0.5)
	}
	return math.Floor(x + 0.5)
}
