package registry

import (
	"fmt"

	"github.com/MoroZvlg/tascript/token"
)

type BinaryRule struct {
	EvalFn   func(left, right Value) Value
	EvalType TypeID
}

type UnaryRule struct {
	EvalFn   func(right Value) Value
	EvalType TypeID
}

type MemberAccessRule struct {
	EvalFn   func(object TypeID, method string) Value
	EvalType TypeID
}

type BinaryKey struct {
	token       token.TokenType
	left, right TypeID
}

type UnaryKey struct {
	token token.TokenType
	right TypeID
}

type Registry struct {
	Binary map[BinaryKey]BinaryRule
	Unary  map[UnaryKey]UnaryRule
	Types  map[TypeID]TypeDef
}

func DefaultRegistry() *Registry {
	reg := &Registry{
		Binary: make(map[BinaryKey]BinaryRule),
		Unary:  make(map[UnaryKey]UnaryRule),
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

func (r *Registry) RegisterUnary(tok token.TokenType, right TypeID, rule UnaryRule) error {
	key := UnaryKey{
		token: tok,
		right: right,
	}
	r.Unary[key] = rule
	return nil
}

func (r *Registry) LookupUnary(tok token.TokenType, right TypeID) (UnaryRule, bool) {
	key := UnaryKey{
		token: tok,
		right: right,
	}
	rule, ok := r.Unary[key]
	return rule, ok
}
