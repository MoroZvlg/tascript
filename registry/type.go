package registry

type TypeID struct {
	id string
}

func NewTypeID(name string) TypeID {
	return TypeID{id: name}
}

func (tid TypeID) String() string {
	return tid.id
}

var IntegerID = NewTypeID("Integer")
var FloatID = NewTypeID("Float")
var StringID = NewTypeID("String")
var BoolID = NewTypeID("Bool")
var ErrorTypeID = NewTypeID("Error")

type TypeShape string

const (
	ScalarShape TypeShape = "SCALAR"
	VectorShape TypeShape = "VECTOR"
	ModuleShape TypeShape = "MODULE"
)

type TypeDef struct {
	Shape  TypeShape
	Fields []FieldDef // set for structural (script-declared) types, empty otherwise
}

type FieldDef struct {
	Name string
	Type TypeID
}

type NamedValue struct {
	Name  string
	Value Value
}

type Value interface {
	TypeID() TypeID
}

type Integer int

func (i Integer) TypeID() TypeID {
	return IntegerID
}

type Float float64

func (f Float) TypeID() TypeID {
	return FloatID
}

type String string

func (s String) TypeID() TypeID {
	return StringID
}

type Bool bool

func (b Bool) TypeID() TypeID {
	return BoolID
}

type PlainModule struct {
	typeID TypeID
}

func (pm *PlainModule) TypeID() TypeID {
	return pm.typeID
}

// Record is the runtime representation of a structural type (inline `{field: Type}` declared on an input/output).
// TODO: double think how it will be wired with host... Not clear right now
type Record struct {
	T      TypeID
	Fields map[string]Value
}

func (r Record) TypeID() TypeID {
	return r.T
}
