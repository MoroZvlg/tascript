package evaluator

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/MoroZvlg/talive"
	"github.com/MoroZvlg/tascript/object"
)

var SignalOutput io.Writer = os.Stdout

type ohlcvAdapter struct{ c *object.Candle }

func (a ohlcvAdapter) Open() float64        { return a.c.Open }
func (a ohlcvAdapter) High() float64        { return a.c.High }
func (a ohlcvAdapter) Low() float64         { return a.c.Low }
func (a ohlcvAdapter) Close() float64       { return a.c.Close }
func (a ohlcvAdapter) Volume() float64      { return a.c.Volume }
func (a ohlcvAdapter) Timestamp() time.Time { return time.Time{} }

func runIndicator(name string, env *object.Environment, args []object.Object, factory func(period int) (talive.Indicator, error)) object.Object {
	if len(args) != 2 {
		return newError("%s: wrong number of arguments. got=%d, want=2", name, len(args))
	}
	candles, ok := args[0].(*object.CandleSeries)
	if !ok {
		return newError("%s: first argument must be CandleSeries, got %s", name, args[0].Type())
	}
	period, ok := args[1].(*object.Integer)
	if !ok {
		return newError("%s: second argument must be Integer, got %s", name, args[1].Type())
	}
	if err := enforceSeriesLength(env, len(candles.Value)); err != nil {
		return err
	}
	ind, err := factory(int(period.Value))
	if err != nil {
		return newError("%s: %s", name, err.Error())
	}
	out := make([]float64, len(candles.Value))
	for i := range candles.Value {
		result := ind.Next(ohlcvAdapter{c: &candles.Value[i]})
		out[i] = result[0]
	}
	return &object.Series{Value: out}
}

func SmaBuiltin(env *object.Environment, args []object.Object) object.Object {
	return runIndicator("sma", env, args, func(p int) (talive.Indicator, error) { return talive.NewSMA(p) })
}

func EmaBuiltin(env *object.Environment, args []object.Object) object.Object {
	return runIndicator("ema", env, args, func(p int) (talive.Indicator, error) { return talive.NewEMA(p) })
}

func RsiBuiltin(env *object.Environment, args []object.Object) object.Object {
	return runIndicator("rsi", env, args, func(p int) (talive.Indicator, error) { return talive.NewRSI(p) })
}

func SignalBuiltin(env *object.Environment, args []object.Object) object.Object {
	if len(args) != 1 {
		return newError("signal: wrong number of arguments. got=%d, want=1", len(args))
	}
	name, ok := args[0].(*object.String)
	if !ok {
		return newError("signal: argument must be String, got %s", args[0].Type())
	}
	fmt.Fprintf(SignalOutput, "received signal: %s\n", name.Value)
	return NULL
}

func RegisterBuiltins(env *object.Environment) {
	env.Set("sma", &object.Builtin{Name: "sma", Fn: SmaBuiltin})
	env.Set("ema", &object.Builtin{Name: "ema", Fn: EmaBuiltin})
	env.Set("rsi", &object.Builtin{Name: "rsi", Fn: RsiBuiltin})
	env.Set("signal", &object.Builtin{Name: "signal", Fn: SignalBuiltin})
}
