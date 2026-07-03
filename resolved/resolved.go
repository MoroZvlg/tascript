package resolved

import (
	"bytes"
	"fmt"

	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/token"
)

type Node interface {
	String() string
}

type Expression interface {
	Node
	expressionNode()
	Type() registry.TypeID
}

type Statement interface {
	Node
	statementNode()
}

type BadExpr struct {
	Token token.Token
}

func (be *BadExpr) String() string {
	return "<error>"
}

func (be *BadExpr) expressionNode() {}

func (be *BadExpr) Type() registry.TypeID {
	return registry.UnknownTypeID
}

type IdentExpr struct {
	Token token.Token
	T     registry.TypeID
}

func (ie *IdentExpr) String() string {
	return ie.Token.Literal
}

func (ie *IdentExpr) expressionNode() {}

func (ie *IdentExpr) Type() registry.TypeID {
	return ie.T
}

type IntegerExpr struct {
	Token token.Token
	Value int
	T     registry.TypeID
}

func (ie *IntegerExpr) String() string {
	return fmt.Sprintf("%d", ie.Value)
}

func (ie *IntegerExpr) expressionNode() {}

func (ie *IntegerExpr) Type() registry.TypeID {
	return ie.T
}

type FloatExpr struct {
	Token token.Token
	Value float64
	T     registry.TypeID
}

func (fe *FloatExpr) String() string {
	return fmt.Sprintf("%g", fe.Value)
}

func (fe *FloatExpr) expressionNode() {}

func (fe *FloatExpr) Type() registry.TypeID {
	return fe.T
}

type StringExpr struct {
	Token token.Token
	Value string
	T     registry.TypeID
}

func (se *StringExpr) String() string {
	// %q re-escapes the decoded value so the rendered source round-trips
	return fmt.Sprintf("%q", se.Value)
}

func (se *StringExpr) expressionNode() {}

func (se *StringExpr) Type() registry.TypeID {
	return se.T
}

type BooleanExpr struct {
	Token token.Token
	Value bool
	T     registry.TypeID
}

func (be *BooleanExpr) String() string {
	return fmt.Sprintf("%t", be.Value)
}

func (be *BooleanExpr) expressionNode() {}

func (be *BooleanExpr) Type() registry.TypeID {
	return be.T
}

