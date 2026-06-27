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
		KWArgs:   map[string]TypeID{"foo": FloatID},
		EvalType: FloatID,
		EvalFn: func(_ []Value, kwArgs map[string]Value) Value {
			return Float(math.Sqrt(float64(kwArgs["foo"].(Float))))
		},
	})

	//reg.RegisterCall(mathModuleType.TypeID(), "sqrt", CallRule{
	//	Args:     []TypeID{IntegerID}, // TODO: it will override float. float stops working...
	//	EvalType: FloatID,
	//	EvalFn: func(args []Value, _ map[string]Value) Value {
	//		return Float(math.Sqrt(float64(args[0].(Integer))))
	//	},
	//})
}
