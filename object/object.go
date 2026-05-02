package object

import "fmt"

type ObjectType string

const (
	IntKind     ObjectType = "int"
	FloatKind   ObjectType = "float"
	StringKind  ObjectType = "string"
	BooleanKind ObjectType = "boolean"
	NullKind    ObjectType = "null"
	SeriesKind  ObjectType = "series"
	ErrorKind   ObjectType = "error"
)

type Object interface {
	Type() ObjectType
	Inspect() string
}

type Integer struct {
	Value int64
}

func (i *Integer) Type() ObjectType { return IntKind }
func (i *Integer) Inspect() string  { return fmt.Sprintf("%d", i.Value) }

type Float struct {
	Value float64
}

func (f *Float) Type() ObjectType { return FloatKind }
func (f *Float) Inspect() string  { return fmt.Sprintf("%g", f.Value) }

type String struct {
	Value string
}

func (s *String) Type() ObjectType { return StringKind }
func (s *String) Inspect() string  { return s.Value }

type Boolean struct {
	Value bool
}

func (b *Boolean) Type() ObjectType { return BooleanKind }
func (b *Boolean) Inspect() string  { return fmt.Sprintf("%t", b.Value) }

type Null struct{}

func (n *Null) Type() ObjectType { return NullKind }
func (n *Null) Inspect() string  { return "null" }

type Series struct {
	Value []float64
}

func (s *Series) Type() ObjectType { return SeriesKind }
func (s *Series) Inspect() string  { return fmt.Sprintf("[%d]", len(s.Value)) }

type Error struct {
	Message string
}

func (e *Error) Type() ObjectType { return ErrorKind }
func (e *Error) Inspect() string  { return "ERROR: " + e.Message }

func IsError(obj Object) bool {
	return obj != nil && obj.Type() == ErrorKind
}
