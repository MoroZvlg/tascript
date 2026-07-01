package registry

import "math"

func RegisterMathModule(reg *Registry) {
	mathModuleType, _ := reg.RegisterModule("math")

	reg.RegisterMemberAccess(mathModuleType.TypeID(), "PI", MemberAccessRule{
		EvalType: FloatID,
		EvalFn: func() Value {
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
		EvalFn: func(args map[string]Value) Value {
			switch args["number"].(type) {
			case Integer:
				return Float(math.Sqrt(float64(args["number"].(Integer))))
			case Float:
				return Float(math.Sqrt(float64(args["number"].(Float))))
			default:
				panic("unknown type")
			}
		},
	})
}
