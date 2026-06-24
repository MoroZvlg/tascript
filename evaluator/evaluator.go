package evaluator

import (
	"fmt"

	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/registry"
)

type Evaluator struct {
	prog     *ast.Program
	registry *registry.Registry
	env      *Env
}

func New(prog *ast.Program, reg *registry.Registry) *Evaluator {
	return &Evaluator{
		prog:     prog,
		registry: reg,
		env:      NewEnv(),
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

func (e *Evaluator) evalConst(decl *ast.ConstDecl, env *Env) error {
	value, err := e.evalExpr(decl.Value, env)
	if err != nil {
		return err
	}
	env.Set(decl.Identifier.String(), value)
	return nil
}

func (e *Evaluator) evalBlock(block *ast.BlockStmt, env *Env) (registry.Value, error) {
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

func (e *Evaluator) evalStmt(stmt ast.Statement, env *Env) (registry.Value, error) {
	switch n := stmt.(type) {
	case *ast.LetStmt:
		return e.evalLet(n, env)
	case *ast.AssignStmt:
		return e.evalAssign(n, env)
	case *ast.ExprStmt:
		return e.evalExpr(n.Expr, env)
	case *ast.BlockStmt:
		return e.evalBlock(n, NewEnclosedEnv(env))
	default:
		return nil, fmt.Errorf("unsupported statement %T", stmt)
	}
}

func (e *Evaluator) evalLet(stmt *ast.LetStmt, env *Env) (registry.Value, error) {
	value, err := e.evalExpr(stmt.Value, env)
	if err != nil {
		return nil, err
	}
	return env.Set(stmt.Name.String(), value), nil
}

func (e *Evaluator) evalAssign(stmt *ast.AssignStmt, env *Env) (registry.Value, error) {
	value, err := e.evalExpr(stmt.Value, env)
	if err != nil {
		return nil, err
	}

	return env.Set(stmt.Target.String(), value), nil
}

func (e *Evaluator) evalExpr(expr ast.Expression, env *Env) (registry.Value, error) {
	switch n := expr.(type) {
	case *ast.IntegerExpr:
		return registry.Integer(n.Value), nil
	case *ast.FloatExpr:
		return registry.Float(n.Value), nil
	case *ast.StringExpr:
		return registry.String(n.Value), nil
	case *ast.BooleanExpr:
		return registry.Bool(n.Value), nil
	case *ast.IdentExpr:
		return e.evalIdent(n, env)
	case *ast.InfixExpr:
		return e.evalInfix(n, env)
	case *ast.PrefixExpr:
		return e.evalPrefix(n, env)
	default:
		return nil, fmt.Errorf("not implemented expression %T", expr)
	}
}

func (e *Evaluator) evalIdent(expr *ast.IdentExpr, env *Env) (registry.Value, error) {
	value, ok := env.Get(expr.String())
	if !ok {
		return nil, fmt.Errorf("unknown identifier %q", expr.String())
	}
	return value, nil
}

func (e *Evaluator) evalInfix(expr *ast.InfixExpr, env *Env) (registry.Value, error) {
	left, err := e.evalExpr(expr.Left, env)
	if err != nil {
		return nil, err
	}

	right, err := e.evalExpr(expr.Right, env)
	if err != nil {
		return nil, err
	}

	rule, ok := e.registry.LookupBinary(expr.Token.Type, left.TypeID(), right.TypeID())
	if !ok {
		return nil, fmt.Errorf("%s can't be %s with %s", left.TypeID(), expr.Token.Type, right.TypeID())
	}
	return rule.EvalFn(expr.Token.Type, left, right), nil
}

func (e *Evaluator) evalPrefix(expr *ast.PrefixExpr, env *Env) (registry.Value, error) {
	right, err := e.evalExpr(expr.Right, env)
	if err != nil {
		return nil, err
	}

	rule, ok := e.registry.LookupUnary(expr.Token.Type, right.TypeID())
	if !ok {
		return nil, fmt.Errorf("%s can't be %s", expr.Token.Type, right.TypeID())
	}
	return rule.EvalFn(expr.Token.Type, right), nil
}
