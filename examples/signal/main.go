package main

import (
	"fmt"

	"github.com/MoroZvlg/tascript"
	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/registry"
)

const src = `
input candles: CandleSeries

output signal: { side: String, price: Float }

setting fastPeriod: Integer = 3
setting slowPeriod: Integer = 5

indicator fast = ta.sma(setting.fastPeriod, source = ta.Close)
indicator slow = ta.sma(setting.slowPeriod, source = ta.Close)

state wasAbove: Bool = false

function Run() {
    let bar = candles.at(0)
    let above = fast.Next(bar) > slow.Next(bar)

    if (above && !state.wasAbove) {
        emit(signal, side = "long", price = bar.close)
    } else if (!above && state.wasAbove) {
        emit(signal, side = "short", price = bar.close)
    }

    state.wasAbove = above
}
`

func main() {
	builder := tascript.NewBuilder()

	ids, err := registerHost(builder)
	if err != nil {
		panic(err)
	}

	program, diags, err := builder.Compile(src)
	if err != nil {
		panic(err)
	}
	if len(diags) > 0 {
		showDiags(diags)
		return
	}
	series := &CandleSeries{T: ids.Series}
	if err := program.BindInput("candles", series); err != nil {
		panic(err)
	}

	router := &orderRouter{}
	if err := program.BindOutput("signal", router); err != nil {
		panic(err)
	}

	// the host overrides the script's default before Init: the initializer never runs
	slowPeriod, ok := program.Slot("setting", "slowPeriod")
	if !ok {
		panic("the script declares no slowPeriod setting")
	}
	if err := slowPeriod.Set(registry.Integer(7)); err != nil {
		panic(err)
	}

	if err := program.Init(); err != nil {
		panic(err)
	}
	showSlots(program)

	for _, price := range []float64{10, 11, 12, 13, 12, 10, 8, 7, 9, 12, 15} {
		series.Bars = append(series.Bars, Candle{T: ids.Candle, Close: price})

		router.bar = price
		if err := program.Run(); err != nil {
			panic(err)
		}
	}
}

type orderRouter struct {
	bar float64
}

func (r *orderRouter) Emit(value registry.Value) {
	signal := value.(registry.Record)
	fmt.Printf("bar %.1f -> %v %v\n", r.bar, signal.Fields["side"], signal.Fields["price"])
}

func showSlots(program *tascript.Executable) {
	for _, slot := range program.Slots() {
		value, err := slot.Get()
		if err != nil {
			fmt.Printf("%s %s: %s (%v)\n", slot.Kind(), slot.Name(), slot.Type(), err)
			continue
		}
		fmt.Printf("%s %s: %s = %v\n", slot.Kind(), slot.Name(), slot.Type(), value)
	}
}

func showDiags(diags []diag.Diagnostic) {
	for _, d := range diags {
		fmt.Println(d)
	}
}
