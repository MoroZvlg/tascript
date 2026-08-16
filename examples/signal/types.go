package main

import (
	"fmt"

	"github.com/MoroZvlg/tascript/registry"
)

type Candle struct {
	T                      registry.TypeID
	Open, High, Low, Close float64
}

func (c Candle) TypeID() registry.TypeID { return c.T }

type CandleSeries struct {
	T    registry.TypeID
	Bars []Candle
}

func (cs *CandleSeries) TypeID() registry.TypeID { return cs.T }

func (cs *CandleSeries) At(i int) (Candle, error) {
	if i < 0 || i >= len(cs.Bars) {
		return Candle{}, fmt.Errorf("candle index %d out of range, %d bars available", i, len(cs.Bars))
	}
	return cs.Bars[len(cs.Bars)-1-i], nil
}

type Source struct {
	T      registry.TypeID
	Select func(Candle) float64
}

func (s Source) TypeID() registry.TypeID { return s.T }

type Indicator struct {
	T      registry.TypeID
	Period int
	Source Source
	window []float64
}

func (ind *Indicator) TypeID() registry.TypeID { return ind.T }

func (ind *Indicator) Next(c Candle) float64 {
	ind.window = append(ind.window, ind.Source.Select(c))
	if len(ind.window) > ind.Period {
		ind.window = ind.window[1:]
	}
	return mean(ind.window)
}

func (ind *Indicator) Current(c Candle) float64 {
	window := append(append([]float64{}, ind.window...), ind.Source.Select(c))
	if len(window) > ind.Period {
		window = window[1:]
	}
	return mean(window)
}

func (ind *Indicator) IsIdle() bool { return len(ind.window) < ind.Period }

func mean(values []float64) float64 {
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}
