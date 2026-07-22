package stdlib

import (
	"math"

	"github.com/MoroZvlg/tascript/registry"
)

func RegisterMath(reg *registry.Registry) {
	mathModule, _ := reg.RegisterModule("math")
	moduleType := mathModule.TypeID()

	registerMathConst(reg, moduleType, "PI", math.Pi)
	registerMathConst(reg, moduleType, "E", math.E)

	registerUnaryMathFunc(reg, moduleType, "abs", math.Abs)
	registerUnaryMathFunc(reg, moduleType, "sqrt", math.Sqrt)
	registerUnaryMathFunc(reg, moduleType, "floor", math.Floor)
	registerUnaryMathFunc(reg, moduleType, "ceil", math.Ceil)
	registerUnaryMathFunc(reg, moduleType, "round", mathRound)
	registerUnaryMathFunc(reg, moduleType, "trunc", math.Trunc)
	// TODO: all methods returns float for now... not sure if it's correct
	registerUnaryMathFunc(reg, moduleType, "sign", mathSign)
	registerUnaryMathFunc(reg, moduleType, "log", math.Log)

	registerBinaryMathFunc(reg, moduleType, "max", "a", "b", math.Max)
	registerBinaryMathFunc(reg, moduleType, "min", "a", "b", math.Min)
	registerBinaryMathFunc(reg, moduleType, "pow", "base", "exponent", math.Pow)
}

func registerMathConst(reg *registry.Registry, mathModule registry.TypeID, name string, value float64) {
	reg.RegisterMemberAccess(mathModule, name, registry.MemberAccessRule{
		EvalType: registry.FloatID,
		EvalFn: func(_ registry.Value) (registry.Value, error) {
			return registry.Float(value), nil
		},
	})
}

func registerUnaryMathFunc(reg *registry.Registry, mathModule registry.TypeID, name string, fn func(float64) float64) {
	reg.RegisterCall(mathModule, name, registry.CallRule{
		Args: []registry.ParamRule{
			{Type: registry.FloatID, Name: "number"},
		},
		EvalType: registry.FloatID,
		EvalFn: func(_ registry.Value, args map[string]registry.Value) (registry.Value, error) {
			return registry.Float(fn(float64(args["number"].(registry.Float)))), nil
		},
	})
}

func registerBinaryMathFunc(reg *registry.Registry, mathModule registry.TypeID, name, leftName, rightName string, fn func(float64, float64) float64) {
	reg.RegisterCall(mathModule, name, registry.CallRule{
		Args: []registry.ParamRule{
			{Type: registry.FloatID, Name: leftName},
			{Type: registry.FloatID, Name: rightName},
		},
		EvalType: registry.FloatID,
		EvalFn: func(_ registry.Value, args map[string]registry.Value) (registry.Value, error) {
			left := float64(args[leftName].(registry.Float))
			right := float64(args[rightName].(registry.Float))
			return registry.Float(fn(left, right)), nil
		},
	})
}

func mathRound(number float64) float64 {
	if math.IsNaN(number) || math.IsInf(number, 0) || number == 0 {
		return number
	}
	if number < 0 && number >= -0.5 {
		return math.Copysign(0, -1)
	}
	return math.Floor(number + 0.5)
}

func mathSign(number float64) float64 {
	if math.IsNaN(number) || number == 0 {
		return number
	}
	if number < 0 {
		return -1
	}
	return 1
}
