package resolver

import "github.com/MoroZvlg/tascript/registry"

type Env struct {
	parent *Env
	values map[Symbol]registry.TypeID
}

func (s *Env) isTopLevel() bool {
	return s.parent == nil
}

func (s *Env) Get(key Symbol) (registry.TypeID, bool) {
	value, ok := s.values[key]
	if ok {
		return value, true
	}
	if s.isTopLevel() {
		return registry.UnknownTypeID, false
	}
	return s.parent.Get(key)
}

func (s *Env) Set(key Symbol, value registry.TypeID) {
	s.values[key] = value
}

func EnvFromRegistry(reg *registry.Registry) *Env {
	env := &Env{values: make(map[Symbol]registry.TypeID)}
	for name, value := range reg.Modules {
		typeID := value.TypeID()
		env.Set(Symbol(name), typeID)
	}
	return env
}

func NewEnclosedEnv(scope *Env) *Env {
	return &Env{parent: scope, values: make(map[Symbol]registry.TypeID)}
}
