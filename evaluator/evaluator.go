package evaluator

import (
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/resolved"
	"github.com/MoroZvlg/tascript/token"
)

type slotCell struct {
	value  registry.Value
	filled bool
}

type Evaluator struct {
	prog        *resolved.Program
	registry    *registry.Registry
	env         *Env
	slots       []slotCell
	slotDecls   []*resolved.SlotDecl
	inputDecls  []*resolved.InputDecl
	outputDecls []*resolved.OutputDecl
	inputs      map[string]registry.Value
	outputs     map[string]registry.Sink
	currFn      string // Entry point/function name, for runtime diagnostics
	// inTick guards the slot handle API and Init/Run against re-entrancy
	inTick bool
}

func New(prog *resolved.Program, reg *registry.Registry) *Evaluator {
	e := &Evaluator{
		prog:     prog,
		registry: reg,
		env:      EnvFromRegistry(reg),
		inputs:   make(map[string]registry.Value),
		outputs:  make(map[string]registry.Sink),
	}

	for _, decl := range prog.Decls {
		switch typedDecl := decl.(type) {
		case *resolved.SlotDecl:
			e.slotDecls = append(e.slotDecls, typedDecl)
		case *resolved.InputDecl:
			e.inputDecls = append(e.inputDecls, typedDecl)
		case *resolved.OutputDecl:
			e.outputDecls = append(e.outputDecls, typedDecl)
		}
	}
	e.slots = make([]slotCell, len(e.slotDecls))

	return e
}

var (
	ErrMidActivation = errors.New("call into the engine while it is mid-activation")
	ErrSlotEmpty     = errors.New("slot has no value yet")
)

func (e *Evaluator) SlotDecls() []*resolved.SlotDecl {
	return e.slotDecls
}

func (e *Evaluator) SlotGet(idx int) (registry.Value, error) {
	if e.inTick {
		return nil, ErrMidActivation
	}
	cell := e.slots[idx]
	if !cell.filled {
		return nil, ErrSlotEmpty
	}
	return cell.value, nil
}

func (e *Evaluator) SlotSet(idx int, value registry.Value) error {
	if e.inTick {
		return ErrMidActivation
	}
	decl := e.slotDecls[idx]
	if value.TypeID() != decl.T {
		rule, canCoerce := e.registry.LookupCoerce(value.TypeID(), decl.T)
		if !canCoerce {
			return diag.SlotTypeMismatch{
				At:       decl.Token.Pos,
				Kind:     decl.Kind,
				Name:     decl.Name,
				Expected: decl.T,
				Got:      value.TypeID(),
			}
		}
		value = rule.EvalFn(value)
	}
	e.slots[idx] = slotCell{value: value, filled: true}
	return nil
}

func (e *Evaluator) BindInput(name string, value registry.Value) error {
	for _, decl := range e.inputDecls {
		if decl.Name != name {
			continue
		}
		if value.TypeID() != decl.T {
			rule, canCoerce := e.registry.LookupCoerce(value.TypeID(), decl.T)
			if !canCoerce {
				return diag.InputTypeMismatch{
					At:       decl.Token.Pos,
					Name:     name,
					Expected: decl.T,
					Got:      value.TypeID(),
				}
			}
			value = rule.EvalFn(value)
		}
		e.inputs[name] = value
		return nil
	}
	return diag.InputUnknown{Name: name}
}

func (e *Evaluator) BindOutput(name string, sink registry.Sink) error {
	for _, decl := range e.outputDecls {
		if decl.Name == name {
			e.outputs[name] = sink
			return nil
		}
	}
	return diag.OutputUnknown{Name: name}
}

