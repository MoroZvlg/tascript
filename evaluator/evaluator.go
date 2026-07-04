package evaluator

import (
	"fmt"

	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/resolved"
)

type Evaluator struct {
	prog     *resolved.Program
	registry *registry.Registry
	env      *Env
	// TODO: temporary emission sink — collects emitted values until real
	// output channels to the host are wired (Engine/host API rework).
	emitted []registry.NamedValue
}

func New(prog *resolved.Program, reg *registry.Registry) *Evaluator {
	return &Evaluator{
		prog:     prog,
		registry: reg,
		env:      EnvFromRegistry(reg),
	}
}

func (e *Evaluator) EvalInit() (registry.Value, error) {
	return nil, nil
}

// Emitted returns the values emitted by the last EvalRun, in emission order.
// TODO: temporary API, see the emitted field.
func (e *Evaluator) Emitted() []registry.NamedValue {
	return e.emitted
}

func (e *Evaluator) EvalRun() (registry.Value, error) {
	e.emitted = nil

	for _, decl := range e.prog.Consts {
		err := e.evalConst(decl, e.env)
		if err != nil {
			return nil, err
		}
	}

	return e.evalBlock(e.prog.RunFn.Body, NewEnclosedEnv(e.env))
}

func (e *Evaluator) evalConst(decl *resolved.ConstDecl, env *Env) error {
	value, err := e.evalExpr(decl.Value, env)
	if err != nil {
		return err
	}
	env.Set(decl.Name.String(), value)
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
	default:
		return nil, fmt.Errorf("unsupported statement %T", stmt)
	}
}

func (e *Evaluator) evalIf(stmt *resolved.IfStmt, env *Env) (registry.Value, error) {
	condition, err := e.evalExpr(stmt.Condition, env)
	if err != nil {
		return nil, err
	}
	condBool, _ := condition.(registry.Bool)

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
		return nil, fmt.Errorf(`variable "%s" does not exist`, stmt.Target)
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
	default:
		return nil, fmt.Errorf("not implemented expression %T", expr)
	}
}

func (e *Evaluator) evalIdent(expr *resolved.IdentExpr, env *Env) (registry.Value, error) {
	value, ok := env.Get(expr.String())
	if !ok {
		return nil, fmt.Errorf("unknown identifier %q", expr.String())
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

	return expr.EvalFn(left, right), nil
}

func (e *Evaluator) evalPrefix(expr *resolved.PrefixExpr, env *Env) (registry.Value, error) {
	right, err := e.evalExpr(expr.Right, env)
	if err != nil {
		return nil, err
	}

	return expr.EvalFn(right), nil
}

func (e *Evaluator) evalMemberAccess(expr *resolved.MemberAccessExpr, env *Env) (registry.Value, error) {
	object, err := e.evalExpr(expr.Object, env)
	if err != nil {
		return nil, err
	}

	return expr.EvalFn(object), nil
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

	return expr.EvalFn(resolvedRec, args), nil
}
