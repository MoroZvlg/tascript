package registry

import (
	"fmt"

	"github.com/MoroZvlg/tascript/token"
)

type BinaryRule struct {
	EvalFn   func(left, right Value) (Value, error)
	EvalType TypeID
}

type UnaryRule struct {
	EvalFn   func(right Value) (Value, error)
	EvalType TypeID
}

type MemberAccessRule struct {
	EvalFn   func(receiver Value) (Value, error)
	EvalType TypeID
}

type CallRule struct {
	Args     []ParamRule
	EvalFn   func(receiver Value, args map[string]Value) (Value, error)
	EvalType TypeID
}

type ParamRule struct {
	Type    TypeID
	Name    string
	Exact   bool
	Default Value
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

type CallKey struct {
	owner  TypeID
	method string
}

type GlobalRule struct {
	Type  TypeID
	Value Value
}

type CoerceKey struct {
	from, to TypeID
}

type CoerceRule struct {
	EvalType TypeID
	EvalFn   func(value Value) Value
}

type Registry struct {
	Binary       map[BinaryKey]BinaryRule
	Unary        map[UnaryKey]UnaryRule
	MemberAccess map[MemberAccessKey]MemberAccessRule
	Call         map[CallKey]CallRule
	Modules      map[string]*PlainModule
	Types        map[TypeID]TypeDef
	Coerces      map[CoerceKey]CoerceRule
}

func DefaultRegistry() *Registry {
	reg := &Registry{
		Binary:       make(map[BinaryKey]BinaryRule),
		Unary:        make(map[UnaryKey]UnaryRule),
		MemberAccess: make(map[MemberAccessKey]MemberAccessRule),
		Call:         make(map[CallKey]CallRule),
		Modules:      make(map[string]*PlainModule),
		Types:        make(map[TypeID]TypeDef),
		Coerces:      make(map[CoerceKey]CoerceRule),
	}

	for _, builtin := range []TypeID{IntegerID, FloatID, StringID, BoolID} {
		reg.Types[builtin] = TypeDef{Shape: ScalarShape}
	}

	RegisterStdMath(reg)
	RegisterMathModule(reg)

	reg.RegisterCoercion(IntegerID, FloatID, CoerceRule{
		EvalType: FloatID,
		EvalFn: func(value Value) Value {
			return Float(float64(value.(Integer)))
		},
	})

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

// RegisterScriptType registers structural type synthesized by the resolver from an inline `{field: Type}` declaration
func (r *Registry) RegisterScriptType(name string, fields []FieldDef) (TypeID, error) {
	id := TypeID{id: name}
	if _, exists := r.Types[id]; exists {
		return TypeID{}, fmt.Errorf("script type %s not registered. ID taken", name)
	}
	r.Types[id] = TypeDef{Fields: fields}

	for _, field := range fields {
		fieldName := field.Name
		r.RegisterMemberAccess(id, fieldName, MemberAccessRule{
			EvalType: field.Type,
			EvalFn: func(receiver Value) (Value, error) {
				return receiver.(Record).Fields[fieldName], nil
			},
		})
	}
	return id, nil
}

func (r *Registry) LookupType(name string) (TypeID, bool) {
	id := TypeID{id: name}
	_, exists := r.Types[id]
	if !exists {
		return TypeID{}, false
	}
	return id, true
}

func (r *Registry) LookupTypeDef(id TypeID) (TypeDef, bool) {
	def, exists := r.Types[id]
	return def, exists
}

// EmitRule is the call signature for emitting a value of the given output type:
// a single "value" param for named types, one param per field for structural ones.
func (r *Registry) EmitRule(outputType TypeID) CallRule {
	def, _ := r.LookupTypeDef(outputType)
	if len(def.Fields) == 0 {
		return CallRule{
			Args:     []ParamRule{{Type: outputType, Name: "value"}},
			EvalType: outputType,
		}
	}
	params := make([]ParamRule, 0, len(def.Fields))
	for _, field := range def.Fields {
		params = append(params, ParamRule{Type: field.Type, Name: field.Name})
	}
	return CallRule{Args: params, EvalType: outputType}
}

// TODO: Make check in every Register as we did in RegisterType

func (r *Registry) RegisterCoercion(from, to TypeID, rule CoerceRule) error {
	key := CoerceKey{from: from, to: to}
	r.Coerces[key] = rule
	return nil
}

func (r *Registry) LookupCoerce(from, to TypeID) (CoerceRule, bool) {
	key := CoerceKey{from: from, to: to}
	rule, ok := r.Coerces[key]
	return rule, ok
}

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

func (r *Registry) RegisterCall(owner TypeID, member string, rule CallRule) error {
	key := CallKey{
		owner:  owner,
		method: member,
	}
	r.Call[key] = rule
	return nil
}

func (r *Registry) LookupCall(owner TypeID, member string) (CallRule, bool) {
	key := CallKey{
		owner:  owner,
		method: member,
	}
	rule, ok := r.Call[key]
	return rule, ok
}
