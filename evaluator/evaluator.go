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

func (e *Evaluator) EvalRun() (registry.Value, error) {
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
	case *resolved.AssignStmt:
		return e.evalAssign(n, env)
	case *resolved.ExprStmt:
		return e.evalExpr(n.Expr, env)
	case *resolved.BlockStmt:
		return e.evalBlock(n, NewEnclosedEnv(env))
	default:
		return nil, fmt.Errorf("unsupported statement %T", stmt)
	}
}

func (e *Evaluator) evalLet(stmt *resolved.LetStmt, env *Env) (registry.Value, error) {
	value, err := e.evalExpr(stmt.Value, env)
	if err != nil {
		return nil, err
	}
	return env.Set(stmt.Name.String(), value), nil
}

func (e *Evaluator) evalAssign(stmt *resolved.AssignStmt, env *Env) (registry.Value, error) {
	value, err := e.evalExpr(stmt.Value, env)
	if err != nil {
		return nil, err
	}

	return env.Set(stmt.Target.String(), value), nil
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
	//case *ast.MemberAccessExpr:
	//	return e.evalMemberAccess(n, env)
	//case *ast.CallExpr:
	//	return e.evalCall(n, env)
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

//func (e *Evaluator) evalMemberAccess(expr *resolved.MemberAccessExpr, env *Env) (registry.Value, error) {
//	object, err := e.evalExpr(expr.Object, env)
//	if err != nil {
//		return nil, err
//	}
//	rule, exists := e.registry.LookupMemberAccess(object.TypeID(), expr.Method.String())
//	if !exists {
//		// TODO: unreachable? we are doing the same lookup on resolve stage?
//		return nil, fmt.Errorf("undefined attribute %s for %s", expr.Method, expr.Object)
//	}
//	return rule.EvalFn(), nil
//}
//
//func (e *Evaluator) evalCall(expr *resolved.CallExpr, env *Env) (registry.Value, error) {
//	switch callee := expr.Callee.(type) {
//	case *ast.MemberAccessExpr:
//		object, err := e.evalExpr(callee.Object, env)
//		if err != nil {
//			return nil, err
//		}
//		callRule, exists := e.registry.LookupCall(object.TypeID(), callee.Method.String())
//		if !exists {
//			// TODO: unreachable? we are doing the same lookup on resolve stage?
//			return nil, fmt.Errorf("undefined attribute %s for %s", callee.Method, callee.Object)
//		}
//
//		args := make(map[string]registry.Value)
//		for i, arg := range expr.Args {
//			value, err := e.evalExpr(arg, env)
//			if err != nil {
//				return nil, err
//			}
//			argRule := callRule.Args[i]
//			args[argRule.Name] = value
//		}
//
//		for _, kwArg := range expr.Kwargs {
//			value, err := e.evalExpr(kwArg.Value, env)
//			if err != nil {
//				return nil, err
//			}
//			for _, argRule := range callRule.Args {
//				if kwArg.Key.String() == argRule.Name {
//					args[argRule.Name] = value
//					break
//				}
//			}
//		}
//		return callRule.EvalFn(args), nil
//	default:
//		return nil, fmt.Errorf("call on type %T not supported", callee)
//	}
//
//}
