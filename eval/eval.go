// Package eval drives a compiled tascript program: it walks the AST,
// resolves identifiers, and produces Events. It does NOT manage data
// sources or sinks — those live in the host runtime.
package eval

import (
	"fmt"
	"math"

	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/token"
)

// Value is the union of runtime values supported by the implemented slices.
type Value any

// Candle is a single OHLCV bar fed by the host runtime.
type Candle struct {
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

// DataSource produces one candle per Runner.Step for an input port.
type DataSource interface {
	NextCandle() (Candle, error)
}

type CandleSeries struct {
	current Candle
	ready   bool
}

func (cs *CandleSeries) push(c Candle) {
	cs.current = c
	cs.ready = true
}

// Event is a single emit() call's product. Mirrors the public tascript.Event
// and the §2 event record: { output, ts, value, data }. Timestamp support is
// not exposed yet and Value is nil for structured outputs.
type Event struct {
	Output string
	Value  Value
	Data   map[string]Value
}

// Engine carries per-program execution state: top-level bindings and the
// outbound event buffer.
type Engine struct {
	prog       *ast.Program
	constBinds map[string]Value
	inputTypes map[string]string
	inputs     map[string]*CandleSeries
	sources    map[string]DataSource
	events     []Event
}

func New(prog *ast.Program, sources map[string]DataSource) *Engine {
	return &Engine{
		prog:       prog,
		constBinds: map[string]Value{},
		inputTypes: map[string]string{},
		inputs:     map[string]*CandleSeries{},
		sources:    sources,
	}
}

// Prepare evaluates the top-level declarations (constants, input ports).
// Called once before [Engine.RunInit].
func (e *Engine) Prepare() *diag.Diagnostic {
	for _, d := range e.prog.Decls {
		switch x := d.(type) {
		case *ast.ConstDecl:
			v, err := e.evalLiteral(x.Value)
			if err != nil {
				return err
			}
			e.constBinds[x.Name] = v
		case *ast.InputDecl:
			e.inputTypes[x.Name] = x.Type
			if x.Type == "CandleSeries" {
				e.inputs[x.Name] = &CandleSeries{}
			}
		}
	}
	return nil
}

// RunInit executes the program's Init() function exactly once.
func (e *Engine) RunInit() *diag.Diagnostic { return e.runFunc(e.prog.Init) }

// RunStep executes the program's Run() function once.
func (e *Engine) RunStep() *diag.Diagnostic {
	for name, src := range e.sources {
		series, ok := e.inputs[name]
		if !ok {
			continue
		}
		c, err := src.NextCandle()
		if err != nil {
			return &diag.Diagnostic{
				Phase: diag.PhaseRuntime, Category: diag.CatTypeMismatch,
				Msg: fmt.Sprintf("input %q failed to produce candle: %v", name, err),
			}
		}
		series.push(c)
	}
	return e.runFunc(e.prog.Run)
}

// DrainEvents returns and clears the emit() buffer.
func (e *Engine) DrainEvents() []Event {
	out := e.events
	e.events = nil
	return out
}

func (e *Engine) runFunc(fn *ast.FuncDecl) *diag.Diagnostic {
	if fn == nil {
		return nil
	}
	for _, s := range fn.Body {
		if err := e.runStmt(s); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) runStmt(s ast.Stmt) *diag.Diagnostic {
	switch x := s.(type) {
	case *ast.EmitStmt:
		return e.runEmit(x)
	case *ast.IfStmt:
		return e.runIf(x)
	default:
		return &diag.Diagnostic{
			Phase: diag.PhaseRuntime, Category: diag.CatNotImplemented,
			Pos: s.Pos(), Msg: fmt.Sprintf("statement %T not supported in this slice", s),
		}
	}
}

func (e *Engine) runEmit(em *ast.EmitStmt) *diag.Diagnostic {
	ev := Event{Output: em.Output, Data: map[string]Value{}}
	if em.Value != nil {
		v, err := e.evalExpr(em.Value)
		if err != nil {
			return err
		}
		ev.Value = v
	}
	for _, kw := range em.Kwargs {
		v, err := e.evalExpr(kw.Value)
		if err != nil {
			return err
		}
		ev.Data[kw.Name] = v
	}
	e.events = append(e.events, ev)
	return nil
}

func (e *Engine) runIf(stmt *ast.IfStmt) *diag.Diagnostic {
	cond, err := e.evalExpr(stmt.Condition)
	if err != nil {
		return err
	}
	b, ok := asBool(cond)
	if !ok {
		return typeErr(stmt.Condition.Pos(), "if condition must be Bool")
	}
	body := stmt.Alternative
	if b {
		body = stmt.Consequence
	}
	for _, s := range body {
		if err := e.runStmt(s); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) evalExpr(x ast.Expr) (Value, *diag.Diagnostic) {
	switch v := x.(type) {
	case *ast.NumberLit:
		return v.Val, nil
	case *ast.StringLit:
		return v.Val, nil
	case *ast.BoolLit:
		return v.Val, nil
	case *ast.Ident:
		if val, ok := e.constBinds[v.Name]; ok {
			return val, nil
		}
		if input, ok := e.inputs[v.Name]; ok {
			return input, nil
		}
		return nil, &diag.Diagnostic{
			Phase: diag.PhaseRuntime, Category: diag.CatTypeMismatch,
			Pos: v.P, Msg: fmt.Sprintf("undefined identifier %q", v.Name),
		}
	case *ast.UnaryExpr:
		return e.evalUnary(v)
	case *ast.BinaryExpr:
		return e.evalBinary(v)
	case *ast.MemberExpr:
		return e.evalMember(v)
	}
	return nil, &diag.Diagnostic{
		Phase: diag.PhaseRuntime, Category: diag.CatNotImplemented,
		Pos: x.Pos(), Msg: fmt.Sprintf("expression %T not supported in this slice", x),
	}
}

func (e *Engine) evalUnary(x *ast.UnaryExpr) (Value, *diag.Diagnostic) {
	right, err := e.evalExpr(x.Right)
	if err != nil {
		return nil, err
	}
	switch x.Op {
	case token.MINUS:
		n, ok := asNumber(right)
		if !ok {
			return nil, typeErr(x.OpPos, "unary '-' requires Number")
		}
		return -n, nil
	case token.BANG:
		b, ok := asBool(right)
		if !ok {
			return nil, typeErr(x.OpPos, "unary '!' requires Bool")
		}
		return !b, nil
	default:
		return nil, &diag.Diagnostic{
			Phase: diag.PhaseRuntime, Category: diag.CatNotImplemented,
			Pos: x.OpPos, Msg: fmt.Sprintf("unary operator %s not supported in this slice", x.Op),
		}
	}
}

func (e *Engine) evalBinary(x *ast.BinaryExpr) (Value, *diag.Diagnostic) {
	switch x.Op {
	case token.AND:
		return e.evalAnd(x)
	case token.OR:
		return e.evalOr(x)
	}

	left, err := e.evalExpr(x.Left)
	if err != nil {
		return nil, err
	}
	right, err := e.evalExpr(x.Right)
	if err != nil {
		return nil, err
	}
	switch x.Op {
	case token.EQ, token.NEQ:
		return evalEquality(x.Op, x.OpPos, left, right)
	case token.LT, token.LTE, token.GT, token.GTE:
		return evalCompare(x.Op, x.OpPos, left, right)
	}

	l, ok := asNumber(left)
	if !ok {
		return nil, typeErr(x.OpPos, "left operand must be Number")
	}
	r, ok := asNumber(right)
	if !ok {
		return nil, typeErr(x.OpPos, "right operand must be Number")
	}
	switch x.Op {
	case token.PLUS:
		return l + r, nil
	case token.MINUS:
		return l - r, nil
	case token.ASTERISK:
		return l * r, nil
	case token.SLASH:
		return l / r, nil
	case token.PERCENT:
		return math.Mod(l, r), nil
	default:
		return nil, &diag.Diagnostic{
			Phase: diag.PhaseRuntime, Category: diag.CatNotImplemented,
			Pos: x.OpPos, Msg: fmt.Sprintf("binary operator %s not supported in this slice", x.Op),
		}
	}
}

func (e *Engine) evalAnd(x *ast.BinaryExpr) (Value, *diag.Diagnostic) {
	left, err := e.evalExpr(x.Left)
	if err != nil {
		return nil, err
	}
	l, ok := asBool(left)
	if !ok {
		return nil, typeErr(x.OpPos, "left operand of && must be Bool")
	}
	if !l {
		return false, nil
	}
	right, err := e.evalExpr(x.Right)
	if err != nil {
		return nil, err
	}
	r, ok := asBool(right)
	if !ok {
		return nil, typeErr(x.OpPos, "right operand of && must be Bool")
	}
	return r, nil
}

func (e *Engine) evalOr(x *ast.BinaryExpr) (Value, *diag.Diagnostic) {
	left, err := e.evalExpr(x.Left)
	if err != nil {
		return nil, err
	}
	l, ok := asBool(left)
	if !ok {
		return nil, typeErr(x.OpPos, "left operand of || must be Bool")
	}
	if l {
		return true, nil
	}
	right, err := e.evalExpr(x.Right)
	if err != nil {
		return nil, err
	}
	r, ok := asBool(right)
	if !ok {
		return nil, typeErr(x.OpPos, "right operand of || must be Bool")
	}
	return r, nil
}

func (e *Engine) evalMember(x *ast.MemberExpr) (Value, *diag.Diagnostic) {
	obj, err := e.evalExpr(x.Object)
	if err != nil {
		return nil, err
	}
	series, ok := obj.(*CandleSeries)
	if !ok {
		return nil, typeErr(x.NamePos, fmt.Sprintf("member %q requires CandleSeries receiver", x.Name))
	}
	if !series.ready {
		return nil, &diag.Diagnostic{
			Phase: diag.PhaseRuntime, Category: diag.CatTypeMismatch,
			Pos: x.NamePos, Msg: "CandleSeries has no current candle; wire a DataSource before reading it",
		}
	}
	c := series.current
	switch x.Name {
	case "open", "opens":
		return c.Open, nil
	case "high", "highs":
		return c.High, nil
	case "low", "lows":
		return c.Low, nil
	case "close", "closes":
		return c.Close, nil
	case "volume", "volumes":
		return c.Volume, nil
	case "hl2":
		return (c.High + c.Low) / 2, nil
	case "hlc3":
		return (c.High + c.Low + c.Close) / 3, nil
	default:
		return nil, typeErr(x.NamePos, fmt.Sprintf("unknown CandleSeries member %q", x.Name))
	}
}

func asNumber(v Value) (float64, bool) {
	n, ok := v.(float64)
	return n, ok
}

func asBool(v Value) (bool, bool) {
	b, ok := v.(bool)
	return b, ok
}

func evalEquality(op token.Kind, pos token.Pos, left, right Value) (Value, *diag.Diagnostic) {
	switch l := left.(type) {
	case float64:
		r, ok := right.(float64)
		if !ok {
			return nil, typeErr(pos, "equality operands must have the same type")
		}
		return applyEquality(op, l == r), nil
	case string:
		r, ok := right.(string)
		if !ok {
			return nil, typeErr(pos, "equality operands must have the same type")
		}
		return applyEquality(op, l == r), nil
	case bool:
		r, ok := right.(bool)
		if !ok {
			return nil, typeErr(pos, "equality operands must have the same type")
		}
		return applyEquality(op, l == r), nil
	default:
		return nil, typeErr(pos, "equality operands must be scalar values")
	}
}

func evalCompare(op token.Kind, pos token.Pos, left, right Value) (Value, *diag.Diagnostic) {
	l, ok := asNumber(left)
	if !ok {
		return nil, typeErr(pos, "comparison operands must be Number")
	}
	r, ok := asNumber(right)
	if !ok {
		return nil, typeErr(pos, "comparison operands must be Number")
	}
	switch op {
	case token.LT:
		return l < r, nil
	case token.LTE:
		return l <= r, nil
	case token.GT:
		return l > r, nil
	case token.GTE:
		return l >= r, nil
	default:
		return nil, typeErr(pos, fmt.Sprintf("unsupported comparison operator %s", op))
	}
}

func applyEquality(op token.Kind, equal bool) bool {
	if op == token.NEQ {
		return !equal
	}
	return equal
}

func typeErr(pos token.Pos, msg string) *diag.Diagnostic {
	return &diag.Diagnostic{Phase: diag.PhaseRuntime, Category: diag.CatTypeMismatch, Pos: pos, Msg: msg}
}

func (e *Engine) evalLiteral(x ast.Expr) (Value, *diag.Diagnostic) {
	switch v := x.(type) {
	case *ast.NumberLit:
		return v.Val, nil
	case *ast.StringLit:
		return v.Val, nil
	}
	return nil, &diag.Diagnostic{
		Phase: diag.PhaseParse, Category: diag.CatTopLevelForm,
		Pos: x.Pos(), Msg: "top-level constant must be a literal in this slice",
	}
}