// EvalInit is the load phase and must be called once before the first EvalRun.
// An error means the program never reached a valid initial state
func (e *Evaluator) EvalInit() (result registry.Value, err error) {
	if e.inTick {
		return nil, ErrMidActivation
	}
	e.currFn = "Init"
	e.inTick = true
	defer func() {
		e.inTick = false
		if r := recover(); r != nil {
			result, err = nil, e.internalFailure(r)
		}
	}()

	for _, decl := range e.outputDecls {
		if _, bound := e.outputs[decl.Name]; !bound {
			return nil, diag.OutputMissing{At: decl.Token.Pos, Name: decl.Name}
		}
	}

	for _, decl := range e.prog.Decls {
		switch typedDecl := decl.(type) {
		case *resolved.ConstDecl:
			if err := e.evalConst(typedDecl, e.env); err != nil {
				return nil, err
			}
		case *resolved.SlotDecl:
			// a host fill at Wire wins: the initializer is only a fallback
			if e.slots[typedDecl.Index].filled || typedDecl.Init == nil {
				continue
			}
			value, err := e.evalExpr(typedDecl.Init, e.env)
			if err != nil {
				return nil, err
			}
			e.slots[typedDecl.Index] = slotCell{value: value, filled: true}
		}
	}

	if e.prog.InitFn != nil {
		result, err = e.evalBlock(e.prog.InitFn.Body, NewEnclosedEnv(e.env))
		if err != nil {
			return nil, err
		}
	}

	for _, decl := range e.slotDecls {
		if !e.slots[decl.Index].filled {
			return nil, diag.RuntimeFailure{
				At:      decl.Token.Pos,
				Kind:    registry.UninitializedSlot,
				Message: fmt.Sprintf("%s.%s left uninitialized after Init", decl.Kind, decl.Name),
			}
		}
	}

	return result, nil
}

// EvalRun executes one tick.
// An aborted tick is unfinished, not undone: slot writes before the error persist.
// Bound inputs must stay immutable for the call's duration (host-owned coherence).
// TODO: result is temp for debug/tests.
func (e *Evaluator) EvalRun() (result registry.Value, err error) {
	if e.inTick {
		return nil, ErrMidActivation
	}
	e.currFn = "Run"
	e.inTick = true
	defer func() {
		e.inTick = false
		if r := recover(); r != nil {
			result, err = nil, e.internalFailure(r)
		}
	}()

	runEnv := NewEnclosedEnv(e.env)
	if err := e.bindInputs(runEnv); err != nil {
		return nil, err
	}

	return e.evalBlock(e.prog.RunFn.Body, runEnv)
}

func (e *Evaluator) bindInputs(env *Env) error {
	for _, decl := range e.inputDecls {
		value, ok := e.inputs[decl.Name]
		if !ok {
			return diag.InputMissing{At: decl.Token.Pos, Name: decl.Name}
		}
		env.Set(decl.Name, value)
	}

	return nil
}

func (e *Evaluator) internalFailure(panicValue any) error {
	return diag.InternalFailure{
		EntryFn: e.currFn,
		Panic:   panicValue,
		Stack:   debug.Stack(),
	}
}

func (e *Evaluator) runtimeFailure(ruleErr error, tok token.Token) error {
	var regErr registry.Error
	if !errors.As(ruleErr, &regErr) {
		// non-registry error (e.g. a host rule returned a plain error)
		regErr = registry.Error{Kind: registry.UnknownKind, Message: ruleErr.Error()}
	}
	return diag.RuntimeFailure{
		At:      tok.Pos,
		Kind:    regErr.Kind,
		Message: regErr.Message,
	}
}

func (e *Evaluator) evalConst(decl *resolved.ConstDecl, env *Env) error {
	value, err := e.evalExpr(decl.Value, env)
	if err != nil {
		return err
	}
	env.Set(decl.Name, value)
	return nil
}

func (e *Evaluator) evalBlock(block *resolved.BlockStmt, env *Env) (registry.Value, error) {
	var last registry.Value
	for _, stmt := range block.Stmts {
		var err error
		last, err = e.evalStmt(stmt, env)
		if err != nil {
			return nil, err
		}
	}
	return last, nil
}

