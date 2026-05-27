// Package registry defines tascript extension metadata used by analysis and
// evaluation. Hosts can build a registry with custom types, helpers, and
// indicators without modifying the parser.
package registry

import (
	"fmt"
	"math"
)

type Value any

type Tuple []Value

type Series interface {
	Current() (float64, error)
	History(n int) (float64, error)
}

type Candle struct {
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

type TypeSpec struct {
	Name  string
	Input bool
	Value bool
	Field bool
}

type HelperFunc func(args []Value) (Value, error)

// HelperLookbackFunc returns the maximum historical index a helper needs for
// one call. Static analysis calls it with literal/constant argument values;
// non-static arguments are passed as nil.
type HelperLookbackFunc func(args []Value) (int, error)

type Indicator interface {
	NextCandle(Candle) (Value, error)
}

type IndicatorFactory func(args []Value) (Indicator, error)

type ScalarIndicator interface {
	NextNumber(float64) (Value, error)
}

type ScalarIndicatorFactory func(args []Value) (ScalarIndicator, error)

type HelperSpec struct {
	Namespace string
	Name      string
	MinArgs   int
	MaxArgs   int // -1 means variadic.
	Eval      HelperFunc
	Lookback  HelperLookbackFunc
}

type IndicatorSpec struct {
	Name        string
	Receiver    []string
	MinArgs     int
	MaxArgs     int
	Scalar      bool
	Build       IndicatorFactory
	BuildScalar ScalarIndicatorFactory
	BuildInfo   any
}

type Registry struct {
	types      map[string]TypeSpec
	helpers    map[helperKey]HelperSpec
	indicators map[string]IndicatorSpec
}

type helperKey struct {
	namespace string
	name      string
}

func New() *Registry {
	return &Registry{
		types:      map[string]TypeSpec{},
		helpers:    map[helperKey]HelperSpec{},
		indicators: map[string]IndicatorSpec{},
	}
}

func Default() *Registry {
	r := New()
	must(r.RegisterType(TypeSpec{Name: "Number", Value: true, Field: true}))
	must(r.RegisterType(TypeSpec{Name: "String", Value: true, Field: true}))
	must(r.RegisterType(TypeSpec{Name: "Bool", Value: true, Field: true}))
	must(r.RegisterType(TypeSpec{Name: "Series", Input: true}))
	must(r.RegisterType(TypeSpec{Name: "CandleSeries", Input: true}))
	must(r.RegisterType(TypeSpec{Name: "Candle"}))
	must(r.RegisterIndicator(IndicatorSpec{Name: "ema", Receiver: []string{"CandleSeries", "Series"}, MinArgs: 1, MaxArgs: 1, Scalar: true, Build: newEMA, BuildScalar: newScalarEMA}))
	must(r.RegisterIndicator(IndicatorSpec{Name: "bb", Receiver: []string{"CandleSeries"}, MinArgs: 2, MaxArgs: 2, Build: newBB}))
	must(r.RegisterHelper(HelperSpec{
		Namespace: "math",
		Name:      "max",
		MinArgs:   1,
		MaxArgs:   -1,
		Eval:      max,
	}))
	must(r.RegisterHelper(HelperSpec{
		Namespace: "math",
		Name:      "min",
		MinArgs:   1,
		MaxArgs:   -1,
		Eval:      min,
	}))
	must(r.RegisterHelper(HelperSpec{Namespace: "math", Name: "abs", MinArgs: 1, MaxArgs: 1, Eval: unaryMath("math.abs", math.Abs)}))
	must(r.RegisterHelper(HelperSpec{Namespace: "math", Name: "sqrt", MinArgs: 1, MaxArgs: 1, Eval: unaryMath("math.sqrt", math.Sqrt)}))
	must(r.RegisterHelper(HelperSpec{Namespace: "math", Name: "floor", MinArgs: 1, MaxArgs: 1, Eval: unaryMath("math.floor", math.Floor)}))
	must(r.RegisterHelper(HelperSpec{Namespace: "math", Name: "ceil", MinArgs: 1, MaxArgs: 1, Eval: unaryMath("math.ceil", math.Ceil)}))
	must(r.RegisterHelper(HelperSpec{Namespace: "math", Name: "round", MinArgs: 1, MaxArgs: 1, Eval: unaryMath("math.round", math.Round)}))
	must(r.RegisterHelper(HelperSpec{Namespace: "math", Name: "pow", MinArgs: 2, MaxArgs: 2, Eval: pow}))
	must(r.RegisterHelper(HelperSpec{Namespace: "ta", Name: "crossover", MinArgs: 2, MaxArgs: 2, Eval: crossover, Lookback: fixedLookback(1)}))
	must(r.RegisterHelper(HelperSpec{Namespace: "ta", Name: "crossunder", MinArgs: 2, MaxArgs: 2, Eval: crossunder, Lookback: fixedLookback(1)}))
	must(r.RegisterHelper(HelperSpec{Namespace: "ta", Name: "rising", MinArgs: 2, MaxArgs: 2, Eval: rising, Lookback: periodLookback}))
	must(r.RegisterHelper(HelperSpec{Namespace: "ta", Name: "falling", MinArgs: 2, MaxArgs: 2, Eval: falling, Lookback: periodLookback}))
	must(r.RegisterHelper(HelperSpec{Namespace: "ta", Name: "highest", MinArgs: 2, MaxArgs: 2, Eval: highest, Lookback: periodLookback}))
	must(r.RegisterHelper(HelperSpec{Namespace: "ta", Name: "lowest", MinArgs: 2, MaxArgs: 2, Eval: lowest, Lookback: periodLookback}))
	return r
}

func (r *Registry) Clone() *Registry {
	if r == nil {
		return Default()
	}
	out := New()
	for name, spec := range r.types {
		out.types[name] = spec
	}
	for key, spec := range r.helpers {
		out.helpers[key] = spec
	}
	for name, spec := range r.indicators {
		out.indicators[name] = spec
	}
	return out
}

func (r *Registry) RegisterType(spec TypeSpec) error {
	if spec.Name == "" {
		return fmt.Errorf("tascript registry: type name is required")
	}
	if _, exists := r.types[spec.Name]; exists {
		return fmt.Errorf("tascript registry: type %q already registered", spec.Name)
	}
	r.types[spec.Name] = spec
	return nil
}

func (r *Registry) Type(name string) (TypeSpec, bool) {
	spec, ok := r.types[name]
	return spec, ok
}

func (r *Registry) RegisterHelper(spec HelperSpec) error {
	if spec.Namespace == "" || spec.Name == "" {
		return fmt.Errorf("tascript registry: helper namespace and name are required")
	}
	if spec.MaxArgs >= 0 && spec.MaxArgs < spec.MinArgs {
		return fmt.Errorf("tascript registry: helper %s.%s has invalid arg bounds", spec.Namespace, spec.Name)
	}
	key := helperKey{namespace: spec.Namespace, name: spec.Name}
	if _, exists := r.helpers[key]; exists {
		return fmt.Errorf("tascript registry: helper %s.%s already registered", spec.Namespace, spec.Name)
	}
	r.helpers[key] = spec
	return nil
}

func (r *Registry) Helper(namespace, name string) (HelperSpec, bool) {
	spec, ok := r.helpers[helperKey{namespace: namespace, name: name}]
	return spec, ok
}

func (r *Registry) RegisterIndicator(spec IndicatorSpec) error {
	if spec.Name == "" {
		return fmt.Errorf("tascript registry: indicator name is required")
	}
	if spec.MaxArgs >= 0 && spec.MaxArgs < spec.MinArgs {
		return fmt.Errorf("tascript registry: indicator %s has invalid arg bounds", spec.Name)
	}
	if _, exists := r.indicators[spec.Name]; exists {
		return fmt.Errorf("tascript registry: indicator %q already registered", spec.Name)
	}
	r.indicators[spec.Name] = spec
	return nil
}

func (r *Registry) Indicator(name string) (IndicatorSpec, bool) {
	spec, ok := r.indicators[name]
	return spec, ok
}

func ValidateArgCount(name string, min, max, got int) error {
	if got < min {
		return fmt.Errorf("%s requires at least %d argument(s), got %d", name, min, got)
	}
	if max >= 0 && got > max {
		return fmt.Errorf("%s accepts at most %d argument(s), got %d", name, max, got)
	}
	return nil
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func max(args []Value) (Value, error) {
	if err := ValidateArgCount("math.max", 1, -1, len(args)); err != nil {
		return nil, err
	}
	result, err := number("math.max", args[0])
	if err != nil {
		return nil, err
	}
	for _, arg := range args[1:] {
		n, err := number("math.max", arg)
		if err != nil {
			return nil, err
		}
		result = math.Max(result, n)
	}
	return result, nil
}

func min(args []Value) (Value, error) {
	if err := ValidateArgCount("math.min", 1, -1, len(args)); err != nil {
		return nil, err
	}
	result, err := number("math.min", args[0])
	if err != nil {
		return nil, err
	}
	for _, arg := range args[1:] {
		n, err := number("math.min", arg)
		if err != nil {
			return nil, err
		}
		result = math.Min(result, n)
	}
	return result, nil
}

func number(name string, v Value) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case Series:
		n, err := x.Current()
		if err != nil {
			return 0, err
		}
		return n, nil
	default:
		return 0, fmt.Errorf("%s arguments must be Number", name)
	}
}

