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
	Callable bool
	EvalFn   func() Value
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

type MemberAccessKey struct {
	owner  TypeID
	method string
}

type GlobalRule struct {
	Type  TypeID
	Value Value
}

type Registry struct {
	Binary       map[BinaryKey]BinaryRule
	Unary        map[UnaryKey]UnaryRule
	MemberAccess map[MemberAccessKey]MemberAccessRule
	Modules      map[string]*PlainModule
	Types        map[TypeID]TypeDef
}

func DefaultRegistry() *Registry {
	reg := &Registry{
		Binary:       make(map[BinaryKey]BinaryRule),
		Unary:        make(map[UnaryKey]UnaryRule),
		MemberAccess: make(map[MemberAccessKey]MemberAccessRule),
		Modules:      make(map[string]*PlainModule),
		Types:        make(map[TypeID]TypeDef),
	}

	RegisterStdMath(reg)
	RegisterMathModule(reg)

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

// TODO: Make check in every Register as we did in RegisterType

func (r *Registry) RegisterModule(name string) (Value, error) {
	moduleType, err := r.RegisterType(name)
	if err != nil {
		return nil, err
	}
	moduleValue := &PlainModule{typeID: moduleType}
	r.Modules[name] = moduleValue
	return moduleValue, nil
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

func (r *Registry) RegisterMemberAccess(owner TypeID, member string, rule MemberAccessRule) error {
	key := MemberAccessKey{
		owner:  owner,
		method: member,
	}
	r.MemberAccess[key] = rule
	return nil
}

func (r *Registry) LookupMemberAccess(owner TypeID, member string) (MemberAccessRule, bool) {
	key := MemberAccessKey{
		owner:  owner,
		method: member,
	}
	rule, ok := r.MemberAccess[key]
	return rule, ok
}
