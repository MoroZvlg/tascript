package ast

import (
	"bytes"
	"fmt"

	"github.com/MoroZvlg/tascript/token"
)

type Program struct {
	Consts      []*ConstDecl
	Inputs      []*InputDecl
	Outputs     []*OutputDecl
	StateFields []*StateFieldDecl
	InitFn      *FunctionDecl
	RunFn       *FunctionDecl
	Valid       bool
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

type TypeDecl interface {
	Node
	typeDeclNode()
}

// InputDecl
// - input btc: CandleSeries
// - input btc: { field: Type, ...}
type InputDecl struct {
	Token      token.Token
	Identifier *IdentExpr
	Type       TypeDecl
}

func (id *InputDecl) String() string {
	var out bytes.Buffer
	out.WriteString(id.Token.Literal)
	out.WriteString(" ")
	if id.Identifier == nil {
		out.WriteString("<unknown>")
	} else {
		out.WriteString(id.Identifier.String())
	}
	out.WriteString(": ")

	if id.Type == nil {
		out.WriteString("<missing type>")
	} else {
		out.WriteString(id.Type.String())
	}
	return out.String()
}

// OutputDecl
// - output alert: String
// - output alert: { field: Type }
type OutputDecl struct {
	Token      token.Token
	Identifier *IdentExpr
	Type       TypeDecl
}

func (od *OutputDecl) String() string {
	var out bytes.Buffer
	out.WriteString(od.Token.Literal)
	out.WriteString(" ")
	if od.Identifier == nil {
		out.WriteString("<unknown>")
	} else {
		out.WriteString(od.Identifier.String())
	}
	out.WriteString(": ")

	if od.Type == nil {
		out.WriteString("<missing type>")
	} else {
		out.WriteString(od.Type.String())
	}
	return out.String()
}

// ConstDecl - const FOO = "BAR" + "ZOO"
type ConstDecl struct {
	Token      token.Token
	Identifier *IdentExpr
	Value      Expression
}

func (cd *ConstDecl) String() string {
	var out bytes.Buffer
	out.WriteString("const ")
	if cd.Identifier == nil {
		out.WriteString("<unknown>")
	} else {
		out.WriteString(cd.Identifier.String())
	}
	out.WriteString(" = ")
	if cd.Value == nil {
		out.WriteString("<missing expression>")
	} else {
		out.WriteString(cd.Value.String())
	}
	return out.String()
}

type StateFieldDecl struct {
	Token      token.Token
	Identifier *IdentExpr
	Type       TypeDecl
	Value      Expression
}

func (sfd *StateFieldDecl) String() string {
	var out bytes.Buffer
	out.WriteString(sfd.Token.Literal)
	out.WriteString(" ")
	if sfd.Identifier == nil {
		out.WriteString("<unknown>")
	} else {
		out.WriteString(sfd.Identifier.String())
	}
	out.WriteString(": ")
	if sfd.Type == nil {
		out.WriteString("<missing type>")
	} else {
		out.WriteString(sfd.Type.String())
	}
	if sfd.Value != nil {
		out.WriteString(" = ")
		out.WriteString(sfd.Value.String())
	}
	return out.String()
}

// FunctionDecl
// - function Init() {}
type FunctionDecl struct {
	Token      token.Token
	Identifier *IdentExpr
	Body       *BlockStmt
}

func (fd *FunctionDecl) String() string {
	var out bytes.Buffer
	out.WriteString("function ")
	if fd.Identifier == nil {
		out.WriteString("<unknown>")
	} else {
		out.WriteString(fd.Identifier.String())
	}
	out.WriteString("(")
	out.WriteString(") ")
	if fd.Body != nil {
		out.WriteString(fd.Body.String())
	}
	return out.String()
}

type BadExpr struct {
	Token token.Token
}

func (be *BadExpr) String() string {
	return "<error>"
}

func (be *BadExpr) expressionNode() {}

type IdentExpr struct {
	Token token.Token
}

func (ie *IdentExpr) String() string {
	return ie.Token.Literal
}

func (ie *IdentExpr) expressionNode() {}

func (ie *IdentExpr) typeDeclNode() {}

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
	// %q re-escapes the decoded value so the rendered source round-trips
	return fmt.Sprintf("%q", se.Value)
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

func (pe *PrefixExpr) String() string {
	var out bytes.Buffer
	out.WriteString(pe.Token.Literal)
	out.WriteString(pe.Right.String())
	return out.String()
}

func (pe *PrefixExpr) expressionNode() {}

type MemberAccessExpr struct {
	Token  token.Token
	Object Expression
	Method *IdentExpr
}

func (ma *MemberAccessExpr) String() string {
	var out bytes.Buffer
	out.WriteString(ma.Object.String())
	out.WriteString(ma.Token.Literal)
	out.WriteString(ma.Method.String())
	return out.String()
}

func (ma *MemberAccessExpr) expressionNode() {}

type IndexExpr struct {
	Token token.Token
	Left  Expression
	Index Expression
}

func (ie *IndexExpr) String() string {
	var out bytes.Buffer
	out.WriteString(ie.Left.String())
	out.WriteString("[")
	out.WriteString(ie.Index.String())
	out.WriteString("]")
	return out.String()
}

func (ie *IndexExpr) expressionNode() {}

type CallExpr struct {
	Token  token.Token
	Callee Expression
	Args   []Expression
	Kwargs []*KwargsExpr
}

func (ce *CallExpr) String() string {
	var out bytes.Buffer
	out.WriteString(ce.Callee.String())
	out.WriteString("(")
	for i, arg := range ce.Args {
		out.WriteString(arg.String())
		if i < len(ce.Args)-1 || len(ce.Kwargs) > 0 {
			out.WriteString(", ")
		}
	}
	for i, kwarg := range ce.Kwargs {
		out.WriteString(kwarg.String())
		if i < len(ce.Kwargs)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString(")")
	return out.String()
}

func (ce *CallExpr) expressionNode() {}

type KwargsExpr struct {
	Token token.Token
	Key   *IdentExpr
	Value Expression
}

func (ke *KwargsExpr) String() string {
	var out bytes.Buffer
	out.WriteString(ke.Key.String())
	out.WriteString(" = ")
	if ke.Value == nil {
		out.WriteString("<missing expression>")
	} else {
		out.WriteString(ke.Value.String())
	}
	return out.String()
}

func (ke *KwargsExpr) expressionNode() {}

type TypeExpr struct {
	Token  token.Token
	Fields []*FieldExpr
}

func (te *TypeExpr) String() string {
	var out bytes.Buffer
	out.WriteString("{")
	for i, field := range te.Fields {
		out.WriteString(field.String())
		if i < len(te.Fields)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString("}")
	return out.String()
}

func (te *TypeExpr) typeDeclNode() {}

type FieldExpr struct {
	Token token.Token
	Name  *IdentExpr
	Type  *IdentExpr // TypeDecl to allow nested types. Don't want to support it right now
}

func (fe *FieldExpr) String() string {
	var out bytes.Buffer
	out.WriteString(fe.Name.String())
	out.WriteString(": ")
	out.WriteString(fe.Type.String())
	return out.String()
}

type LetStmt struct {
	Token token.Token
	Name  *IdentExpr
	Value Expression
}

func (ls *LetStmt) String() string {
	var out bytes.Buffer
	out.WriteString("let ")
	if ls.Name == nil {
		out.WriteString("<unknown>")
	} else {
		out.WriteString(ls.Name.String())
	}
	out.WriteString(" = ")
	if ls.Value == nil {
		out.WriteString("<missing expression>")
	} else {
		out.WriteString(ls.Value.String())
	}
	return out.String()
}

func (ls *LetStmt) statementNode() {}

type AssignStmt struct {
	Token  token.Token // the `=`
	Target Expression  // TODO: should be ident?
	Value  Expression
}

func (as *AssignStmt) String() string {
	var out bytes.Buffer
	if as.Target == nil {
		out.WriteString("<unknown>")
	} else {
		out.WriteString(as.Target.String())
	}
	out.WriteString(" = ")
	if as.Value == nil {
		out.WriteString("<missing expression>")
	} else {
		out.WriteString(as.Value.String())
	}
	return out.String()
}

func (as *AssignStmt) statementNode() {}

// BadStmt is a placeholder kept in the tree when a statement is too broken to
// form a meaningful partial node. It preserves the source span so later passes
// (tooling, analyzer) keep working over recovered code. See [IsBadExpr] for the
// expression-level analogue.
type BadStmt struct {
	From token.Pos
	To   token.Pos
}

func (bs *BadStmt) String() string { return "<bad statement>" }

func (bs *BadStmt) statementNode() {}

type ExprStmt struct {
	Token token.Token
	Expr  Expression
}

func (es *ExprStmt) String() string {
	if es.Expr != nil {
		return es.Expr.String()
	}
	return "<missing expression>"
}

func (es *ExprStmt) statementNode() {}

type BlockStmt struct {
	Token token.Token
	Stmts []Statement
}

func (bs *BlockStmt) String() string {
	var out bytes.Buffer
	out.WriteString("{")
	if len(bs.Stmts) > 0 {
		out.WriteString("\n")
	}
	for i, stmt := range bs.Stmts {
		out.WriteString(stmt.String())
		if i < len(bs.Stmts)-1 {
			out.WriteString("\n")
		}
	}
	if len(bs.Stmts) > 0 {
		out.WriteString("\n")
	}
	out.WriteString("}")
	return out.String()
}

func (bs *BlockStmt) statementNode() {}

type IfStmt struct {
	Token       token.Token
	Condition   Expression
	Consequence *BlockStmt
	Else        Statement
}

func (is *IfStmt) String() string {
	var out bytes.Buffer
	out.WriteString("if (")
	if is.Condition == nil {
		out.WriteString("<missing condition>")
	} else {
		out.WriteString(is.Condition.String())
	}
	out.WriteString(") ")
	if is.Consequence == nil {
		out.WriteString("{}")
	} else {
		out.WriteString(is.Consequence.String())
	}
	if is.Else != nil {
		out.WriteString(" else ")
		out.WriteString(is.Else.String())
	}
	return out.String()
}

func (is *IfStmt) statementNode() {}

func IsBadExpr(expression Expression) bool {
	_, ok := expression.(*BadExpr)
	return ok
}
