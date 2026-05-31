package ast

import (
	"bytes"
	"fmt"

	"github.com/MoroZvlg/tascript/token"
)

type Program struct {
	Consts  []*ConstDecl
	Inputs  []*InputDecl
	Outputs []*OutputDecl
	InitFn  *FunctionDecl
	RunFn   *FunctionDecl
	Valid   bool
}

type Node interface {
	String() string
}

type Statement interface {
	Node
	statementNode()
}

type Expression interface {
	Node
	expressionNode()
}

type Declaration interface {
	Node
	declarationNode()
}

// InputDecl
// - input btc: CandleSeries
// - input btc: { field: Type, ...}
type InputDecl struct {
	Token      token.Token
	Identifier IdentExpr
	Type       IdentExpr
}

func (id *InputDecl) String() string {
	var out bytes.Buffer
	out.WriteString(id.Token.Literal)
	out.WriteString(" ")
	out.WriteString(id.Identifier.String())
	out.WriteString(": ")
	out.WriteString(id.Type.String())
	return out.String()
}

func (id *InputDecl) declarationNode() {}

// OutputDecl
// - output alert: String
// - output alert: { field: Type }
type OutputDecl struct {
	Token      token.Token
	Identifier IdentExpr
	Type       IdentExpr
}

func (od *OutputDecl) String() string {
	var out bytes.Buffer
	out.WriteString(od.Token.Literal)
	out.WriteString(" ")
	out.WriteString(od.Identifier.String())
	out.WriteString(": ")
	out.WriteString(od.Type.String())
	return out.String()
}

func (od *OutputDecl) declarationNode() {}

// ConstDecl - const FOO = "BAR" + "ZOO"
type ConstDecl struct {
	Token      token.Token
	Identifier *IdentExpr
	Value      Expression
}

func (cd *ConstDecl) String() string {
	var out bytes.Buffer
	out.WriteString("const ")
	out.WriteString(cd.Identifier.String())
	out.WriteString(" = ")
	out.WriteString(cd.Value.String())
	return out.String()
}

func (cd *ConstDecl) declarationNode() {}

// FunctionDecl
// - function Init() {}
type FunctionDecl struct {
	Token      token.Token
	Identifier IdentExpr
	// we don't have params for now
	//Parameters []*Identifier
	Body []Statement
}

func (fd *FunctionDecl) String() string {
	var out bytes.Buffer
	out.WriteString("function ")
	out.WriteString(fd.Identifier.String())
	out.WriteString("(")
	//for i, param := range fd.Parameters {
	//	out.WriteString(param.String())
	//	if i < len(fd.Parameters)-1 {
	//		out.WriteString(", ")
	//	}
	//}
	out.WriteString(") ")
	for _, stmt := range fd.Body {
		out.WriteString(stmt.String())
	}
	return out.String()
}

func (fd *FunctionDecl) declarationNode() {}

type IdentExpr struct {
	Token token.Token
}

func (ie *IdentExpr) String() string {
	return ie.Token.Literal
}

func (ie *IdentExpr) expressionNode() {}

type IntegerExpr struct {
	Token token.Token
	Value int
}

func (ie *IntegerExpr) String() string {
	return fmt.Sprintf("%d", ie.Value)
}

func (ie *IntegerExpr) expressionNode() {}

type FloatExpr struct {
	Token token.Token
	Value float64
}

func (fe *FloatExpr) String() string {
	return fmt.Sprintf("%g", fe.Value)
}

func (fe *FloatExpr) expressionNode() {}

type StringExpr struct {
	Token token.Token
	Value string
}

func (se *StringExpr) String() string {
	return fmt.Sprintf("\"%s\"", se.Value)
}

func (se *StringExpr) expressionNode() {}

type BooleanExpr struct {
	Token token.Token
	Value bool
}

func (be *BooleanExpr) String() string {
	return fmt.Sprintf("%t", be.Value)
}

func (be *BooleanExpr) expressionNode() {}

type InfixExpr struct {
	Token token.Token
	Left  Expression
	Right Expression
}

func (ie *InfixExpr) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(ie.Left.String())
	out.WriteString(" ")
	out.WriteString(ie.Token.Literal)
	out.WriteString(" ")
	out.WriteString(ie.Right.String())
	out.WriteString(")")
	return out.String()
}

func (ie *InfixExpr) expressionNode() {}

type PrefixExpr struct {
	Token token.Token
	Right Expression
}

func (re *PrefixExpr) String() string {
	var out bytes.Buffer
	out.WriteString(re.Token.Literal)
	out.WriteString(re.Right.String())
	return out.String()
}

func (re *PrefixExpr) expressionNode() {}

type MemberAccessExpr struct {
	Token  token.Token
	Object Expression
	Method Expression
}

func (ma *MemberAccessExpr) String() string {
	var out bytes.Buffer
	out.WriteString(ma.Object.String())
	out.WriteString(ma.Token.Literal)
	out.WriteString(ma.Method.String())
	return out.String()
}

func (ma *MemberAccessExpr) expressionNode() {}
