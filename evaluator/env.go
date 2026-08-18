package evaluator

import "github.com/MoroZvlg/tascript/registry"

type Env struct {
	parent *Env
	values map[string]registry.Value
}

func EnvFromRegistry(reg *registry.Registry) *Env {
	env := &Env{values: make(map[string]registry.Value)}
	for name, rule := range reg.Modules() {
		env.values[name] = rule
	}
	return env
}

func NewEnclosedEnv(parent *Env) *Env {
	return &Env{parent: parent, values: make(map[string]registry.Value)}
}

func (e *Env) Get(name string) (registry.Value, bool) {
	if value, ok := e.values[name]; ok {
		return value, true
	}
	if e.parent != nil {
		return e.parent.Get(name)
	}
	return nil, false
}

func (e *Env) Set(name string, value registry.Value) registry.Value {
	e.values[name] = value
	return value
}

func (e *Env) Assign(name string, value registry.Value) bool {
	if _, ok := e.values[name]; ok {
		e.values[name] = value
		return true
	}
	if e.parent != nil {
		return e.parent.Assign(name, value)
	}
	return false
}
