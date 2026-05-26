// Package ast defines the tascript abstract syntax tree.
package ast

import "github.com/MoroZvlg/tascript/token"

type Node interface {
	Pos() token.Pos
}

type Program struct {
	Decls []TopDecl
	Init  *FuncDecl
	Run   *FuncDecl
}

type TopDecl interface {
	Node
	topDecl()
}

type ConstDecl struct {
	Name    string
	NamePos token.Pos
	Value   Expr
}

func (c *ConstDecl) Pos() token.Pos { return c.NamePos }
func (*ConstDecl) topDecl()         {}

// InputDecl is `input <name>: <Type>`. The name becomes a read-only
// top-level binding; Type is the declared port type (e.g. "CandleSeries").
type InputDecl struct {
	Name    string
	NamePos token.Pos
	Type    string
	TypePos token.Pos
}

func (i *InputDecl) Pos() token.Pos { return i.NamePos }
func (*InputDecl) topDecl()         {}

// OutputDecl is one of the three §3.3 shapes:
//
//	output <name>: <ValueType>
//	output <name> { field: Type, ... }
//	output <name>: <ValueType> { field: Type, ... }
//
// ValueType is "" when no `: Type` is present. Structured is true when a
// `{ ... }` schema block is present (Fields may still be empty for an
// empty schema). A value-only output has Structured == false.
type OutputDecl struct {
	Name         string
	NamePos      token.Pos
	ValueType    string
	ValueTypePos token.Pos
	Structured   bool
	Fields       []SchemaField
}

func (o *OutputDecl) Pos() token.Pos { return o.NamePos }
func (*OutputDecl) topDecl()         {}

type SchemaField struct {
	Name    string
	NamePos token.Pos
	Type    string
	TypePos token.Pos
}

type FuncDecl struct {
	Name    string
	NamePos token.Pos
	Params  []Param
	Body    []Stmt
}

func (f *FuncDecl) Pos() token.Pos { return f.NamePos }
func (*FuncDecl) topDecl()         {}

type Param struct {
	Name    string
	NamePos token.Pos
}

type Stmt interface {
	Node
	stmt()
}

// AssignStmt is statement-level assignment: `target = value`. Assignment is
// not an expression in tascript.
type AssignStmt struct {
	Target Expr
	Value  Expr
}

func (a *AssignStmt) Pos() token.Pos { return a.Target.Pos() }
func (*AssignStmt) stmt()            {}

// EmitStmt is `emit(Output [, Value] [, ident=expr]*)`. Output is a
// declared output identifier (not a string literal). Value is the leading
// positional value for value-outputs, nil otherwise.
type EmitStmt struct {
	CallPos   token.Pos
	Output    string
	OutputPos token.Pos
	Value     Expr
	Kwargs    []KwArg
}

func (e *EmitStmt) Pos() token.Pos { return e.CallPos }
func (*EmitStmt) stmt()            {}

type IfStmt struct {
	IfPos       token.Pos
	Condition   Expr
	Consequence []Stmt
	Alternative []Stmt
}

func (i *IfStmt) Pos() token.Pos { return i.IfPos }
func (*IfStmt) stmt()            {}

type ExprStmt struct {
	Expr Expr
}

func (e *ExprStmt) Pos() token.Pos { return e.Expr.Pos() }
func (*ExprStmt) stmt()            {}

type KwArg struct {
	Name    string
	NamePos token.Pos
	Value   Expr
}

type Expr interface {
	Node
	expr()
}

type NumberLit struct {
	Val float64
	P   token.Pos
}

func (n *NumberLit) Pos() token.Pos { return n.P }
func (*NumberLit) expr()            {}

type StringLit struct {
	Val string
	P   token.Pos
}

func (s *StringLit) Pos() token.Pos { return s.P }
func (*StringLit) expr()            {}

type Ident struct {
	Name string
	P    token.Pos
}

func (i *Ident) Pos() token.Pos { return i.P }
func (*Ident) expr()            {}

type BoolLit struct {
	Val bool
	P   token.Pos
}

func (b *BoolLit) Pos() token.Pos { return b.P }
func (*BoolLit) expr()            {}

type UnaryExpr struct {
	Op    token.Kind
	OpPos token.Pos
	Right Expr
}

func (u *UnaryExpr) Pos() token.Pos { return u.OpPos }
func (*UnaryExpr) expr()            {}

type BinaryExpr struct {
	Left  Expr
	Op    token.Kind
	OpPos token.Pos
	Right Expr
}

func (b *BinaryExpr) Pos() token.Pos { return b.Left.Pos() }
func (*BinaryExpr) expr()            {}

type MemberExpr struct {
	Object  Expr
	Name    string
	NamePos token.Pos
}

func (m *MemberExpr) Pos() token.Pos { return m.Object.Pos() }
func (*MemberExpr) expr()            {}

type IndexExpr struct {
	Object Expr
	Index  Expr
	LPos   token.Pos
}

func (i *IndexExpr) Pos() token.Pos { return i.Object.Pos() }
func (*IndexExpr) expr()            {}

type CallExpr struct {
	Callee Expr
	LPos   token.Pos
	Args   []CallArg
}

func (c *CallExpr) Pos() token.Pos { return c.Callee.Pos() }
func (*CallExpr) expr()            {}

type CallArg struct {
	Name    string
	NamePos token.Pos
	Value   Expr
}
