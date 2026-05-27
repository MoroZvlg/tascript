// Package analysis performs tascript static checks after syntax parsing.
package analysis

import (
	"fmt"
	"math"

	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/token"
)

type Options struct {
	MaxHistoryIndex int
	MaxDiagnostics  int
	MaxEmitKwargs   int
	MaxExprDepth    int
}

type valueType string

const (
	typeUnknown      valueType = ""
	typeNumber       valueType = "Number"
	typeString       valueType = "String"
	typeBool         valueType = "Bool"
	typeTime         valueType = "Time"
	typeDuration     valueType = "Duration"
	typeSeries       valueType = "Series"
	typeTimeSeries   valueType = "TimeSeries"
	typeCandle       valueType = "Candle"
	typeCandleSeries valueType = "CandleSeries"
	typeTuple        valueType = "Tuple"
	typeNamespace    valueType = "Namespace"
)

// Analyze validates a parsed program against the currently implemented
// language slice. Diagnostics still use PhaseParse because they are surfaced
// to users before launch/runtime, but this package is deliberately separate
// from syntax parsing.
func Analyze(prog *ast.Program, reg *registry.Registry, opts Options) []diag.Diagnostic {
	a := &analyzer{
		outputs:         map[string]*ast.OutputDecl{},
		inputs:          map[string]*ast.InputDecl{},
		constants:       map[string]registry.Value{},
		constTypes:      map[string]valueType{},
		stateTypes:      map[string]valueType{},
		registry:        reg.Clone(),
		maxHistoryIndex: opts.MaxHistoryIndex,
		maxDiagnostics:  opts.MaxDiagnostics,
		maxEmitKwargs:   opts.MaxEmitKwargs,
		maxExprDepth:    opts.MaxExprDepth,
	}
	a.analyze(prog)
	return a.diags
}

type analyzer struct {
	diags           []diag.Diagnostic
	outputs         map[string]*ast.OutputDecl
	inputs          map[string]*ast.InputDecl
	constants       map[string]registry.Value
	constTypes      map[string]valueType
	stateTypes      map[string]valueType
	registry        *registry.Registry
	maxHistoryIndex int
	maxDiagnostics  int
	maxEmitKwargs   int
	maxExprDepth    int
}

// reservedKwargs are emit() keyword names the runtime injects itself; user
// code may not supply them.
var reservedKwargs = map[string]struct{}{
	"ts":     {},
	"output": {},
}

