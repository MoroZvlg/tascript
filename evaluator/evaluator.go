package evaluator

import (
	"errors"
	"fmt"
	"maps"
	"runtime/debug"

	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/resolved"
	"github.com/MoroZvlg/tascript/token"
)

type Evaluator struct {
	prog     *resolved.Program
	registry *registry.Registry
	env      *Env
	states   map[string]registry.Value
	inputs   map[string]registry.Value
	currFn   string // Entry point/function name, for runtime diagnostics
	// TODO: temporary emission sink — collects emitted values until real
	// output channels to the host are wired (Engine/host API rework).
	emitted []registry.NamedValue
}

func New(prog *resolved.Program, reg *registry.Registry) *Evaluator {
	return &Evaluator{
		prog:     prog,
		registry: reg,
		env:      EnvFromRegistry(reg),
		states:   make(map[string]registry.Value),
		inputs:   make(map[string]registry.Value),
	}
}

func (e *Evaluator) BindInput(name string, value registry.Value) error {
	for _, decl := range e.prog.Inputs {
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

// Emitted returns the values emitted by the last EvalRun, in emission order.
// TODO: temporary API, see the emitted field.
func (e *Evaluator) Emitted() []registry.NamedValue {
	return e.emitted
}

// EvalInit is the load phase and must be called once before the first EvalRun.
// An error means the program never reached a valid initial state
func (e *Evaluator) EvalInit() (result registry.Value, err error) {
	e.currFn = "Init"
	defer func() {
		if r := recover(); r != nil {
			result, err = nil, e.internalFailure(r)
		}
	}()

	for _, decl := range e.prog.Consts {
		if err := e.evalConst(decl, e.env); err != nil {
			return nil, err
		}
	}

	for _, field := range e.prog.State.Fields {
		if field.InitValue == nil {
			continue // will be seeded in Init()
		}
		value, err := e.evalExpr(field.InitValue, e.env)
		if err != nil {
			return nil, err
		}
		e.states[field.Name] = value
	}

	if e.prog.InitFn != nil {
		return e.evalBlock(e.prog.InitFn.Body, NewEnclosedEnv(e.env))
	}
	return nil, nil
}

// EvalRun executes one tick.
// On error the tick is rolled back (state + emits); a later Run may still be called.
// Bound inputs must stay immutable for the call's duration (host-owned coherence).
// TODO: result is temp for debug/tests.
func (e *Evaluator) EvalRun() (result registry.Value, err error) {
	e.currFn = "Run"
	snapshot := maps.Clone(e.states)
	defer func() {
		if r := recover(); r != nil {
			result, err = nil, e.internalFailure(r)
		}
		if err != nil {
			e.states = snapshot
			e.emitted = nil
		}
	}()

	e.emitted = nil

	runEnv := NewEnclosedEnv(e.env)
	if err := e.bindInputs(runEnv); err != nil {
		return nil, err
	}

	return e.evalBlock(e.prog.RunFn.Body, runEnv)
}

func (e *Evaluator) bindInputs(env *Env) error {
	for _, decl := range e.prog.Inputs {
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
	case *resolved.AssignStateStmt:
		return e.evalAssignState(n, env)
	default:
		panic(fmt.Sprintf("unhandled resolved statement %T: the resolver produced a node the evaluator never wired up", stmt))
	}
}

func (e *Evaluator) evalAssignState(stmt *resolved.AssignStateStmt, env *Env) (registry.Value, error) {
	value, err := e.evalExpr(stmt.Value, env)
	if err != nil {
		return nil, err
	}
	e.states[stmt.Target.Name] = value
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

	e.emitted = append(e.emitted, registry.NamedValue{Name: stmt.Output, Value: value})
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
	case *resolved.StateAccessExpr:
		return e.evalStateAccess(n)
	default:
		panic(fmt.Sprintf("unhandled resolved expression %T: the resolver produced a node the evaluator never wired up", expr))
	}
}

func (e *Evaluator) evalStateAccess(expr *resolved.StateAccessExpr) (registry.Value, error) {
	value, ok := e.states[expr.Field]
	if !ok {
		// unreachable: definite-assignment analysis rejects unseeded reads
		panic(fmt.Sprintf("state field %s read before initialization", expr.Field))
	}
	return value, nil
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
