package registry

import (
	"errors"
	"fmt"
	"iter"
	"sort"

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
	Type  TypeID
	Name  string
	Exact bool
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

type InitializerRule uint8

const (
	InitializerRequired InitializerRule = iota
	InitializerOptional
	InitializerForbidden
)

type DeclKind struct {
	Word         string
	Initializer  InitializerRule
	Assignable   bool
	Namespaced   bool
	AllowedTypes []TypeID
}

type Registry struct {
	binary       map[BinaryKey]BinaryRule
	unary        map[UnaryKey]UnaryRule
	memberAccess map[MemberAccessKey]MemberAccessRule
	call         map[CallKey]CallRule
	modules      map[string]*PlainModule
	types        map[TypeID]TypeDef
	coerces      map[CoerceKey]CoerceRule
	declKinds    map[string]DeclKind
	sealed       bool
}

var ErrSealed = errors.New("registry is sealed: it already backs a compiled program")

func (r *Registry) Seal() { r.sealed = true }

func (r *Registry) checkOpen() error {
	if r.sealed {
		return ErrSealed
	}
	return nil
}

func (r *Registry) Types() iter.Seq2[TypeID, TypeDef] {
	return func(yield func(TypeID, TypeDef) bool) {
		for id, def := range r.types {
			if !yield(id, def) {
				return
			}
		}
	}
}

func (r *Registry) Modules() iter.Seq2[string, Value] {
	return func(yield func(string, Value) bool) {
		for name, module := range r.modules {
			if !yield(name, module) {
				return
			}
		}
	}
}

func (r *Registry) Module(name string) (Value, bool) {
	module, ok := r.modules[name]
	if !ok {
		return nil, false
	}
	return module, true
}

func DefaultRegistry() *Registry {
	reg := &Registry{
		binary:       make(map[BinaryKey]BinaryRule),
		unary:        make(map[UnaryKey]UnaryRule),
		memberAccess: make(map[MemberAccessKey]MemberAccessRule),
		call:         make(map[CallKey]CallRule),
		modules:      make(map[string]*PlainModule),
		types:        make(map[TypeID]TypeDef),
		coerces:      make(map[CoerceKey]CoerceRule),
		declKinds:    make(map[string]DeclKind),
	}

	for _, builtin := range []TypeID{IntegerID, FloatID, StringID, BoolID} {
		reg.types[builtin] = TypeDef{Shape: ScalarShape}
	}

	RegisterStdMath(reg)

	_ = reg.RegisterCoercion(IntegerID, FloatID, CoerceRule{
		EvalType: FloatID,
		EvalFn: func(value Value) Value {
			return Float(float64(asInteger(value)))
		},
	})

	return reg
}

func rejectReserved(ids ...TypeID) error {
	for _, id := range ids {
		if id == ErrorTypeID {
			return fmt.Errorf("type %s is reserved for error recovery", ErrorTypeID)
		}
	}
	return nil
}

func (r *Registry) RegisterType(id TypeID, shape TypeShape) error {
	if err := r.checkOpen(); err != nil {
		return err
	}
	if shape == ModuleShape {
		return fmt.Errorf("type %s not registered. modules must go through RegisterModule", id)
	}
	return r.registerType(id, shape)
}

func (r *Registry) RegisterScalarType(name string) (TypeID, error) {
	id := NewTypeID(name)
	return id, r.RegisterType(id, ScalarShape)
}

func (r *Registry) registerType(id TypeID, shape TypeShape) error {
	if err := rejectReserved(id); err != nil {
		return err
	}
	if err := r.rejectDeclKindWord(id.String()); err != nil {
		return err
	}
	if _, exists := r.types[id]; exists {
		return fmt.Errorf("type %s not registered. ID taken", id)
	}
	r.types[id] = TypeDef{Shape: shape}
	return nil
}

// RegisterScriptType registers structural type synthesized by the resolver from an inline `{field: Type}` declaration
func (r *Registry) RegisterScriptType(name string, fields []FieldDef) (TypeID, error) {
	if err := r.checkOpen(); err != nil {
		return TypeID{}, err
	}
	id := TypeID{id: name}
	if err := rejectReserved(id); err != nil {
		return TypeID{}, err
	}
	if err := r.rejectDeclKindWord(name); err != nil {
		return TypeID{}, err
	}
	for _, field := range fields {
		if err := rejectReserved(field.Type); err != nil {
			return TypeID{}, err
		}
	}
	if _, exists := r.types[id]; exists {
		return TypeID{}, fmt.Errorf("script type %s not registered. ID taken", name)
	}
	r.types[id] = TypeDef{Fields: fields}

	for _, field := range fields {
		fieldName := field.Name
		_ = r.RegisterMemberAccess(id, fieldName, MemberAccessRule{
			EvalType: field.Type,
			EvalFn: func(receiver Value) (Value, error) {
				return asRecord(receiver).Fields[fieldName], nil
			},
		})
	}
	return id, nil
}

func (r *Registry) LookupType(name string) (TypeID, bool) {
	id := TypeID{id: name}
	_, exists := r.types[id]
	if !exists {
		return TypeID{}, false
	}
	return id, true
}

func (r *Registry) LookupTypeDef(id TypeID) (TypeDef, bool) {
	def, exists := r.types[id]
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

func (r *Registry) RegisterCoercion(from, to TypeID, rule CoerceRule) error {
	if err := r.checkOpen(); err != nil {
		return err
	}
	if err := rejectReserved(from, to, rule.EvalType); err != nil {
		return err
	}
	key := CoerceKey{from: from, to: to}
	if _, exists := r.coerces[key]; exists {
		return fmt.Errorf("coercion %s -> %s already registered", from, to)
	}
	r.coerces[key] = rule
	return nil
}

func (r *Registry) LookupCoerce(from, to TypeID) (CoerceRule, bool) {
	key := CoerceKey{from: from, to: to}
	rule, ok := r.coerces[key]
	return rule, ok
}

func (r *Registry) RegisterModule(name string) (Value, error) {
	if err := r.checkOpen(); err != nil {
		return nil, err
	}
	moduleType := NewTypeID(name)
	if err := r.registerType(moduleType, ModuleShape); err != nil {
		return nil, err
	}
	moduleValue := &PlainModule{typeID: moduleType}
	r.modules[name] = moduleValue
	return moduleValue, nil
}

func (r *Registry) RegisterBinary(tok token.TokenType, left, right TypeID, rule BinaryRule) error {
	if err := r.checkOpen(); err != nil {
		return err
	}
	if err := rejectReserved(left, right, rule.EvalType); err != nil {
		return err
	}
	key := BinaryKey{
		token: tok,
		left:  left,
		right: right,
	}
	if _, exists := r.binary[key]; exists {
		return fmt.Errorf("binary rule %s(%s, %s) already registered", tok, left, right)
	}
	r.binary[key] = rule
	return nil
}

func (r *Registry) LookupBinary(tok token.TokenType, left, right TypeID) (BinaryRule, bool) {
	key := BinaryKey{
		token: tok,
		left:  left,
		right: right,
	}
	rule, ok := r.binary[key]
	return rule, ok
}

func (r *Registry) RegisterUnary(tok token.TokenType, right TypeID, rule UnaryRule) error {
	if err := r.checkOpen(); err != nil {
		return err
	}
	if err := rejectReserved(right, rule.EvalType); err != nil {
		return err
	}
	key := UnaryKey{
		token: tok,
		right: right,
	}
	if _, exists := r.unary[key]; exists {
		return fmt.Errorf("unary rule %s(%s) already registered", tok, right)
	}
	r.unary[key] = rule
	return nil
}

func (r *Registry) LookupUnary(tok token.TokenType, right TypeID) (UnaryRule, bool) {
	key := UnaryKey{
		token: tok,
		right: right,
	}
	rule, ok := r.unary[key]
	return rule, ok
}

func (r *Registry) RegisterMemberAccess(owner TypeID, member string, rule MemberAccessRule) error {
	if err := r.checkOpen(); err != nil {
		return err
	}
	if err := rejectReserved(owner, rule.EvalType); err != nil {
		return err
	}
	key := MemberAccessKey{
		owner:  owner,
		method: member,
	}
	if _, exists := r.memberAccess[key]; exists {
		return fmt.Errorf("member access rule %s.%s already registered", owner, member)
	}
	r.memberAccess[key] = rule
	return nil
}

func (r *Registry) LookupMemberAccess(owner TypeID, member string) (MemberAccessRule, bool) {
	key := MemberAccessKey{
		owner:  owner,
		method: member,
	}
	rule, ok := r.memberAccess[key]
	return rule, ok
}

func (r *Registry) RegisterCall(owner TypeID, member string, rule CallRule) error {
	if err := r.checkOpen(); err != nil {
		return err
	}
	if err := rejectReserved(owner, rule.EvalType); err != nil {
		return err
	}
	for _, arg := range rule.Args {
		if err := rejectReserved(arg.Type); err != nil {
			return err
		}
	}
	key := CallKey{
		owner:  owner,
		method: member,
	}
	if _, exists := r.call[key]; exists {
		return fmt.Errorf("call rule %s.%s already registered", owner, member)
	}
	r.call[key] = rule
	return nil
}

func (r *Registry) LookupCall(owner TypeID, member string) (CallRule, bool) {
	key := CallKey{
		owner:  owner,
		method: member,
	}
	rule, ok := r.call[key]
	return rule, ok
}

func (r *Registry) RegisterDeclKind(k DeclKind) error {
	if err := r.checkOpen(); err != nil {
		return err
	}
	if !token.IsIdent(k.Word) {
		return fmt.Errorf("declaration kind %q not registered. word is not a valid identifier", k.Word)
	}
	if token.IsKeyword(k.Word) {
		return fmt.Errorf("declaration kind %q not registered. word is a language keyword", k.Word)
	}
	if token.IsReservedIdent(k.Word) {
		return fmt.Errorf("declaration kind %q not registered. word is reserved", k.Word)
	}
	if _, exists := r.declKinds[k.Word]; exists {
		return fmt.Errorf("declaration kind %q not registered. word taken", k.Word)
	}
	if _, exists := r.LookupType(k.Word); exists {
		return fmt.Errorf("declaration kind %q not registered. word taken by a type", k.Word)
	}
	if _, exists := r.modules[k.Word]; exists {
		return fmt.Errorf("declaration kind %q not registered. word taken by a module", k.Word)
	}
	if err := rejectReserved(k.AllowedTypes...); err != nil {
		return err
	}
	for _, id := range k.AllowedTypes {
		if _, exists := r.types[id]; !exists {
			return fmt.Errorf("declaration kind %q not registered. allowed type %s is unknown", k.Word, id)
		}
	}
	r.declKinds[k.Word] = k
	return nil
}

func (r *Registry) LookupDeclKind(word string) (DeclKind, bool) {
	k, ok := r.declKinds[word]
	return k, ok
}

func (r *Registry) DeclKinds() []DeclKind {
	kinds := make([]DeclKind, 0, len(r.declKinds))
	for _, k := range r.declKinds {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i].Word < kinds[j].Word })
	return kinds
}

func (r *Registry) rejectDeclKindWord(name string) error {
	if _, taken := r.LookupDeclKind(name); taken {
		return fmt.Errorf("%s not registered. name taken by a declaration kind", name)
	}
	return nil
}