func unaryMath(name string, fn func(float64) float64) HelperFunc {
	return func(args []Value) (Value, error) {
		if err := ValidateArgCount(name, 1, 1, len(args)); err != nil {
			return nil, err
		}
		n, err := number(name, args[0])
		if err != nil {
			return nil, err
		}
		return fn(n), nil
	}
}

func pow(args []Value) (Value, error) {
	if err := ValidateArgCount("math.pow", 2, 2, len(args)); err != nil {
		return nil, err
	}
	x, err := number("math.pow", args[0])
	if err != nil {
		return nil, err
	}
	y, err := number("math.pow", args[1])
	if err != nil {
		return nil, err
	}
	return math.Pow(x, y), nil
}

func crossover(args []Value) (Value, error) {
	if err := ValidateArgCount("ta.crossover", 2, 2, len(args)); err != nil {
		return nil, err
	}
	a0, a1, err := currPrev("ta.crossover", args[0])
	if err != nil {
		return nil, err
	}
	b0, b1, err := currPrev("ta.crossover", args[1])
	if err != nil {
		return nil, err
	}
	return a0 > b0 && a1 <= b1, nil
}

func crossunder(args []Value) (Value, error) {
	if err := ValidateArgCount("ta.crossunder", 2, 2, len(args)); err != nil {
		return nil, err
	}
	a0, a1, err := currPrev("ta.crossunder", args[0])
	if err != nil {
		return nil, err
	}
	b0, b1, err := currPrev("ta.crossunder", args[1])
	if err != nil {
		return nil, err
	}
	return a0 < b0 && a1 >= b1, nil
}

