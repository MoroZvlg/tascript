package resolver

import (
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/resolved"
	"github.com/MoroZvlg/tascript/token"
)

type BindingKind string

const (
	KindLet      BindingKind = "let"
	KindConst    BindingKind = "const"
	KindInput    BindingKind = "input"
	KindOutput   BindingKind = "output"
	KindModule   BindingKind = "module"
	KindFunction BindingKind = "function"
	KindType     BindingKind = "type"
	KindSlot     BindingKind = "slot"
	KindDeclWord BindingKind = "declaration keyword"
)

type Binding struct {
	T          registry.TypeID
	Kind       BindingKind
	Slot       *resolved.SlotDecl
	assignable bool
}

func (b Binding) Assignable() bool {
	return b.assignable
}

func (b Binding) KindLabel() BindingKind {
	if b.Slot != nil {
		return BindingKind(b.Slot.Kind)
	}
	return b.Kind
}

func (b Binding) Readable() bool {
	return b.Kind == KindLet || b.Kind == KindConst || b.Kind == KindInput || b.Kind == KindSlot
}

func (b Binding) Reserved() bool {
	// we have only builtin top level function and not allowing user to define its own funcs
	// if we will allow to defin function, reserved func names should be reworked
	return b.Kind == KindModule || b.Kind == KindFunction || b.Kind == KindType || b.Kind == KindDeclWord
}

// a qualified key can never collide with a user name: identifiers hold no dot
func namespacedSymbol(word, name string) Symbol {
	return Symbol(word + "." + name)
}

func slotSymbol(kind registry.DeclKind, name string) Symbol {
	if kind.Namespaced {
		return namespacedSymbol(kind.Word, name)
	}
	return Symbol(name)
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
	for _, kind := range reg.DeclKinds() {
		if kind.Namespaced {
			env.Set(Symbol(kind.Word), Binding{T: registry.NoTypeID, Kind: KindDeclWord})
		}
	}
	for _, ident := range token.ReservedIdents() {
		env.Set(Symbol(ident), Binding{T: registry.NoTypeID, Kind: KindFunction})
	}
	return env
}

func NewEnclosedEnv(scope *Env) *Env {
	return &Env{parent: scope, values: make(map[Symbol]Binding)}
}