func (e *Evaluator) evalStmt(stmt resolved.Statement, env *Env) (registry.Value, error) {
	switch n := stmt.(type) {
	case *resolved.LetStmt:
		return e.evalLet(n, env)
	case *resolved.AssignNameStmt:
		return e.evalAssignName(n, env)
	case *resolved.ExprStmt:
		return e.evalExpr(n.Expr, env)
	case *resolved.BlockStmt:
		return e.evalBlock(n, NewEnclosedEnv(env))
	case *resolved.IfStmt:
		return e.evalIf(n, env)
	case *resolved.EmitStmt:
		return e.evalEmit(n, env)
	case *resolved.AssignSlotStmt:
		return e.evalAssignSlot(n, env)
	default:
		panic(fmt.Sprintf("unhandled resolved statement %T: the resolver produced a node the evaluator never wired up", stmt))
	}
}

func (e *Evaluator) evalAssignSlot(stmt *resolved.AssignSlotStmt, env *Env) (registry.Value, error) {
	value, err := e.evalExpr(stmt.Value, env)
	if err != nil {
		return nil, err
	}
	e.slots[stmt.Target.Index] = slotCell{value: value, filled: true}
	return value, nil
}

func (e *Evaluator) evalIf(stmt *resolved.IfStmt, env *Env) (registry.Value, error) {
	condition, err := e.evalExpr(stmt.Condition, env)
	if err != nil {
		return nil, err
	}
	condBool, ok := condition.(registry.Bool)
	if !ok {
		panic("if condition is not Bool: resolver failed to guard boolean position")
	}

	if condBool {
		return e.evalBlock(stmt.Consequence, NewEnclosedEnv(env))
	}
	if stmt.Else != nil {
		return e.evalStmt(stmt.Else, env)
	}
	return nil, nil
}

func (e *Evaluator) evalEmit(stmt *resolved.EmitStmt, env *Env) (registry.Value, error) {
	def, _ := e.registry.LookupTypeDef(stmt.T)

	var value registry.Value
	if len(def.Fields) == 0 {
		scalar, err := e.evalExpr(stmt.Args[0].Value, env)
		if err != nil {
			return nil, err
		}
		value = scalar
	} else {
		fields := make(map[string]registry.Value, len(stmt.Args))
		for _, arg := range stmt.Args {
			fieldValue, err := e.evalExpr(arg.Value, env)
			if err != nil {
				return nil, err
			}
			fields[arg.Name] = fieldValue
		}
		value = registry.Record{T: stmt.T, Fields: fields}
	}

	e.outputs[stmt.Output].Emit(value)
	return nil, nil
}

func (e *Evaluator) evalLet(stmt *resolved.LetStmt, env *Env) (registry.Value, error) {
	value, err := e.evalExpr(stmt.Value, env)
	if err != nil {
		return nil, err
	}
	return env.Set(stmt.Name, value), nil
}

func (e *Evaluator) evalAssignName(stmt *resolved.AssignNameStmt, env *Env) (registry.Value, error) {
	value, err := e.evalExpr(stmt.Value, env)
	if err != nil {
		return nil, err
	}

	if !env.Assign(stmt.Target, value) {
		// unreachable: the resolver rejects assignments to unknown bindings
		panic(fmt.Sprintf("variable %q does not exist", stmt.Target))
	}
	return value, nil
}

func (e *Evaluator) evalExpr(expr resolved.Expression, env *Env) (registry.Value, error) {
	switch n := expr.(type) {
	case *resolved.IntegerExpr:
		return registry.Integer(n.Value), nil
	case *resolved.FloatExpr:
		return registry.Float(n.Value), nil
	case *resolved.StringExpr:
		return registry.String(n.Value), nil
	case *resolved.BooleanExpr:
		return registry.Bool(n.Value), nil
	case *resolved.IdentExpr:
		return e.evalIdent(n, env)
	case *resolved.InfixExpr:
		return e.evalInfix(n, env)
	case *resolved.LogicalExpr:
		return e.evalLogical(n, env)
	case *resolved.PrefixExpr:
		return e.evalPrefix(n, env)
	case *resolved.CoerceExpr:
		innerValue, err := e.evalExpr(n.Inner, env)
		if err != nil {
			return nil, err
		}
		return n.EvalFn(innerValue), nil
	case *resolved.MemberAccessExpr:
		return e.evalMemberAccess(n, env)
	case *resolved.MethodCallExpr:
		return e.evalMethodCall(n, env)
	case *resolved.SlotRefExpr:
		return e.evalSlotRef(n)
	default:
		panic(fmt.Sprintf("unhandled resolved expression %T: the resolver produced a node the evaluator never wired up", expr))
	}
}

