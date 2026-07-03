package registry

import "math"

func RegisterMathModule(reg *Registry) {
	mathModuleType, _ := reg.RegisterModule("math")

	reg.RegisterMemberAccess(mathModuleType.TypeID(), "PI", MemberAccessRule{
		EvalType: FloatID,
		EvalFn: func(_ Value) Value {
			return Float(math.Pi)
		},
	})

	reg.RegisterCall(mathModuleType.TypeID(), "sqrt", CallRule{
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
			return Float(math.Sqrt(float64(args["number"].(Float))))
		},
	})
}