func (a *analyzer) analyze(prog *ast.Program) {
	a.collectOutputs(prog)
	a.collectInputs(prog)
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

func (a *analyzer) collectInputs(prog *ast.Program) {
	for _, d := range prog.Decls {
		if in, ok := d.(*ast.InputDecl); ok {
			a.inputs[in.Name] = in
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
			typ := a.exprType(x.Value)
			if typ != typeNumber && typ != typeString && typ != typeBool && typ != typeDuration {
				a.addErrf(x.Value.Pos(), diag.CatTopLevelForm,
					"top-level constants must be Number, String, Bool, or Duration constant expressions")
				break
			}
			a.constTypes[x.Name] = typ
			if val, ok := a.staticValue(x.Value); ok {
				a.constants[x.Name] = val
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
		if member, ok := x.Target.(*ast.MemberExpr); ok && isIdent(member.Object, "state") {
			typ := a.exprType(x.Value)
			if prev, ok := a.stateTypes[member.Name]; ok && prev != typeUnknown && typ != typeUnknown && prev != typ {
				a.addErrf(x.Pos(), diag.CatTypeMismatch,
					"state.%s was previously assigned %s, cannot assign %s", member.Name, prev, typ)
			}
			if typ != typeUnknown {
				a.stateTypes[member.Name] = typ
			}
		}
	case *ast.IfStmt:
		a.checkExprImplemented(x.Condition)
		a.requireAssignable(typeBool, a.exprType(x.Condition), x.Condition.Pos(), "if condition")
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
	if a.maxEmitKwargs > 0 && len(em.Kwargs) > a.maxEmitKwargs {
		a.addErrf(em.CallPos, diag.CatKwargLimit,
			"emit() has %d keyword arguments, exceeding the %d-argument limit", len(em.Kwargs), a.maxEmitKwargs)
	}
	for _, kw := range em.Kwargs {
		a.checkExprImplemented(kw.Value)
	}

	// Positional value must agree with the declared value type's presence.
	if em.Value != nil && out.ValueType == "" {
		a.addErrf(em.Value.Pos(), diag.CatEmitPayload,
			"output %q declares no value type; remove the positional value", em.Output)
	}
	if em.Value != nil && out.ValueType != "" {
		a.requireAssignable(valueType(out.ValueType), a.exprType(em.Value), em.Value.Pos(),
			fmt.Sprintf("output %q value", em.Output))
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
		for _, field := range out.Fields {
			if field.Name == kw.Name {
				a.requireAssignable(valueType(field.Type), a.exprType(kw.Value), kw.Value.Pos(),
					fmt.Sprintf("output %q field %q", em.Output, kw.Name))
				break
			}
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
	a.checkExprDepth(x, 1)
	a.checkExprImplementedNode(x)
}

func (a *analyzer) checkExprImplementedNode(x ast.Expr) {
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
		if ok {
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
			a.checkHelperLookback(v, spec)
			return
		}
		ind, ok := a.indicatorSpec(v)
		if !ok {
			a.addErrf(x.Pos(), diag.CatNotImplemented,
				"expression %T is not implemented in this slice", x)
			return
		}
		if err := registry.ValidateArgCount(ind.Name, ind.MinArgs, ind.MaxArgs, len(v.Args)); err != nil {
			a.addErrf(v.LPos, diag.CatTypeMismatch, "%s", err)
		}
		if member, ok := v.Callee.(*ast.MemberExpr); ok {
			a.checkExprImplemented(member.Object)
		}
		for _, arg := range v.Args {
			if arg.Name != "" {
				a.addErrf(arg.NamePos, diag.CatTypeMismatch,
					"%s does not accept keyword arguments in this slice", ind.Name)
				continue
			}
			if _, ok := a.staticValue(arg.Value); !ok {
				a.addErrf(arg.Value.Pos(), diag.CatTopLevelForm,
					"indicator %s arguments must be literal values or top-level constants", ind.Name)
			}
			a.checkExprImplemented(arg.Value)
		}
	case *ast.IndexExpr:
		a.checkExprImplemented(v.Object)
		a.checkHistoryIndex(v.Index)
	default:
		a.addErrf(x.Pos(), diag.CatNotImplemented,
			"expression %T is not implemented in this slice", x)
	}
}

func (a *analyzer) checkExprDepth(x ast.Expr, depth int) {
	if x == nil {
		return
	}
	if a.maxExprDepth > 0 && depth > a.maxExprDepth {
		a.addErrf(x.Pos(), diag.CatDepthLimit,
			"expression depth exceeds the %d-level limit", a.maxExprDepth)
		return
	}
	switch v := x.(type) {
	case *ast.UnaryExpr:
		a.checkExprDepth(v.Right, depth+1)
	case *ast.BinaryExpr:
		a.checkExprDepth(v.Left, depth+1)
		a.checkExprDepth(v.Right, depth+1)
	case *ast.MemberExpr:
		a.checkExprDepth(v.Object, depth+1)
	case *ast.IndexExpr:
		a.checkExprDepth(v.Object, depth+1)
		a.checkExprDepth(v.Index, depth+1)
	case *ast.CallExpr:
		a.checkExprDepth(v.Callee, depth+1)
		for _, arg := range v.Args {
			a.checkExprDepth(arg.Value, depth+1)
		}
	}
}

func (a *analyzer) exprType(x ast.Expr) valueType {
	switch v := x.(type) {
	case *ast.NumberLit:
		return typeNumber
	case *ast.StringLit:
		return typeString
	case *ast.BoolLit:
		return typeBool
	case *ast.Ident:
		switch v.Name {
		case "math", "ta", "time", "state":
			return typeNamespace
		}
		if typ, ok := a.constTypes[v.Name]; ok {
			return typ
		}
		if in, ok := a.inputs[v.Name]; ok {
			return valueType(in.Type)
		}
		return typeUnknown
	case *ast.UnaryExpr:
		right := a.exprType(v.Right)
		switch v.Op {
		case token.MINUS:
			if right == typeDuration {
				return typeDuration
			}
			if isNumberLike(right) {
				return typeNumber
			}
			if right != typeUnknown {
				a.addErrf(v.OpPos, diag.CatTypeMismatch, "unary '-' requires Number or Duration, got %s", right)
			}
		case token.BANG:
			if right == typeBool {
				return typeBool
			}
			if right != typeUnknown {
				a.addErrf(v.OpPos, diag.CatTypeMismatch, "unary '!' requires Bool, got %s", right)
			}
		}
		return typeUnknown
	case *ast.BinaryExpr:
		return a.binaryType(v)
	case *ast.MemberExpr:
		return a.memberType(v)
	case *ast.IndexExpr:
		obj := a.exprType(v.Object)
		switch obj {
		case typeSeries:
			return typeNumber
		case typeTimeSeries:
			return typeTime
		case typeCandleSeries:
			return typeCandle
		case typeTuple:
			return typeSeries
		case typeUnknown:
			return typeUnknown
		default:
			a.addErrf(v.LPos, diag.CatTypeMismatch, "indexing requires Series, TimeSeries, CandleSeries, or Tuple, got %s", obj)
			return typeUnknown
		}
	case *ast.CallExpr:
		if spec, ok := a.helperSpec(v); ok {
			if spec.ReturnType != "" {
				return valueType(spec.ReturnType)
			}
			return typeUnknown
		}
		if spec, ok := a.indicatorSpec(v); ok {
			if spec.ReturnType != "" {
				return valueType(spec.ReturnType)
			}
			return typeSeries
		}
		return typeUnknown
	default:
		return typeUnknown
	}
}

func (a *analyzer) memberType(x *ast.MemberExpr) valueType {
	if isIdent(x.Object, "state") {
		if typ, ok := a.stateTypes[x.Name]; ok {
			return typ
		}
		return typeUnknown
	}
	if isIdent(x.Object, "time") {
		switch x.Name {
		case "MILLISECOND", "SECOND", "MINUTE", "HOUR", "DAY", "WEEK":
			return typeDuration
		default:
			a.addErrf(x.NamePos, diag.CatTypeMismatch, "unknown time constant %q", x.Name)
			return typeUnknown
		}
	}

	obj := a.exprType(x.Object)
	switch obj {
	case typeCandleSeries:
		switch x.Name {
		case "opens", "highs", "lows", "closes", "volumes", "hl2", "hlc3":
			return typeSeries
		case "timestamps":
			return typeTimeSeries
		case "open", "high", "low", "close", "volume":
			return typeNumber
		case "ts":
			return typeTime
		}
	case typeCandle:
		switch x.Name {
		case "open", "high", "low", "close", "volume":
			return typeNumber
		case "ts":
			return typeTime
		}
	case typeTime:
		switch x.Name {
		case "unix_ms", "year", "month", "day", "weekday", "hour", "minute", "second", "millisecond":
			return typeNumber
		}
	case typeDuration:
		switch x.Name {
		case "unix_ms", "seconds", "minutes", "hours", "days", "weeks":
			return typeNumber
		}
	case typeUnknown, typeNamespace, typeSeries, typeTuple, typeTimeSeries:
		return typeUnknown
	}
	a.addErrf(x.NamePos, diag.CatTypeMismatch, "unknown member %q on %s", x.Name, obj)
	return typeUnknown
}

func (a *analyzer) binaryType(x *ast.BinaryExpr) valueType {
	left := a.exprType(x.Left)
	right := a.exprType(x.Right)
	switch x.Op {
	case token.AND, token.OR:
		if left != typeBool && left != typeUnknown {
			a.addErrf(x.OpPos, diag.CatTypeMismatch, "left operand of %s must be Bool, got %s", x.Op, left)
		}
		if right != typeBool && right != typeUnknown {
			a.addErrf(x.OpPos, diag.CatTypeMismatch, "right operand of %s must be Bool, got %s", x.Op, right)
		}
		return typeBool
	case token.EQ, token.NEQ:
		if !sameComparableType(left, right) {
			a.addErrf(x.OpPos, diag.CatTypeMismatch, "equality operands must have the same scalar type, got %s and %s", left, right)
		}
		return typeBool
	case token.LT, token.LTE, token.GT, token.GTE:
		if comparableOrderType(left, right) {
			return typeBool
		}
		a.addErrf(x.OpPos, diag.CatTypeMismatch, "comparison operands must both be Number, Time, or Duration, got %s and %s", left, right)
		return typeBool
	case token.PLUS, token.MINUS, token.ASTERISK, token.SLASH, token.PERCENT:
		return a.arithmeticType(x.Op, x.OpPos, left, right)
	default:
		return typeUnknown
	}
}

func (a *analyzer) arithmeticType(op token.Kind, pos token.Pos, left, right valueType) valueType {
	if isNumberLike(left) && isNumberLike(right) {
		return typeNumber
	}
	if left == typeTime && right == typeTime && op == token.MINUS {
		return typeDuration
	}
	if left == typeTime && right == typeDuration && (op == token.PLUS || op == token.MINUS) {
		return typeTime
	}
	if left == typeDuration && right == typeDuration {
		switch op {
		case token.PLUS, token.MINUS:
			return typeDuration
		case token.SLASH:
			return typeNumber
		}
	}
	if left == typeDuration && isNumberLike(right) {
		switch op {
		case token.ASTERISK, token.SLASH:
			return typeDuration
		}
	}
	if isNumberLike(left) && right == typeDuration && op == token.ASTERISK {
		return typeDuration
	}
	if left == typeUnknown || right == typeUnknown {
		return typeUnknown
	}
	a.addErrf(pos, diag.CatTypeMismatch, "unsupported arithmetic between %s and %s", left, right)
	return typeUnknown
}

func (a *analyzer) requireAssignable(dst, src valueType, pos token.Pos, ctx string) {
	if dst == typeUnknown || src == typeUnknown {
		return
	}
	if dst == src {
		return
	}
	if dst == typeNumber && src == typeSeries {
		return
	}
	if !isBuiltinType(dst) || !isBuiltinType(src) {
		return
	}
	a.addErrf(pos, diag.CatTypeMismatch, "%s expects %s, got %s", ctx, dst, src)
}

func isNumberLike(t valueType) bool {
	return t == typeNumber || t == typeSeries
}

func sameComparableType(left, right valueType) bool {
	if left == typeUnknown || right == typeUnknown {
		return true
	}
	if isNumberLike(left) && isNumberLike(right) {
		return true
	}
	return left == right && (left == typeString || left == typeBool || left == typeTime || left == typeDuration)
}

func comparableOrderType(left, right valueType) bool {
	if left == typeUnknown || right == typeUnknown {
		return true
	}
	if isNumberLike(left) && isNumberLike(right) {
		return true
	}
	return left == right && (left == typeTime || left == typeDuration)
}

func isBuiltinType(t valueType) bool {
	switch t {
	case typeNumber, typeString, typeBool, typeTime, typeDuration, typeSeries, typeCandle, typeCandleSeries, typeTuple, typeTimeSeries:
		return true
	default:
		return false
	}
}

func (a *analyzer) checkHistoryIndex(x ast.Expr) {
	lit, ok := x.(*ast.NumberLit)
	if !ok {
		a.addErrf(x.Pos(), diag.CatTopLevelForm,
			"history index must be a non-negative integer literal")
		return
	}
	if lit.Val < 0 || lit.Val != math.Trunc(lit.Val) {
		a.addErrf(x.Pos(), diag.CatTopLevelForm,
			"history index must be a non-negative integer literal")
		return
	}
	if a.maxHistoryIndex > 0 && lit.Val > float64(a.maxHistoryIndex) {
		a.addErrf(x.Pos(), diag.CatHistoryLimit,
			"history index %.0f exceeds the %d-bar limit", lit.Val, a.maxHistoryIndex)
	}
}

func (a *analyzer) checkHelperLookback(x *ast.CallExpr, spec registry.HelperSpec) {
	if spec.Lookback == nil {
		return
	}
	args := make([]registry.Value, 0, len(x.Args))
	for _, arg := range x.Args {
		v, ok := a.staticValue(arg.Value)
		if !ok {
			args = append(args, nil)
			continue
		}
		args = append(args, v)
	}
	lookback, err := spec.Lookback(args)
	if err != nil {
		a.addErrf(x.LPos, diag.CatTypeMismatch, "%s.%s: %s", spec.Namespace, spec.Name, err)
		return
	}
	if a.maxHistoryIndex > 0 && lookback > a.maxHistoryIndex {
		a.addErrf(x.LPos, diag.CatHistoryLimit,
			"%s.%s requires history index %d, exceeding the %d-bar limit",
			spec.Namespace, spec.Name, lookback, a.maxHistoryIndex)
	}
}

func (a *analyzer) staticValue(x ast.Expr) (registry.Value, bool) {
	switch v := x.(type) {
	case *ast.NumberLit:
		return v.Val, true
	case *ast.StringLit:
		return v.Val, true
	case *ast.BoolLit:
		return v.Val, true
	case *ast.Ident:
		val, ok := a.constants[v.Name]
		return val, ok
	case *ast.MemberExpr:
		if isIdent(v.Object, "time") {
			switch v.Name {
			case "MILLISECOND":
				return registry.Duration{Milliseconds: 1}, true
			case "SECOND":
				return registry.Duration{Milliseconds: 1000}, true
			case "MINUTE":
				return registry.Duration{Milliseconds: 60 * 1000}, true
			case "HOUR":
				return registry.Duration{Milliseconds: 60 * 60 * 1000}, true
			case "DAY":
				return registry.Duration{Milliseconds: 24 * 60 * 60 * 1000}, true
			case "WEEK":
				return registry.Duration{Milliseconds: 7 * 24 * 60 * 60 * 1000}, true
			}
		}
	case *ast.UnaryExpr:
		val, ok := a.staticValue(v.Right)
		if !ok {
			return nil, false
		}
		switch v.Op {
		case token.MINUS:
			switch x := val.(type) {
			case float64:
				return -x, true
			case registry.Duration:
				return registry.Duration{Milliseconds: -x.Milliseconds}, true
			}
		}
	case *ast.BinaryExpr:
		left, ok := a.staticValue(v.Left)
		if !ok {
			return nil, false
		}
		right, ok := a.staticValue(v.Right)
		if !ok {
			return nil, false
		}
		return staticBinaryValue(v.Op, left, right)
	case *ast.CallExpr:
		spec, ok := a.helperSpec(v)
		if !ok || spec.Eval == nil {
			return nil, false
		}
		args := make([]registry.Value, 0, len(v.Args))
		for _, arg := range v.Args {
			if arg.Name != "" {
				return nil, false
			}
			val, ok := a.staticValue(arg.Value)
			if !ok {
				return nil, false
			}
			args = append(args, val)
		}
		out, err := spec.Eval(args)
		return out, err == nil
	default:
		return nil, false
	}
	return nil, false
}

func staticBinaryValue(op token.Kind, left, right registry.Value) (registry.Value, bool) {
	if l, ok := left.(float64); ok {
		if r, ok := right.(float64); ok {
			switch op {
			case token.PLUS:
				return l + r, true
			case token.MINUS:
				return l - r, true
			case token.ASTERISK:
				return l * r, true
			case token.SLASH:
				return l / r, true
			case token.PERCENT:
				return math.Mod(l, r), true
			}
		}
		if r, ok := right.(registry.Duration); ok && op == token.ASTERISK {
			return registry.Duration{Milliseconds: int64(l * float64(r.Milliseconds))}, true
		}
	}
	if l, ok := left.(registry.Duration); ok {
		switch r := right.(type) {
		case registry.Duration:
			switch op {
			case token.PLUS:
				return registry.Duration{Milliseconds: l.Milliseconds + r.Milliseconds}, true
			case token.MINUS:
				return registry.Duration{Milliseconds: l.Milliseconds - r.Milliseconds}, true
			case token.SLASH:
				return float64(l.Milliseconds) / float64(r.Milliseconds), true
			}
		case float64:
			switch op {
			case token.ASTERISK:
				return registry.Duration{Milliseconds: int64(float64(l.Milliseconds) * r)}, true
			case token.SLASH:
				return registry.Duration{Milliseconds: int64(float64(l.Milliseconds) / r)}, true
			}
		}
	}
	return nil, false
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

func (a *analyzer) indicatorSpec(x *ast.CallExpr) (registry.IndicatorSpec, bool) {
	m, ok := x.Callee.(*ast.MemberExpr)
	if !ok {
		return registry.IndicatorSpec{}, false
	}
	spec, ok := a.registry.Indicator(m.Name)
	if !ok {
		return registry.IndicatorSpec{}, false
	}
	if root, ok := m.Object.(*ast.Ident); ok {
		if in, ok := a.inputs[root.Name]; ok {
			if in.Type != "CandleSeries" {
				a.addErrf(m.NamePos, diag.CatTypeMismatch,
					"indicator %s requires CandleSeries receiver", spec.Name)
			}
			return spec, true
		}
	}
	if spec.Scalar {
		return spec, true
	}
	return registry.IndicatorSpec{}, false
}

func isIdent(x ast.Expr, name string) bool {
	id, ok := x.(*ast.Ident)
	return ok && id.Name == name
}

func (a *analyzer) addErrf(pos token.Pos, cat diag.Category, format string, args ...any) {
	if a.maxDiagnostics > 0 && len(a.diags) >= a.maxDiagnostics {
		return
	}
	a.diags = append(a.diags, diag.Diagnostic{
		Phase: diag.PhaseParse, Category: cat, Pos: pos, Msg: fmt.Sprintf(format, args...),
	})
}