// evalSlotRef can hit an empty cell: an initializer may read a slot above it that
// the host was expected to fill at Wire and did not.
func (e *Evaluator) evalSlotRef(expr *resolved.SlotRefExpr) (registry.Value, error) {
	cell := e.slots[expr.Slot.Index]
	if !cell.filled {
		return nil, diag.RuntimeFailure{
			At:      expr.Token.Pos,
			Kind:    registry.UninitializedSlot,
			Message: fmt.Sprintf("%s.%s read before it was filled", expr.Slot.Kind, expr.Slot.Name),
		}
	}
	return cell.value, nil
}

func (e *Evaluator) evalIdent(expr *resolved.IdentExpr, env *Env) (registry.Value, error) {
	value, ok := env.Get(expr.String())
	if !ok {
		// unreachable: the resolver rejects unknown identifiers
		panic(fmt.Sprintf("unknown identifier %q", expr.String()))
	}
	return value, nil
}

func (e *Evaluator) evalInfix(expr *resolved.InfixExpr, env *Env) (registry.Value, error) {
	left, err := e.evalExpr(expr.Left, env)
	if err != nil {
		return nil, err
	}

	right, err := e.evalExpr(expr.Right, env)
	if err != nil {
		return nil, err
	}

	value, err := expr.EvalFn(left, right)
	if err != nil {
		return nil, e.runtimeFailure(err, expr.Token)
	}
	return value, nil
}

func (e *Evaluator) evalLogical(expr *resolved.LogicalExpr, env *Env) (registry.Value, error) {
	left, err := e.evalExpr(expr.Left, env)
	if err != nil {
		return nil, err
	}
	leftBool, ok := left.(registry.Bool)
	if !ok {
		panic("logical operand is not Bool: resolver failed to guard boolean position")
	}

	switch expr.Token.Type {
	case token.AND:
		if !leftBool {
			return registry.Bool(false), nil
		}
	case token.OR:
		if leftBool {
			return registry.Bool(true), nil
		}
	}

	right, err := e.evalExpr(expr.Right, env)
	if err != nil {
		return nil, err
	}
	rightBool, ok := right.(registry.Bool)
	if !ok {
		panic("logical operand is not Bool: resolver failed to guard boolean position")
	}
	return rightBool, nil
}

func (e *Evaluator) evalPrefix(expr *resolved.PrefixExpr, env *Env) (registry.Value, error) {
	right, err := e.evalExpr(expr.Right, env)
	if err != nil {
		return nil, err
	}

	value, err := expr.EvalFn(right)
	if err != nil {
		return nil, e.runtimeFailure(err, expr.Token)
	}
	return value, nil
}

func (e *Evaluator) evalMemberAccess(expr *resolved.MemberAccessExpr, env *Env) (registry.Value, error) {
	object, err := e.evalExpr(expr.Object, env)
	if err != nil {
		return nil, err
	}

	value, err := expr.EvalFn(object)
	if err != nil {
		return nil, e.runtimeFailure(err, expr.Token)
	}
	return value, nil
}

func (e *Evaluator) evalMethodCall(expr *resolved.MethodCallExpr, env *Env) (registry.Value, error) {
	resolvedRec, err := e.evalExpr(expr.Receiver, env)
	if err != nil {
		return nil, err
	}
	args := make(map[string]registry.Value)

	for _, arg := range expr.Args {
		value, err := e.evalExpr(arg.Value, env)
		if err != nil {
			return nil, err
		}
		args[arg.Name] = value
	}

	value, err := expr.EvalFn(resolvedRec, args)
	if err != nil {
		return nil, e.runtimeFailure(err, expr.Token)
	}
	return value, nil
}
