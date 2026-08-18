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
	return registry.ErrorTypeID
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
	EvalFn func(left, right registry.Value) (registry.Value, error)
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

type LogicalExpr struct {
	Token token.Token
	Left  Expression
	Right Expression
}

func (le *LogicalExpr) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(le.Left.String())
	out.WriteString(" ")
	out.WriteString(le.Token.Literal)
	out.WriteString(" ")
	out.WriteString(le.Right.String())
	out.WriteString(")")
	return out.String()
}

func (le *LogicalExpr) expressionNode() {}

func (le *LogicalExpr) Type() registry.TypeID {
	return registry.BoolID
}

type PrefixExpr struct {
	Token  token.Token
	Right  Expression
	EvalFn func(right registry.Value) (registry.Value, error)
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
	Token     token.Token
	Object    Expression
	Attribute string
	T         registry.TypeID
	EvalFn    func(registry.Value) (registry.Value, error)
}

func (ma *MemberAccessExpr) String() string {
	var out bytes.Buffer
	out.WriteString(ma.Object.String())
	out.WriteString(ma.Token.Literal)
	out.WriteString(ma.Attribute)
	return out.String()
}

func (ma *MemberAccessExpr) expressionNode() {}

func (ma *MemberAccessExpr) Type() registry.TypeID {
	return ma.T
}

type SlotRefExpr struct {
	Token token.Token
	Slot  *SlotDecl
}

func (sr *SlotRefExpr) String() string {
	return sr.Slot.Kind + "." + sr.Slot.Name
}

func (sr *SlotRefExpr) expressionNode() {}

func (sr *SlotRefExpr) Type() registry.TypeID {
	return sr.Slot.T
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
	EvalFn   func(receiver registry.Value, args map[string]registry.Value) (registry.Value, error)
}

type CallArgExpr struct {
	Token token.Token
	Name  string
	Value Expression
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

func (ls *LetStmt) statementNode() {}

type AssignNameStmt struct {
	Token  token.Token // the `=`
	Target string
	Value  Expression
	T      registry.TypeID
}

func (as *AssignNameStmt) String() string {
	var out bytes.Buffer
	out.WriteString(as.Target)
	out.WriteString(" = ")
	if as.Value == nil {
		out.WriteString("<missing expression>")
	} else {
		out.WriteString(as.Value.String())
	}
	return out.String()
}

func (as *AssignNameStmt) statementNode() {}

type AssignSlotStmt struct {
	Token  token.Token
	Target *SlotDecl
	Value  Expression
	T      registry.TypeID
}

func (as *AssignSlotStmt) String() string {
	var out bytes.Buffer
	out.WriteString(as.Target.Kind)
	out.WriteString(".")
	out.WriteString(as.Target.Name)
	out.WriteString(" = ")
	if as.Value == nil {
		out.WriteString("<missing expression>")
	} else {
		out.WriteString(as.Value.String())
	}
	return out.String()
}

func (as *AssignSlotStmt) statementNode() {}

type EmitStmt struct {
	Token  token.Token
	Output string
	Args   []*CallArgExpr
	T      registry.TypeID
}

func (es *EmitStmt) String() string {
	var out bytes.Buffer
	out.WriteString("emit(")
	out.WriteString(es.Output)
	for _, arg := range es.Args {
		out.WriteString(", ")
		out.WriteString(arg.Name)
		out.WriteString("=")
		out.WriteString(arg.Value.String())
	}
	out.WriteString(")")
	return out.String()
}

func (es *EmitStmt) statementNode() {}

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

type BadStmt struct {
	Token token.Token
}

func (bs *BadStmt) String() string { return "<bad statement>" }

func (bs *BadStmt) statementNode() {}

type Program struct {
	// Decls is the *ordered* walk list and the only storage
	Decls  []Decl
	InitFn *FunctionDecl
	RunFn  *FunctionDecl
}

func (p *Program) Consts() []*ConstDecl {
	var consts []*ConstDecl
	for _, d := range p.Decls {
		if decl, ok := d.(*ConstDecl); ok {
			consts = append(consts, decl)
		}
	}
	return consts
}

func (p *Program) Slots() []*SlotDecl {
	var slots []*SlotDecl
	for _, d := range p.Decls {
		if decl, ok := d.(*SlotDecl); ok {
			slots = append(slots, decl)
		}
	}
	return slots
}

func (p *Program) Inputs() []*InputDecl {
	var inputs []*InputDecl
	for _, d := range p.Decls {
		if decl, ok := d.(*InputDecl); ok {
			inputs = append(inputs, decl)
		}
	}
	return inputs
}

func (p *Program) Outputs() []*OutputDecl {
	var outputs []*OutputDecl
	for _, d := range p.Decls {
		if decl, ok := d.(*OutputDecl); ok {
			outputs = append(outputs, decl)
		}
	}
	return outputs
}

type Decl interface {
	Node
	declNode()
}

type SlotDecl struct {
	Token token.Token
	Kind  string
	Name  string
	T     registry.TypeID
	Init  Expression
	Index int
}

func (sd *SlotDecl) String() string {
	var out bytes.Buffer
	out.WriteString(sd.Kind)
	out.WriteString(" ")
	out.WriteString(sd.Name)
	out.WriteString(": ")
	out.WriteString(sd.T.String())
	if sd.Init != nil {
		out.WriteString(" = ")
		out.WriteString(sd.Init.String())
	}
	return out.String()
}

func (sd *SlotDecl) declNode() {}

type InputDecl struct {
	Token token.Token
	Name  string
	T     registry.TypeID
}

func (id *InputDecl) String() string {
	return "input " + id.Name + ":" + id.T.String()
}

func (id *InputDecl) declNode() {}

type OutputDecl struct {
	Token token.Token
	Name  string
	T     registry.TypeID
}

func (od *OutputDecl) String() string {
	return "output " + od.Name + ":" + od.T.String()
}

func (od *OutputDecl) declNode() {}

type FunctionDecl struct {
	Token token.Token
	Body  *BlockStmt
}

type ConstDecl struct {
	Token token.Token
	Name  string
	Value Expression
	T     registry.TypeID
}

func (cd *ConstDecl) declNode() {}

func (cd *ConstDecl) String() string {
	var out bytes.Buffer
	out.WriteString("const ")
	out.WriteString(cd.Name)
	out.WriteString(" = ")
	if cd.Value == nil {
		out.WriteString("<missing expression>")
	} else {
		out.WriteString(cd.Value.String())
	}
	return out.String()
}
