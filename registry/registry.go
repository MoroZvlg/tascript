package registry

import (
	"fmt"

	"github.com/MoroZvlg/tascript/token"
)

type BinaryRule struct {
	EvalFn   func(tok token.TokenType, left, right Value) Value
	EvalType TypeID
}

type BinaryKey struct {
	token       token.TokenType
	left, right TypeID
}

type Registry struct {
	Binary map[BinaryKey]BinaryRule
	Types  map[TypeID]TypeDef
}

func DefaultRegistry() *Registry {
	reg := &Registry{
		Binary: make(map[BinaryKey]BinaryRule),
		Types:  make(map[TypeID]TypeDef),
	}

	RegisterStdMath(reg)

	return reg
}

func (r *Registry) RegisterType(customType string) (TypeID, error) {
	id := TypeID{id: customType}
	if _, exists := r.Types[id]; exists {
		return TypeID{}, fmt.Errorf("custom type %s not registered. ID taken", customType)
	}
	r.Types[id] = TypeDef{}
	return id, nil
}

func (r *Registry) RegisterBinary(tok token.TokenType, left, right TypeID, rule BinaryRule) error {
	key := BinaryKey{
		token: tok,
		left:  left,
		right: right,
	}
	r.Binary[key] = rule
	return nil
}

func (r *Registry) LookupBinary(tok token.TokenType, left, right TypeID) (BinaryRule, bool) {
	key := BinaryKey{
		token: tok,
		left:  left,
		right: right,
	}
	rule, ok := r.Binary[key]
	return rule, ok
}

func (r *Registry) ResolveType(tok token.TokenType, left, right TypeID) TypeID {
	rule, _ := r.LookupBinary(tok, left, right)
	return rule.EvalType
}

func (r *Registry) ResolveValue(tok token.TokenType, left, right Value) Value {
	rule, _ := r.LookupBinary(tok, left.TypeID(), right.TypeID())
	return rule.EvalFn(tok, left, right)
}
