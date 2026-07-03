package registry

type TypeID struct {
	id string
}

func (tid TypeID) String() string {
	return tid.id
}

var IntegerID = TypeID{id: "Integer"}
var FloatID = TypeID{id: "Float"}
var StringID = TypeID{id: "String"}
var BoolID = TypeID{id: "Bool"}
var UnknownTypeID = TypeID{id: "Unknown"} // as default value. Is it ok?

type TypeShape string

const (
	ScalarShape TypeShape = "SCALAR"
	VectorShape TypeShape = "VECTOR"
)

type TypeDef struct {
	Shape TypeShape
}

type NamedValue struct {
	Name  string
	Value Value
}

type Value interface {
	TypeID() TypeID
	//TypeShape() TypeShape
}

type Integer int

func (i Integer) TypeID() TypeID {
	return IntegerID
}

//func (i Integer) TypeShape() TypeShape {
//	return ScalarShape
//}

type Float float64

func (f Float) TypeID() TypeID {
	return FloatID
}

//func (f Float) TypeShape() TypeShape {
//	return ScalarShape
//}

type String string

func (s String) TypeID() TypeID {
	return StringID
}

//func (s String) TypeShape() TypeShape {
//	return ScalarShape
//}

type Bool bool

func (b Bool) TypeID() TypeID {
	return BoolID
}

//func (b Bool) TypeShape() TypeShape {
//	return ScalarShape
//}

type PlainModule struct {
	typeID TypeID
}

func (pm *PlainModule) TypeID() TypeID {
	return pm.typeID
}
