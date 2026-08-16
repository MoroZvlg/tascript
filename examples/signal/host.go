package main

import (
	"github.com/MoroZvlg/tascript"
	"github.com/MoroZvlg/tascript/registry"
)

type hostTypes struct {
	Candle    registry.TypeID
	Series    registry.TypeID
	Indicator registry.TypeID
	Source    registry.TypeID
}

func registerHost(builder *tascript.Builder) (hostTypes, error) {
	var ids hostTypes
	var err error

	if ids.Candle, err = builder.RegisterType("Candle"); err != nil {
		return ids, err
	}
	if ids.Series, err = builder.RegisterType("CandleSeries"); err != nil {
		return ids, err
	}
	if ids.Indicator, err = builder.RegisterType("Indicator"); err != nil {
		return ids, err
	}
	if ids.Source, err = builder.RegisterType("Source"); err != nil {
		return ids, err
	}

	for name, sel := range map[string]func(Candle) float64{
		"open":  func(c Candle) float64 { return c.Open },
		"high":  func(c Candle) float64 { return c.High },
		"low":   func(c Candle) float64 { return c.Low },
		"close": func(c Candle) float64 { return c.Close },
	} {
		if err = builder.RegisterMemberAccess(ids.Candle, name, registry.MemberAccessRule{
			EvalType: registry.FloatID,
			EvalFn: func(receiver registry.Value) (registry.Value, error) {
				return registry.Float(sel(receiver.(Candle))), nil
			},
		}); err != nil {
			return ids, err
		}
	}

	if err = builder.RegisterCall(ids.Series, "at", registry.CallRule{
		Args:     []registry.ParamRule{{Type: registry.IntegerID, Name: "index"}},
		EvalType: ids.Candle,
		EvalFn: func(receiver registry.Value, args map[string]registry.Value) (registry.Value, error) {
			candle, err := receiver.(*CandleSeries).At(int(args["index"].(registry.Integer)))
			if err != nil {
				return nil, registry.Error{Kind: registry.OutOfRange, Message: err.Error()}
			}
			return candle, nil
		},
	}); err != nil {
		return ids, err
	}

	if err = builder.RegisterCall(ids.Indicator, "Next", registry.CallRule{
		Args:     []registry.ParamRule{{Type: ids.Candle, Name: "candle"}},
		EvalType: registry.FloatID,
		EvalFn: func(receiver registry.Value, args map[string]registry.Value) (registry.Value, error) {
			return registry.Float(receiver.(*Indicator).Next(args["candle"].(Candle))), nil
		},
	}); err != nil {
		return ids, err
	}

	if err = builder.RegisterCall(ids.Indicator, "Current", registry.CallRule{
		Args:     []registry.ParamRule{{Type: ids.Candle, Name: "candle"}},
		EvalType: registry.FloatID,
		EvalFn: func(receiver registry.Value, args map[string]registry.Value) (registry.Value, error) {
			return registry.Float(receiver.(*Indicator).Current(args["candle"].(Candle))), nil
		},
	}); err != nil {
		return ids, err
	}

	ta, err := builder.RegisterModule("ta")
	if err != nil {
		return ids, err
	}

	for name, sel := range map[string]func(Candle) float64{
		"Open":  func(c Candle) float64 { return c.Open },
		"High":  func(c Candle) float64 { return c.High },
		"Low":   func(c Candle) float64 { return c.Low },
		"Close": func(c Candle) float64 { return c.Close },
	} {
		source := Source{T: ids.Source, Select: sel}
		if err = builder.RegisterMemberAccess(ta.TypeID(), name, registry.MemberAccessRule{
			EvalType: ids.Source,
			EvalFn: func(registry.Value) (registry.Value, error) {
				return source, nil
			},
		}); err != nil {
			return ids, err
		}
	}

	if err = builder.RegisterCall(ta.TypeID(), "sma", registry.CallRule{
		Args: []registry.ParamRule{
			{Type: registry.IntegerID, Name: "period"},
			{Type: ids.Source, Name: "source"},
		},
		EvalType: ids.Indicator,
		EvalFn: func(_ registry.Value, args map[string]registry.Value) (registry.Value, error) {
			return &Indicator{
				T:      ids.Indicator,
				Period: int(args["period"].(registry.Integer)),
				Source: args["source"].(Source),
			}, nil
		},
	}); err != nil {
		return ids, err
	}

	if err = builder.RegisterDeclKind(registry.DeclKind{
		Word:         "indicator",
		Initializer:  registry.InitializerRequired,
		AllowedTypes: []registry.TypeID{ids.Indicator},
	}); err != nil {
		return ids, err
	}

	if err = builder.RegisterDeclKind(registry.DeclKind{
		Word:        "setting",
		Initializer: registry.InitializerOptional,
		Namespaced:  true,
	}); err != nil {
		return ids, err
	}

	if err = builder.RegisterDeclKind(registry.DeclKind{
		Word:        "state",
		Initializer: registry.InitializerOptional,
		Assignable:  true,
		Namespaced:  true,
	}); err != nil {
		return ids, err
	}

	return ids, nil
}
