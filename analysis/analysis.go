// Package analysis performs tascript static checks after syntax parsing.
package analysis

import (
	"fmt"

	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/token"
)

// Analyze validates a parsed program against the currently implemented
// language slice. Diagnostics still use PhaseParse because they are surfaced
// to users before launch/runtime, but this package is deliberately separate
// from syntax parsing.
func Analyze(prog *ast.Program, reg *registry.Registry) []diag.Diagnostic {
	a := &analyzer{outputs: map[string]*ast.OutputDecl{}, registry: reg.Clone()}
	a.analyze(prog)
	return a.diags
}

type analyzer struct {
	diags    []diag.Diagnostic
	outputs  map[string]*ast.OutputDecl
	registry *registry.Registry
}

// reservedKwargs are emit() keyword names the runtime injects itself; user
// code may not supply them.
var reservedKwargs = map[string]struct{}{
	"ts":     {},
	"output": {},
}

func (a *analyzer) analyze(prog *ast.Program) {
	a.collectOutputs(prog)
	a.checkRequiredFns(prog)
	a.checkTopNames(prog)
	a.checkTopDecls(prog)
	a.checkFuncs(prog)
}

func (a *analyzer) collectOutputs(prog *ast.Program) {
	for _, d := range prog.Decls {
		if o, ok := d.(*ast.OutputDecl); ok {
			a.outputs[o.Name] = o
		}
	}
}

func (a *analyzer) checkRequiredFns(prog *ast.Program) {
	if prog.Init == nil {
		a.addErrf(token.Pos{}, diag.CatMissingRequiredFn,
			"program is missing required 'function Init()'")
	}
	if prog.Run == nil {
		a.addErrf(token.Pos{}, diag.CatMissingRequiredFn,
			"program is missing required 'function Run()'")
	}
}

// checkTopNames enforces the single shared top-level namespace (§3.3):
// inputs, outputs, constants, and functions may not collide.
func (a *analyzer) checkTopNames(prog *ast.Program) {
	seen := map[string]token.Pos{}
	declare := func(name string, pos token.Pos) {
		if prev, ok := seen[name]; ok {
			a.addErrf(pos, diag.CatPortDuplicate,
				"top-level name %q already declared at %s", name, prev)
			return
		}
		seen[name] = pos
	}
	for _, d := range prog.Decls {
		switch x := d.(type) {
		case *ast.ConstDecl:
			declare(x.Name, x.NamePos)
		case *ast.InputDecl:
			declare(x.Name, x.NamePos)
		case *ast.OutputDecl:
			declare(x.Name, x.NamePos)
		case *ast.FuncDecl:
			declare(x.Name, x.NamePos)
		}
	}
}

func (a *analyzer) checkTopDecls(prog *ast.Program) {
	for _, d := range prog.Decls {
		switch x := d.(type) {
		case *ast.ConstDecl:
			switch x.Value.(type) {
			case *ast.NumberLit, *ast.StringLit:
				// Slice 0 accepts only Number/String literal constants.
			default:
				a.addErrf(x.Value.Pos(), diag.CatTopLevelForm,
					"top-level constants must be Number or String literal values in this slice")
			}
		case *ast.InputDecl:
			spec, ok := a.registry.Type(x.Type)
			if !ok || !spec.Input {
				a.addErrf(x.TypePos, diag.CatTopLevelForm,
					"%q is not a registered input type", x.Type)
			}
		case *ast.OutputDecl:
			if x.ValueType != "" {
				spec, ok := a.registry.Type(x.ValueType)
				if !ok || !spec.Value {
					a.addErrf(x.ValueTypePos, diag.CatTopLevelForm,
						"%q is not a registered output value type", x.ValueType)
				}
			}
			for _, field := range x.Fields {
				spec, ok := a.registry.Type(field.Type)
				if !ok || !spec.Field {
					a.addErrf(field.TypePos, diag.CatTopLevelForm,
						"%q is not a registered output field type", field.Type)
				}
			}
		}
	}
}

func (a *analyzer) checkFuncs(prog *ast.Program) {
	for _, d := range prog.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name != "Init" && fn.Name != "Run" {
			a.addErrf(fn.NamePos, diag.CatTopLevelForm,
				"only Init() and Run() functions are allowed in this slice (got %q)", fn.Name)
		}
		if len(fn.Params) > 0 {
			a.addErrf(fn.Params[0].NamePos, diag.CatTopLevelForm,
				"function parameters are not supported in this slice")
		}
		for _, s := range fn.Body {
			a.checkStmt(fn.Name, s)
		}
	}
}

func (a *analyzer) checkStmt(fnName string, s ast.Stmt) {
	switch x := s.(type) {
	case *ast.EmitStmt:
		if fnName != "Run" {
			a.addErrf(x.CallPos, diag.CatEmitOutsideRun,
				"emit(...) is only allowed inside function Run()")
			return
		}
		a.checkEmit(x)
	case *ast.AssignStmt:
		if !isStateMember(x.Target) {
			a.addErrf(x.Pos(), diag.CatNotImplemented,
				"only state.* assignment is implemented in this slice")
		}
		a.checkExprImplemented(x.Value)
	case *ast.IfStmt:
		a.checkExprImplemented(x.Condition)
		for _, nested := range x.Consequence {
			a.checkStmt(fnName, nested)
		}
		for _, nested := range x.Alternative {
			a.checkStmt(fnName, nested)
		}
	case *ast.ExprStmt:
		a.addErrf(x.Pos(), diag.CatNotImplemented,
			"expression statements are not implemented in this slice")
	default:
		a.addErrf(s.Pos(), diag.CatNotImplemented,
			"statement %T is not implemented in this slice", s)
	}
}

