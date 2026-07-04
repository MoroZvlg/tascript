package registry

import "math"

func RegisterMathModule(reg *Registry) {
	mathModuleType, _ := reg.RegisterModule("math")

	registerMathConst(reg, mathModuleType.TypeID(), "PI", math.Pi)
	registerMathConst(reg, mathModuleType.TypeID(), "E", math.E)

	registerUnaryMathFunc(reg, mathModuleType.TypeID(), "abs", math.Abs)
	registerUnaryMathFunc(reg, mathModuleType.TypeID(), "sqrt", math.Sqrt)
	registerUnaryMathFunc(reg, mathModuleType.TypeID(), "floor", math.Floor)
	registerUnaryMathFunc(reg, mathModuleType.TypeID(), "ceil", math.Ceil)
	registerUnaryMathFunc(reg, mathModuleType.TypeID(), "round", mathRound)
	registerUnaryMathFunc(reg, mathModuleType.TypeID(), "trunc", math.Trunc)
	// TODO: all methods returns float for now... not sure if it's correct
	registerUnaryMathFunc(reg, mathModuleType.TypeID(), "sign", mathSign)
	registerUnaryMathFunc(reg, mathModuleType.TypeID(), "log", math.Log)

	registerBinaryMathFunc(reg, mathModuleType.TypeID(), "max", "a", "b", math.Max)
	registerBinaryMathFunc(reg, mathModuleType.TypeID(), "min", "a", "b", math.Min)
	registerBinaryMathFunc(reg, mathModuleType.TypeID(), "pow", "base", "exponent", math.Pow)
}

func registerMathConst(reg *Registry, mathModule TypeID, name string, value float64) {
	reg.RegisterMemberAccess(mathModule, name, MemberAccessRule{
		EvalType: FloatID,
		EvalFn: func(_ Value) Value {
			return Float(value)
		},
	})
}

func registerUnaryMathFunc(reg *Registry, mathModule TypeID, name string, fn func(float64) float64) {
	reg.RegisterCall(mathModule, name, CallRule{
		Args: []ParamRule{
			{
				Type:    FloatID,
				Name:    "number",
				Exact:   false,
				Default: nil,
			},
		},
		EvalType: FloatID,
		EvalFn: func(_ Value, args map[string]Value) Value {
			return Float(fn(float64(args["number"].(Float))))
		},
	})
}

func registerBinaryMathFunc(reg *Registry, mathModule TypeID, name, leftName, rightName string, fn func(float64, float64) float64) {
	reg.RegisterCall(mathModule, name, CallRule{
		Args: []ParamRule{
			{
				Type:    FloatID,
				Name:    leftName,
				Exact:   false,
				Default: nil,
			},
			{
				Type:    FloatID,
				Name:    rightName,
				Exact:   false,
				Default: nil,
			},
		},
		EvalType: FloatID,
		EvalFn: func(_ Value, args map[string]Value) Value {
			left := float64(args[leftName].(Float))
			right := float64(args[rightName].(Float))
			return Float(fn(left, right))
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