func rising(args []Value) (Value, error) {
	series, n, err := seriesAndLookback("ta.rising", args)
	if err != nil {
		return nil, err
	}
	prev, err := series.History(n)
	if err != nil {
		return nil, err
	}
	curr, err := series.Current()
	if err != nil {
		return nil, err
	}
	return curr > prev, nil
}

func falling(args []Value) (Value, error) {
	series, n, err := seriesAndLookback("ta.falling", args)
	if err != nil {
		return nil, err
	}
	prev, err := series.History(n)
	if err != nil {
		return nil, err
	}
	curr, err := series.Current()
	if err != nil {
		return nil, err
	}
	return curr < prev, nil
}

func highest(args []Value) (Value, error) {
	series, n, err := seriesAndLookback("ta.highest", args)
	if err != nil {
		return nil, err
	}
	hi, err := series.Current()
	if err != nil {
		return nil, err
	}
	for i := 1; i < n; i++ {
		v, err := series.History(i)
		if err != nil {
			return nil, err
		}
		hi = math.Max(hi, v)
	}
	return hi, nil
}

func lowest(args []Value) (Value, error) {
	series, n, err := seriesAndLookback("ta.lowest", args)
	if err != nil {
		return nil, err
	}
	lo, err := series.Current()
	if err != nil {
		return nil, err
	}
	for i := 1; i < n; i++ {
		v, err := series.History(i)
		if err != nil {
			return nil, err
		}
		lo = math.Min(lo, v)
	}
	return lo, nil
}

