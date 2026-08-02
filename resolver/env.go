package resolver

import "github.com/MoroZvlg/tascript/registry"

type BindingKind string

const (
	KindLet      BindingKind = "let"
	KindConst    BindingKind = "const"
	KindInput    BindingKind = "input"
	KindOutput   BindingKind = "output"
	KindModule   BindingKind = "module"
	KindFunction BindingKind = "function"
	KindType     BindingKind = "type"
)

type Binding struct {
	T    registry.TypeID
	Kind BindingKind
}

func (b Binding) Assignable() bool {
	return b.Kind == KindLet
}

func (b Binding) Readable() bool {
	return b.Kind == KindLet || b.Kind == KindConst || b.Kind == KindInput
}

func (b Binding) Reserved() bool {
	// we have only builtin top level function and not allowing user to define its own funcs
	// if we will allow to defin function, reserved func names should be reworked
	return b.Kind == KindModule || b.Kind == KindFunction || b.Kind == KindType
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

// EnvFromRegistry adds reserved names to the env so they can't be used by user
// TODO: shape -> kind mapping probably belongs in the registry
func EnvFromRegistry(reg *registry.Registry) *Env {
	env := &Env{values: make(map[Symbol]Binding)}
	for id, def := range reg.Types {
		kind := KindType
		if def.Shape == registry.ModuleShape {
			kind = KindModule
		}
		env.Set(Symbol(id.String()), Binding{T: id, Kind: kind})
	}
	env.Set(Symbol("Run"), Binding{T: registry.NoTypeID, Kind: KindFunction})
	env.Set(Symbol("Init"), Binding{T: registry.NoTypeID, Kind: KindFunction})
	env.Set(Symbol("emit"), Binding{T: registry.NoTypeID, Kind: KindFunction})
	return env
}

func NewEnclosedEnv(scope *Env) *Env {
	return &Env{parent: scope, values: make(map[Symbol]Binding)}
}