func (a *analyzer) checkEmit(em *ast.EmitStmt) {
	out, ok := a.outputs[em.Output]
	if !ok {
		a.addErrf(em.OutputPos, diag.CatUnknownOutput,
			"emit() targets %q which is not a declared output", em.Output)
		return
	}

	if em.Value != nil {
		a.checkExprImplemented(em.Value)
	}
	for _, kw := range em.Kwargs {
		a.checkExprImplemented(kw.Value)
	}

	// Positional value must agree with the declared value type's presence.
	if em.Value != nil && out.ValueType == "" {
		a.addErrf(em.Value.Pos(), diag.CatEmitPayload,
			"output %q declares no value type; remove the positional value", em.Output)
	}
	if em.Value == nil && out.ValueType != "" {
		a.addErrf(em.CallPos, diag.CatEmitPayload,
			"output %q declares value type %q; emit() must supply a positional value", em.Output, out.ValueType)
	}

	// Field-name closedness (names only - value type-matching is deferred).
	declared := map[string]bool{}
	for _, f := range out.Fields {
		declared[f.Name] = true
	}
	supplied := map[string]bool{}
	for _, kw := range em.Kwargs {
		supplied[kw.Name] = true
		if _, reserved := reservedKwargs[kw.Name]; reserved {
			a.addErrf(kw.NamePos, diag.CatEmitReservedKwarg,
				"%q is reserved and is injected by the runtime", kw.Name)
			continue
		}
		if !declared[kw.Name] {
			a.addErrf(kw.NamePos, diag.CatEmitPayload,
				"output %q has no declared field %q", em.Output, kw.Name)
		}
	}
	for _, f := range out.Fields {
		if !supplied[f.Name] {
			a.addErrf(em.CallPos, diag.CatEmitPayload,
				"emit() to %q is missing declared field %q", em.Output, f.Name)
		}
	}
}

func (a *analyzer) checkExprImplemented(x ast.Expr) {
	switch v := x.(type) {
	case *ast.NumberLit, *ast.StringLit, *ast.Ident:
		return
	case *ast.BoolLit:
		return
	case *ast.UnaryExpr:
		if v.Op != token.MINUS && v.Op != token.BANG {
			a.addErrf(v.OpPos, diag.CatNotImplemented,
				"unary operator %s is not implemented in this slice", v.Op)
		}
		a.checkExprImplemented(v.Right)
	case *ast.BinaryExpr:
		switch v.Op {
		case token.PLUS, token.MINUS, token.ASTERISK, token.SLASH, token.PERCENT:
			a.checkExprImplemented(v.Left)
			a.checkExprImplemented(v.Right)
		case token.EQ, token.NEQ, token.LT, token.LTE, token.GT, token.GTE, token.AND, token.OR:
			a.checkExprImplemented(v.Left)
			a.checkExprImplemented(v.Right)
		default:
			a.addErrf(v.OpPos, diag.CatNotImplemented,
				"binary operator %s is not implemented in this slice", v.Op)
		}
	case *ast.MemberExpr:
		a.checkExprImplemented(v.Object)
	case *ast.CallExpr:
		spec, ok := a.helperSpec(v)
		if !ok {
			a.addErrf(x.Pos(), diag.CatNotImplemented,
				"expression %T is not implemented in this slice", x)
			return
		}
		if err := registry.ValidateArgCount(spec.Namespace+"."+spec.Name, spec.MinArgs, spec.MaxArgs, len(v.Args)); err != nil {
			a.addErrf(v.LPos, diag.CatTypeMismatch, "%s", err)
		}
		for _, arg := range v.Args {
			if arg.Name != "" {
				a.addErrf(arg.NamePos, diag.CatTypeMismatch,
					"%s.%s does not accept keyword arguments", spec.Namespace, spec.Name)
				continue
			}
			a.checkExprImplemented(arg.Value)
		}
	case *ast.IndexExpr:
		a.addErrf(x.Pos(), diag.CatNotImplemented,
			"expression %T is not implemented in this slice", x)
	default:
		a.addErrf(x.Pos(), diag.CatNotImplemented,
			"expression %T is not implemented in this slice", x)
	}
}

func isStateMember(x ast.Expr) bool {
	m, ok := x.(*ast.MemberExpr)
	return ok && isIdent(m.Object, "state")
}

func (a *analyzer) helperSpec(x *ast.CallExpr) (registry.HelperSpec, bool) {
	m, ok := x.Callee.(*ast.MemberExpr)
	if !ok {
		return registry.HelperSpec{}, false
	}
	ns, ok := m.Object.(*ast.Ident)
	if !ok {
		return registry.HelperSpec{}, false
	}
	return a.registry.Helper(ns.Name, m.Name)
}

func isIdent(x ast.Expr, name string) bool {
	id, ok := x.(*ast.Ident)
	return ok && id.Name == name
}

func (a *analyzer) addErrf(pos token.Pos, cat diag.Category, format string, args ...any) {
	a.diags = append(a.diags, diag.Diagnostic{
		Phase: diag.PhaseParse, Category: cat, Pos: pos, Msg: fmt.Sprintf(format, args...),
	})
}
