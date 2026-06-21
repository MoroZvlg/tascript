package resolver

import "github.com/MoroZvlg/tascript/registry"

type Env struct {
	parent *Env
	values map[Symbol]registry.TypeID
}

func (s *Env) isTopLevel() bool {
	return s.parent == nil
}

func NewEnclosedEnv(scope *Env) *Env {
	return &Env{parent: scope, values: make(map[Symbol]registry.TypeID)}
}
