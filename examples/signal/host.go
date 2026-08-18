package main

import (
	"github.com/MoroZvlg/tascript"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/stdlib"
)

func registerHost(builder *tascript.Builder) error {
	reg := builder.Registry()
	stdlib.RegisterMath(reg)
	stdlib.RegisterTime(reg)

	for _, name := range []string{"Candle", "CandleSeries", "Indicator", "Source"} {
		if _, err := reg.RegisterScalarType(name); err != nil {
			return err
		}
	}

	for name, sel := range map[string]func(Candle) float64{
		"open":  func(c Candle) float64 { return c.Open },
		"high":  func(c Candle) float64 { return c.High },
		"low":   func(c Candle) float64 { return c.Low },
		"close": func(c Candle) float64 { return c.Close },
	} {
		if err := reg.RegisterMemberAccess(candleID, name, registry.MemberAccessRule{
			EvalType: registry.FloatID,
			EvalFn: func(receiver registry.Value) (registry.Value, error) {
				return registry.Float(sel(receiver.(Candle))), nil
			},
		}); err != nil {
			return err
		}
	}

	if err := reg.RegisterCall(seriesID, "at", registry.CallRule{
		Args:     []registry.ParamRule{{Type: registry.IntegerID, Name: "index"}},
		EvalType: candleID,
		EvalFn: func(receiver registry.Value, args map[string]registry.Value) (registry.Value, error) {
			candle, err := receiver.(*CandleSeries).At(int(args["index"].(registry.Integer)))
			if err != nil {
				return nil, registry.Error{Kind: registry.OutOfRange, Message: err.Error()}
			}
			return candle, nil
		},
	}); err != nil {
		return err
	}

	for name, advance := range map[string]func(*Indicator, Candle) float64{
		"Next":    (*Indicator).Next,
		"Current": (*Indicator).Current,
	} {
		if err := reg.RegisterCall(indicatorID, name, registry.CallRule{
			Args:     []registry.ParamRule{{Type: candleID, Name: "candle"}},
			EvalType: registry.FloatID,
			EvalFn: func(receiver registry.Value, args map[string]registry.Value) (registry.Value, error) {
				return registry.Float(advance(receiver.(*Indicator), args["candle"].(Candle))), nil
			},
		}); err != nil {
			return err
		}
	}

	ta, err := reg.RegisterModule("ta")
	if err != nil {
		return err
	}

	for name, sel := range map[string]func(Candle) float64{
		"Open":  func(c Candle) float64 { return c.Open },
		"High":  func(c Candle) float64 { return c.High },
		"Low":   func(c Candle) float64 { return c.Low },
		"Close": func(c Candle) float64 { return c.Close },
	} {
		source := Source{Select: sel}
		if err := reg.RegisterMemberAccess(ta.TypeID(), name, registry.MemberAccessRule{
			EvalType: sourceID,
			EvalFn: func(registry.Value) (registry.Value, error) {
				return source, nil
			},
		}); err != nil {
			return err
		}
	}

	if err := reg.RegisterCall(ta.TypeID(), "sma", registry.CallRule{
		Args: []registry.ParamRule{
			{Type: registry.IntegerID, Name: "period"},
			{Type: sourceID, Name: "source"},
		},
		EvalType: indicatorID,
		EvalFn: func(_ registry.Value, args map[string]registry.Value) (registry.Value, error) {
			return &Indicator{
				Period: int(args["period"].(registry.Integer)),
				Source: args["source"].(Source),
			}, nil
		},
	}); err != nil {
		return err
	}

	for _, kind := range []registry.DeclKind{
		{Word: "indicator", Initializer: registry.InitializerRequired, AllowedTypes: []registry.TypeID{indicatorID}},
		{Word: "setting", Initializer: registry.InitializerOptional, Namespaced: true},
		{Word: "state", Initializer: registry.InitializerOptional, Assignable: true, Namespaced: true},
	} {
		if err := reg.RegisterDeclKind(kind); err != nil {
			return err
		}
	}

	return nil
}
