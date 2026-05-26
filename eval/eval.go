// Package eval drives a compiled tascript program: it walks the AST,
// resolves identifiers, and produces Events. It does NOT manage data
// sources or sinks — those live in the host runtime.
package eval

import (
	"fmt"

	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/diag"
)

// Value is the union of slice-0 value types: Number (float64), String (string).
type Value any

// Event is a single emit() call's product. Mirrors the public tascript.Event
// and the §2 event record: { output, ts, value, data }. In this slice ts is
// unset (no candle clock yet) and Value is nil for structured outputs.
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
	inputs     map[string]string // port name -> declared type (slice 0: placeholder, no data)
	events     []Event
}

func New(prog *ast.Program) *Engine {
	return &Engine{prog: prog, constBinds: map[string]Value{}, inputs: map[string]string{}}
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
			e.inputs[x.Name] = x.Type
		}
	}
	return nil
}

// RunInit executes the program's Init() function exactly once.
func (e *Engine) RunInit() *diag.Diagnostic { return e.runFunc(e.prog.Init) }

// RunStep executes the program's Run() function once.
func (e *Engine) RunStep() *diag.Diagnostic { return e.runFunc(e.prog.Run) }

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

func (e *Engine) evalExpr(x ast.Expr) (Value, *diag.Diagnostic) {
	switch v := x.(type) {
	case *ast.NumberLit:
		return v.Val, nil
	case *ast.StringLit:
		return v.Val, nil
	case *ast.Ident:
		if val, ok := e.constBinds[v.Name]; ok {
			return val, nil
		}
		if _, ok := e.inputs[v.Name]; ok {
			return nil, &diag.Diagnostic{
				Phase: diag.PhaseRuntime, Category: diag.CatNotImplemented,
				Pos: v.P, Msg: fmt.Sprintf("input %q has no readable value in this slice", v.Name),
			}
		}
		return nil, &diag.Diagnostic{
			Phase: diag.PhaseRuntime, Category: diag.CatTypeMismatch,
			Pos: v.P, Msg: fmt.Sprintf("undefined identifier %q", v.Name),
		}
	}
	return nil, &diag.Diagnostic{
		Phase: diag.PhaseRuntime, Category: diag.CatNotImplemented,
		Pos: x.Pos(), Msg: fmt.Sprintf("expression %T not supported in this slice", x),
	}
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
