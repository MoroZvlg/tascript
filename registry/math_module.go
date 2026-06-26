package registry

import "math"

func RegisterMathModule(reg *Registry) {
	mathModuleType, _ := reg.RegisterModule("math")

	reg.RegisterMemberAccess(mathModuleType.TypeID(), "PI", MemberAccessRule{
		Callable: false,
		EvalType: FloatID,
		EvalFn: func() Value {
			return Float(math.Pi)
		},
	})
}
