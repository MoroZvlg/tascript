package resolver

import (
	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/resolved"
	"github.com/MoroZvlg/tascript/token"
)

type Symbol string

// Resolver doing both resolve and type check
type Resolver struct {
	prog         *ast.Program
	resolvedProg *resolved.Program
	reg          *registry.Registry
	errs         []diag.Diagnostic
}

func New(prog *ast.Program, reg *registry.Registry) *Resolver {
	return &Resolver{
		prog:         prog,
		resolvedProg: &resolved.Program{},
		reg:          reg,
	}
}

func (r *Resolver) Diagnostics() []diag.Diagnostic {
	return r.errs
}

func (r *Resolver) Resolve() (*resolved.Program, bool) {
	if !r.prog.Valid {
		return r.resolvedProg, false
	}

	topLevelEnv := EnvFromRegistry(r.reg)

	r.resolvedProg.Consts = r.resolveConst(r.prog.Consts, topLevelEnv)

	r.resolvedProg.RunFn = r.resolveFunc(r.prog.RunFn, topLevelEnv)
	if r.prog.InitFn != nil {
		r.resolvedProg.InitFn = r.resolveFunc(r.prog.InitFn, topLevelEnv)
	}

	if len(r.errs) > 0 {
		return r.resolvedProg, false
	}
	return r.resolvedProg, true
}

func (r *Resolver) resolveFunc(astFunc *ast.FunctionDecl, env *Env) *resolved.Function {
	resolvedFunc := &resolved.Function{Token: astFunc.Token}
	resolvedStmts := make([]resolved.Statement, 0)
	for _, stmt := range astFunc.Body.Stmts {
		resolvedStmts = append(resolvedStmts, r.resolveStmt(stmt, env))
	}
	resolvedFunc.Body = &resolved.BlockStmt{
		Token: astFunc.Body.Token,
		Stmts: resolvedStmts,
	}
	return resolvedFunc
}

func (r *Resolver) resolveStmt(astStmt ast.Statement, env *Env) resolved.Statement {
	switch astStmtTyped := astStmt.(type) {
	case *ast.ExprStmt:
		return &resolved.ExprStmt{
			Token: astStmtTyped.Token,
			Expr:  r.resolveExpr(astStmtTyped.Expr, env),
		}
	default:
		return nil // TODO: raise error?
	}
}

func (r *Resolver) resolveConst(consts []*ast.ConstDecl, env *Env) []*resolved.ConstDecl {
	resolvedConsts := make([]*resolved.ConstDecl, 0)
	for _, c := range consts {
		sym := Symbol(c.Identifier.String())
		if _, exists := env.values[sym]; exists {
			r.addDuplicateDeclaration(c.Token, c.Identifier.Token) // How to add existing
			return resolvedConsts
		}
		constValue := r.resolveExpr(c.Value, env)
		env.values[sym] = constValue.Type()
		resolvedConsts = append(resolvedConsts, &resolved.ConstDecl{
			Token: c.Token,
			Name:  &resolved.IdentExpr{Token: c.Identifier.Token, T: constValue.Type()},
			Value: constValue,
		})
	}
	return resolvedConsts
}

func (r *Resolver) resolveExpr(expr ast.Expression, env *Env) resolved.Expression {
	switch typedExpr := expr.(type) {
	case *ast.IntegerExpr:
		return &resolved.IntegerExpr{Token: typedExpr.Token, Value: typedExpr.Value, T: registry.IntegerID}
	case *ast.FloatExpr:
		return &resolved.FloatExpr{Token: typedExpr.Token, Value: typedExpr.Value, T: registry.FloatID}
	case *ast.StringExpr:
		return &resolved.StringExpr{Token: typedExpr.Token, Value: typedExpr.Value, T: registry.StringID}
	case *ast.IdentExpr:
		t, exists := env.values[Symbol(typedExpr.String())]
		if !exists {
			r.addUndefinedIdent(typedExpr.Token)
			return &resolved.BadExpr{Token: typedExpr.Token}
		}
		return &resolved.IdentExpr{Token: typedExpr.Token, T: t}
	case *ast.InfixExpr:
		errsBefore := len(r.errs)
		left := r.resolveExpr(typedExpr.Left, env)
		right := r.resolveExpr(typedExpr.Right, env)
		binaryRule, exists := r.reg.LookupBinary(typedExpr.Token.Type, left.Type(), right.Type())
		if !exists {
			if len(r.errs) == errsBefore {
				r.addInvalidBinaryOp(typedExpr.Token, left.Type(), right.Type())
			}
			return &resolved.BadExpr{Token: typedExpr.Token}
		}
		return &resolved.InfixExpr{Token: typedExpr.Token, Left: left, Right: right, T: binaryRule.EvalType, EvalFn: binaryRule.EvalFn}
	case *ast.PrefixExpr:
		errsBefore := len(r.errs)
		right := r.resolveExpr(typedExpr.Right, env)
		unaryRule, exists := r.reg.LookupUnary(typedExpr.Token.Type, right.Type())
		if !exists {
			if len(r.errs) == errsBefore {
				r.addInvalidUnaryOp(typedExpr.Token, right.Type())
			}
			return &resolved.BadExpr{Token: typedExpr.Token}
		}
		return &resolved.PrefixExpr{Token: typedExpr.Token, Right: right, T: unaryRule.EvalType, EvalFn: unaryRule.EvalFn}

	//case *ast.MemberAccessExpr:
	//	errsBefore := len(r.errs)
	//	objectID := r.resolveExpr(typedExpr.Object, env)
	//	rule, exists := r.reg.LookupMemberAccess(objectID, typedExpr.Method.String())
	//	if !exists {
	//		if len(r.errs) == errsBefore {
	//			r.addUndefinedAttribute(typedExpr.Method.Token)
	//		}
	//		return registry.UnknownTypeID
	//	}
	//	return rule.EvalType
	//case *ast.CallExpr:
	//	switch callee := typedExpr.Callee.(type) {
	//	case *ast.MemberAccessExpr:
	//		errsBefore := len(r.errs)
	//		objectID := r.resolveExpr(callee.Object, env)
	//
	//		rule, callExists := r.reg.LookupCall(objectID, callee.Method.String())
	//		if !callExists {
	//			if len(r.errs) == errsBefore {
	//				r.addUndefinedAttribute(callee.Method.Token)
	//			}
	//			return registry.UnknownTypeID
	//		}
	//
	//		argsValid := r.checkAllArgs(typedExpr.Token, typedExpr.Args, typedExpr.Kwargs, rule, env)
	//		if argsValid {
	//			return rule.EvalType
	//		}
	//		return registry.UnknownTypeID
	//	default:
	//		return registry.UnknownTypeID
	//	}

	default:
		return &resolved.BadExpr{} // unreachable. we know all ast types. otherwise error on prev phase
	}
}

