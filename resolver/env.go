package resolver

import "github.com/MoroZvlg/tascript/registry"

type BindingKind string

const (
	KindLet    BindingKind = "let"
	KindConst  BindingKind = "const"
	KindInput  BindingKind = "input"
	KindOutput BindingKind = "output"
	KindModule BindingKind = "module"
)

type Binding struct {
	T    registry.TypeID
	Kind BindingKind
}

func (b Binding) Assignable() bool {
	return b.Kind == KindLet
}

type Env struct {
	parent *Env
	values map[Symbol]Binding
}

func (s *Env) isTopLevel() bool {
	return s.parent == nil
}

func (s *Env) Get(key Symbol) (Binding, bool) {
	binding, ok := s.values[key]
	if ok {
		return binding, true
	}
	if s.isTopLevel() {
		return Binding{T: registry.ErrorTypeID}, false
	}
	return s.parent.Get(key)
}

func (s *Env) Set(key Symbol, binding Binding) {
	s.values[key] = binding
}

func EnvFromRegistry(reg *registry.Registry) *Env {
	env := &Env{values: make(map[Symbol]Binding)}
	for name, value := range reg.Modules {
		env.Set(Symbol(name), Binding{T: value.TypeID(), Kind: KindModule})
	}
	return env
}

func NewEnclosedEnv(scope *Env) *Env {
	return &Env{parent: scope, values: make(map[Symbol]Binding)}
}
