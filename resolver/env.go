package resolver

import "github.com/MoroZvlg/tascript/registry"

type Env struct {
	parent *Env
	values map[Symbol]registry.TypeID
}

func (s *Env) isTopLevel() bool {
	return s.parent == nil
}

func EnvFromRegistry(reg *registry.Registry) *Env {
	env := &Env{values: make(map[Symbol]registry.TypeID)}
	for name, value := range reg.Modules {
		env.values[Symbol(name)] = value.TypeID()
	}
	return env
}

func NewEnclosedEnv(scope *Env) *Env {
	return &Env{parent: scope, values: make(map[Symbol]registry.TypeID)}
}