//func (r *Resolver) checkAllArgs(token token.Token, args []ast.Expression, kwargs []*ast.KwargsExpr, rule registry.CallRule, env *Env) bool {
//	if len(args)+len(kwargs) != len(rule.Args) {
//		r.addArgsNumberMismatch(token, len(rule.Args), len(args)+len(kwargs))
//		return false
//	}
//	argSlots := make(map[string]registry.TypeID)
//
//	hasErrs := false
//
//	for i, arg := range args {
//		argType := r.resolveExpr(arg, env)
//		paramRule := rule.Args[i]
//
//		ok := argType == paramRule.Type
//		if !ok && !paramRule.Exact {
//			_, ok = r.reg.LookupCoerce(argType, paramRule.Type)
//		}
//
//		if !ok {
//			hasErrs = true
//			r.addArgTypeMissmatch(token, paramRule.Type, argType)
//		}
//
//		argSlots[paramRule.Name] = argType
//	}
//
//	for _, kwExpr := range kwargs {
//		argType := r.resolveExpr(kwExpr.Value, env)
//		argName := kwExpr.Key.String()
//		var paramRule *registry.ParamRule
//
//		for _, argRule := range rule.Args {
//			if argRule.Name == argName {
//				paramRule = &argRule
//				break
//			}
//		}
//
//		if paramRule == nil {
//			continue
//		}
//
//		ok := argType == paramRule.Type
//		if !ok && !paramRule.Exact {
//			_, ok = r.reg.LookupCoerce(argType, paramRule.Type)
//		}
//
//		if !ok {
//			hasErrs = true
//			r.addArgTypeMissmatch(token, paramRule.Type, argType)
//		}
//		argSlots[argName] = argType
//		break
//	}
//
//	for _, argRule := range rule.Args {
//		_, exists := argSlots[argRule.Name]
//		if !exists && argRule.Exact {
//			hasErrs = true
//			r.addArgsMissingKWArg(token, argRule.Name)
//		}
//	}
//
//	return !hasErrs
//}

func (r *Resolver) addDuplicateDeclaration(kwToken, identToken token.Token) {
	r.errs = append(r.errs, &diag.DuplicateDeclaration{
		Phase:        diag.PhaseCheck,
		KeywordToken: kwToken,
		IdentToken:   identToken,
	})
}

func (r *Resolver) addInvalidBinaryOp(cmpToken token.Token, left, right registry.TypeID) {
	r.errs = append(r.errs, &diag.InvalidBinaryOperation{
		Phase: diag.PhaseCheck,
		Token: cmpToken,
		Left:  left,
		Right: right,
	})
}

func (r *Resolver) addInvalidUnaryOp(opToken token.Token, right registry.TypeID) {
	r.errs = append(r.errs, &diag.UnaryBinaryOperation{
		Phase: diag.PhaseCheck,
		Token: opToken,
		Right: right,
	})
}

func (r *Resolver) addUndefinedIdent(opToken token.Token) {
	r.errs = append(r.errs, &diag.UndefinedIdent{
		Phase: diag.PhaseCheck,
		Token: opToken,
	})
}

func (r *Resolver) addUndefinedAttribute(opToken token.Token) {
	r.errs = append(r.errs, &diag.UndefinedAttribute{
		Phase:  diag.PhaseCheck,
		Member: opToken,
	})
}

func (r *Resolver) addUndefinedMethod(opToken token.Token) {
	r.errs = append(r.errs, &diag.UndefinedMethod{
		Phase:  diag.PhaseCheck,
		Method: opToken,
	})
}

func (r *Resolver) addArgsNumberMismatch(opToken token.Token, expected, got int) {
	r.errs = append(r.errs, &diag.ArgsNumberMissmatch{
		Phase:    diag.PhaseCheck,
		Token:    opToken,
		Expected: expected,
		Got:      got,
	})
}

func (r *Resolver) addArgsMissingKWArg(opToken token.Token, expected string) {
	r.errs = append(r.errs, &diag.MissingKWARG{
		Phase:    diag.PhaseCheck,
		Token:    opToken,
		Expected: expected,
	})
}

func (r *Resolver) addArgTypeMissmatch(opToken token.Token, expected, got registry.TypeID) {
	r.errs = append(r.errs, &diag.ArgTypeMissmatch{
		Phase:    diag.PhaseCheck,
		Token:    opToken,
		Expected: expected,
		Got:      got,
	})
}
