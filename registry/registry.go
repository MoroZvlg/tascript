// Package registry defines tascript extension metadata used by analysis and
// evaluation. Hosts can build a registry with custom types, helpers, and
// indicators without modifying the parser.
package registry

import (
	"fmt"
	"math"
)

type Value any

type TypeSpec struct {
	Name  string
	Input bool
	Value bool
	Field bool
}

type HelperFunc func(args []Value) (Value, error)

type HelperSpec struct {
	Namespace string
	Name      string
	MinArgs   int
	MaxArgs   int // -1 means variadic.
	Eval      HelperFunc
}

type IndicatorSpec struct {
	Name      string
	Receiver  []string
	MinArgs   int
	MaxArgs   int
	Scalar    bool
	BuildInfo any
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
	n, ok := v.(float64)
	if !ok {
		return 0, fmt.Errorf("%s arguments must be Number", name)
	}
	return n, nil
}
