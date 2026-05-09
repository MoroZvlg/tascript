package object

import "fmt"

type Limits struct {
	MaxSeriesLength int
	MaxStringLength int
}

var DefaultLimits = Limits{
	MaxSeriesLength: 1024,
	MaxStringLength: 1024,
}

type Environment struct {
	store map[string]Object
	outer *Environment
}

func NewEnvironment() *Environment {
	return &Environment{store: make(map[string]Object), outer: nil}
}

func NewEnclosedEnvironment(outer *Environment) *Environment {
	if outer == nil {
		outer = NewEnvironment()
	}
	return &Environment{store: make(map[string]Object), outer: outer}
}

func (e *Environment) Get(name string) (Object, bool) {
	obj, ok := e.store[name]
	if !ok && e.outer != nil {
		obj, ok = e.outer.Get(name)
	}
	return obj, ok
}

func (e *Environment) Set(name string, val Object) Object {
	e.store[name] = val
	return val
}

func (e *Environment) Limits() Limits {
	return DefaultLimits
}

func (e *Environment) Debug() {
	for key, val := range e.store {
		fmt.Println(key, ":", val)
	}
}