type InfixExpr struct {
	Token  token.Token
	Left   Expression
	Right  Expression
	T      registry.TypeID
	EvalFn func(left, right registry.Value) registry.Value
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

func (ie *InfixExpr) Type() registry.TypeID {
	return ie.T
}

type PrefixExpr struct {
	Token  token.Token
	Right  Expression
	EvalFn func(right registry.Value) registry.Value
	T      registry.TypeID
}

func (pe *PrefixExpr) String() string {
	var out bytes.Buffer
	out.WriteString(pe.Token.Literal)
	out.WriteString(pe.Right.String())
	return out.String()
}

func (pe *PrefixExpr) expressionNode() {}

func (pe *PrefixExpr) Type() registry.TypeID {
	return pe.T
}

type MemberAccessExpr struct {
	Token  token.Token
	Object Expression
	// TODO: i rewrote it to string, is it looks correct? method do not returns any type/evalFn/etc.
	// It's just a name of the attribute. TypeID showed by MemberAccess itself
	Method string
	T      registry.TypeID
	EvalFn func(registry.Value) registry.Value
}

func (ma *MemberAccessExpr) String() string {
	var out bytes.Buffer
	out.WriteString(ma.Object.String())
	out.WriteString(ma.Token.Literal)
	out.WriteString(ma.Method)
	return out.String()
}

func (ma *MemberAccessExpr) expressionNode() {}

func (ma *MemberAccessExpr) Type() registry.TypeID {
	return ma.T
}

type IndexExpr struct {
	Token token.Token
	Left  Expression
	Index Expression
	T     registry.TypeID
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

func (ie *IndexExpr) Type() registry.TypeID {
	return ie.T
}

type MethodCallExpr struct {
	Token    token.Token
	Receiver Expression
	Method   string
	Args     []*CallArgExpr
	T        registry.TypeID
	EvalFn   func(receiver registry.Value, args map[string]registry.Value) registry.Value
}

type CallArgExpr struct {
	Token token.Token
	Name  string
	Value Expression
	T     registry.TypeID
}

func (ce *MethodCallExpr) String() string {
	var out bytes.Buffer
	out.WriteString(ce.Receiver.String())
	out.WriteString(".")
	out.WriteString(ce.Method)
	out.WriteString("(")
	for i, arg := range ce.Args {
		out.WriteString(arg.Name)
		out.WriteString("=")
		out.WriteString(arg.Value.String())
		if i < len(ce.Args)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString(")")
	return out.String()
}

func (ce *MethodCallExpr) expressionNode() {}

func (ce *MethodCallExpr) Type() registry.TypeID {
	return ce.T
}

// KwargsExpr is a call-argument helper, not a standalone expression: it only
// ever appears inside CallExpr.Kwargs, so it does not implement Expression.
type KwargsExpr struct {
	Token token.Token
	Key   token.Token
	Value Expression
}

func (ke *KwargsExpr) String() string {
	var out bytes.Buffer
	out.WriteString(ke.Key.Literal)
	out.WriteString(" = ")
	if ke.Value == nil {
		out.WriteString("<missing expression>")
	} else {
		out.WriteString(ke.Value.String())
	}
	return out.String()
}

// CoerceExpr is synthesized by the resolver (it has no source token of its own)
// to mark an implicit conversion of Inner to type T, e.g. Integer -> Float. The
// evaluator applies the registered coercion when it reaches this node.
type CoerceExpr struct {
	Inner  Expression
	T      registry.TypeID
	EvalFn func(registry.Value) registry.Value
}

func (ce *CoerceExpr) String() string {
	// Transparent in source rendering — the conversion is implicit.
	return ce.Inner.String()
}

func (ce *CoerceExpr) expressionNode() {}

func (ce *CoerceExpr) Type() registry.TypeID {
	return ce.T
}

type LetStmt struct {
	Token token.Token
	Name  string
	Value Expression
	T     registry.TypeID
}

func (ls *LetStmt) String() string {
	var out bytes.Buffer
	out.WriteString("let ")
	out.WriteString(ls.Name)
	out.WriteString(" = ")
	if ls.Value == nil {
		out.WriteString("<missing expression>")
	} else {
		out.WriteString(ls.Value.String())
	}
	return out.String()
}

func (ls *LetStmt) Type() registry.TypeID {
	return ls.T
}

func (ls *LetStmt) statementNode() {}

type AssignStmt struct {
	Token  token.Token // the `=`
	Target Expression
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

// BadStmt mirrors ast.BadStmt: a placeholder kept when a statement is too broken
// to resolve. It only exists so resolution can keep collecting errors; the
// evaluator never walks a tree that contains one (resolution gates evaluation).
type BadStmt struct {
	From token.Pos
	To   token.Pos
}

func (bs *BadStmt) String() string { return "<bad statement>" }

func (bs *BadStmt) statementNode() {}

// Program is the resolver's output and the evaluator's input — the resolved
// counterpart of ast.Program. Input/Output/Type declarations are resolve-time
// only and are not carried here until the evaluator needs them at runtime.
type Program struct {
	Consts []*ConstDecl
	InitFn *Function
	RunFn  *Function
}

type Function struct {
	Token token.Token
	Body  *BlockStmt
}

type ConstDecl struct {
	Token token.Token
	Name  *IdentExpr
	Value Expression
	T     registry.TypeID
}

func (cd *ConstDecl) Type() registry.TypeID {
	return cd.T
}

func (cd *ConstDecl) String() string {
	var out bytes.Buffer
	out.WriteString("const ")
	if cd.Name == nil {
		out.WriteString("<unknown>")
	} else {
		out.WriteString(cd.Name.String())
	}
	out.WriteString(" = ")
	if cd.Value == nil {
		out.WriteString("<missing expression>")
	} else {
		out.WriteString(cd.Value.String())
	}
	return out.String()
}
