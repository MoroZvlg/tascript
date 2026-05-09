package object

import (
	"bytes"
	"fmt"

	"github.com/MoroZvlg/tascript/ast"
)

type ObjectType string

const (
	IntKind          ObjectType = "int"
	FloatKind        ObjectType = "float"
	StringKind       ObjectType = "string"
	BooleanKind      ObjectType = "boolean"
	NullKind         ObjectType = "null"
	FunctionKind     ObjectType = "function"
	ReturnKind       ObjectType = "return"
	SeriesKind       ObjectType = "series"
	CandleKind       ObjectType = "candle"
	CandleSeriesKind ObjectType = "CandleSeries"
	BuiltinKind      ObjectType = "builtin"
	ErrorKind        ObjectType = "error"
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

type Return struct {
	Value Object
}

func (r *Return) Type() ObjectType { return ReturnKind }
func (r *Return) Inspect() string  { return r.Value.Inspect() }

type Function struct {
	Parameters []*ast.Identifier
	Body       *ast.BlockStatement
	Env        *Environment
}

func (f *Function) Type() ObjectType { return FunctionKind }
func (f *Function) Inspect() string {
	var out bytes.Buffer
	out.WriteString("function(")
	for i, s := range f.Parameters {
		out.WriteString(s.String())
		if i < len(f.Parameters)-1 {
			out.WriteString(", ")
		} else {
			out.WriteString(") ")
		}
	}
	out.WriteString("{ ... }")
	return out.String()
}

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

type Candle struct {
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

func (c *Candle) Type() ObjectType { return CandleKind }
func (c *Candle) Inspect() string {
	return fmt.Sprintf("o=%g,h=%g,l=%g,c=%g,v=%g", c.Open, c.High, c.Low, c.Close, c.Volume)
}

type CandleSeries struct {
	Value []Candle
}

func (c *CandleSeries) Type() ObjectType { return CandleSeriesKind }
func (c *CandleSeries) Inspect() string  { return fmt.Sprintf("Candles[%d]", len(c.Value)) }

type Builtin struct {
	Name string
	Fn   func(env *Environment, args []Object) Object
}

func (b *Builtin) Type() ObjectType { return BuiltinKind }
func (b *Builtin) Inspect() string  { return b.Name }