func currPrev(name string, v Value) (float64, float64, error) {
	switch x := v.(type) {
	case float64:
		return x, x, nil
	case Series:
		curr, err := x.Current()
		if err != nil {
			return 0, 0, err
		}
		prev, err := x.History(1)
		if err != nil {
			return 0, 0, err
		}
		return curr, prev, nil
	default:
		return 0, 0, fmt.Errorf("%s arguments must be Number or Series", name)
	}
}

func seriesAndLookback(name string, args []Value) (Series, int, error) {
	if err := ValidateArgCount(name, 2, 2, len(args)); err != nil {
		return nil, 0, err
	}
	series, ok := args[0].(Series)
	if !ok {
		return nil, 0, fmt.Errorf("%s first argument must be Series", name)
	}
	lookback, err := positiveInteger(name, args[1])
	if err != nil {
		return nil, 0, err
	}
	return series, lookback, nil
}

func positiveInteger(name string, v Value) (int, error) {
	n, err := number(name, v)
	if err != nil {
		return 0, err
	}
	if n < 1 || n != math.Trunc(n) {
		return 0, fmt.Errorf("%s lookback must be a positive integer", name)
	}
	return int(n), nil
}

func fixedLookback(n int) HelperLookbackFunc {
	return func(args []Value) (int, error) {
		return n, nil
	}
}

func periodLookback(args []Value) (int, error) {
	if len(args) < 2 || args[1] == nil {
		return 0, fmt.Errorf("TA helper lookback must be a positive integer literal or constant")
	}
	n, err := positiveInteger("TA helper", args[1])
	if err != nil {
		return 0, err
	}
	return n - 1, nil
}

type emaIndicator struct {
	alpha float64
	value float64
	ready bool
}

func newEMA(args []Value) (Indicator, error) {
	return newEMAState(args)
}

func newScalarEMA(args []Value) (ScalarIndicator, error) {
	return newEMAState(args)
}

func newEMAState(args []Value) (*emaIndicator, error) {
	if err := ValidateArgCount("ema", 1, 1, len(args)); err != nil {
		return nil, err
	}
	period, err := positiveInteger("ema", args[0])
	if err != nil {
		return nil, err
	}
	return &emaIndicator{alpha: 2 / float64(period+1)}, nil
}

func (i *emaIndicator) NextCandle(c Candle) (Value, error) {
	return i.NextNumber(c.Close)
}

func (i *emaIndicator) NextNumber(n float64) (Value, error) {
	if !i.ready {
		i.value = n
		i.ready = true
		return i.value, nil
	}
	i.value = n*i.alpha + i.value*(1-i.alpha)
	return i.value, nil
}

type bbIndicator struct {
	period int
	mult   float64
	values []float64
}

func newBB(args []Value) (Indicator, error) {
	if err := ValidateArgCount("bb", 2, 2, len(args)); err != nil {
		return nil, err
	}
	period, err := positiveInteger("bb", args[0])
	if err != nil {
		return nil, err
	}
	mult, err := number("bb", args[1])
	if err != nil {
		return nil, err
	}
	if mult < 0 {
		return nil, fmt.Errorf("bb multiplier must be non-negative")
	}
	return &bbIndicator{period: period, mult: mult}, nil
}

func (i *bbIndicator) NextCandle(c Candle) (Value, error) {
	i.values = append(i.values, c.Close)
	if len(i.values) > i.period {
		i.values = i.values[len(i.values)-i.period:]
	}

	var sum float64
	for _, v := range i.values {
		sum += v
	}
	middle := sum / float64(len(i.values))

	var variance float64
	for _, v := range i.values {
		delta := v - middle
		variance += delta * delta
	}
	stddev := math.Sqrt(variance / float64(len(i.values)))
	upper := middle + i.mult*stddev
	lower := middle - i.mult*stddev
	return Tuple{upper, middle, lower}, nil
}
