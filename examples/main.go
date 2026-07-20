package main

import (
	"fmt"

	"github.com/MoroZvlg/tascript"
	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/token"
)

type Money struct {
	Cents    int
	Currency string
}

func (m Money) String() string {
	unit := m.Cents / 100
	cents := m.Cents % 100
	return fmt.Sprintf("%d.%d %s", unit, cents, m.Currency)
}

type TaScriptMoney struct {
	data       Money
	DataTypeID registry.TypeID
}

func (tam TaScriptMoney) TypeID() registry.TypeID {
	return tam.DataTypeID
}

func (tam TaScriptMoney) TypeShape() registry.TypeShape {
	return registry.ScalarShape
}

func main() {
	src := `const foo = math.sqrt(5, number=9.0)
function Run() { foo }
`
	engine, diags := tascript.NewEngine(src)
	if len(diags) > 0 {
		showDiags(diags)
		return
	}

	// NOTE: test operations injection
	err := engine.RegisterBinary(token.PLUS, registry.StringID, registry.StringID, registry.BinaryRule{
		EvalFn: func(left, right registry.Value) (registry.Value, error) {
			leftV := left.(registry.String)
			rightV := right.(registry.String)
			result := fmt.Sprintf("%s%s", leftV, rightV)
			return registry.String(result), nil
		},
		EvalType: registry.StringID,
	})
	if err != nil {
		panic(err)
	}

	//moneyID, err := engine.RegisterType("Money")
	//if err != nil {
	//	panic(err)
	//}
	//
	//err = engine.RegisterBinary(token.ASTERISK, moneyID, registry.FloatID, registry.BinaryRule{
	//	EvalFn: func(tok token.TokenType, left, right registry.Value) registry.Value {
	//		moneyV := left.(TaScriptMoney)
	//		rightF := float64(right.(registry.Float))
	//		resultCents := float64(moneyV.data.Cents) * rightF
	//		return TaScriptMoney{data: Money{int(resultCents), moneyV.data.Currency}, DataTypeID: moneyV.DataTypeID}
	//	},
	//	EvalType: moneyID,
	//})
	//if err != nil {
	//	panic(err)
	//}

	program, diags := engine.Compile()
	if len(diags) > 0 {
		showDiags(diags)
		return
	}

	result, err := program.Run()
	if err != nil {
		panic(err)
	}
	fmt.Println(result)
}

func showDiags(diags []diag.Diagnostic) {
	for _, d := range diags {
		fmt.Println(d)
	}
}
