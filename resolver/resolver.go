package resolver

import (
	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/token"
)

type Symbol string

// Resolver doing both resolve and type check
type Resolver struct {
	prog *ast.Program
	reg  *registry.Registry
	errs []diag.Diagnostic
}

func New(prog *ast.Program, reg *registry.Registry) *Resolver {
	return &Resolver{
		prog: prog,
		reg:  reg,
	}
}

func (r *Resolver) Diagnostics() []diag.Diagnostic {
	return r.errs
}

func (r *Resolver) Resolve() bool {
	if !r.prog.Valid {
		return false
	}

	topLevelEnv := EnvFromRegistry(r.reg)

	r.resolveConst(r.prog.Consts, topLevelEnv)

	if len(r.errs) > 0 {
		return false
	}
	return true
}

func (r *Resolver) resolveConst(consts []*ast.ConstDecl, env *Env) {
	for _, c := range consts {
		sym := Symbol(c.Identifier.String())
		if _, exists := env.values[sym]; exists {
			r.addDuplicateDeclaration(c.Token, c.Identifier.Token) // How to add existing
			return
		}
		env.values[sym] = r.resolveExpr(c.Value, env)
	}
}

func (r *Resolver) resolveExpr(expr ast.Expression, env *Env) registry.TypeID {
	switch typedExpr := expr.(type) {
	case *ast.IntegerExpr:
		return registry.IntegerID
	case *ast.FloatExpr:
		return registry.FloatID
	case *ast.StringExpr:
		return registry.StringID
	case *ast.IdentExpr:
		value, exists := env.values[Symbol(typedExpr.String())]
		if !exists {
			r.addUndefinedIdent(typedExpr.Token)
			return registry.UnknownTypeID
		}
		return value
	case *ast.InfixExpr:
		errsBefore := len(r.errs)
		left := r.resolveExpr(typedExpr.Left, env)
		right := r.resolveExpr(typedExpr.Right, env)
		binaryRule, exists := r.reg.LookupBinary(typedExpr.Token.Type, left, right)
		if exists {
			return binaryRule.EvalType
		}
		if len(r.errs) == errsBefore {
			r.addInvalidBinaryOp(typedExpr.Token, left, right)
		}
		return registry.UnknownTypeID
	case *ast.PrefixExpr:
		errsBefore := len(r.errs)
		right := r.resolveExpr(typedExpr.Right, env)
		unaryRule, exists := r.reg.LookupUnary(typedExpr.Token.Type, right)
		if exists {
			return unaryRule.EvalType
		}
		if len(r.errs) == errsBefore {
			r.addInvalidUnaryOp(typedExpr.Token, right)
		}
		return registry.UnknownTypeID
	case *ast.MemberAccessExpr:
		errsBefore := len(r.errs)
		objectID := r.resolveExpr(typedExpr.Object, env)
		rule, exists := r.reg.LookupMemberAccess(objectID, typedExpr.Method.String())
		if !exists {
			if len(r.errs) == errsBefore {
				r.addUndefinedAttribute(typedExpr.Method.Token)
			}
			return registry.UnknownTypeID
		}
		return rule.EvalType
	case *ast.CallExpr:
		switch callee := typedExpr.Callee.(type) {
		case *ast.MemberAccessExpr:
			errsBefore := len(r.errs)
			objectID := r.resolveExpr(callee.Object, env)

			rule, callExists := r.reg.LookupCall(objectID, callee.Method.String())
			if !callExists {
				if len(r.errs) == errsBefore {
					r.addUndefinedAttribute(callee.Method.Token)
				}
				return registry.UnknownTypeID
			}

			argsValid := r.checkAllArgs(typedExpr.Token, typedExpr.Args, typedExpr.Kwargs, rule, env)
			if argsValid {
				return rule.EvalType
			}
			return registry.UnknownTypeID
		default:
			return registry.UnknownTypeID
		}

	default:
		return registry.UnknownTypeID // unreachable. we know all ast types. otherwise error on prev phase
	}
}

func (r *Resolver) checkAllArgs(token token.Token, args []ast.Expression, kwargs []*ast.KwargsExpr, rule registry.CallRule, env *Env) bool {
	if len(args) != len(rule.Args) {
		r.addArgsNumberMismatch(token, len(rule.Args), len(args))
		return false
	}

	hasErrs := false

	for i, arg := range args {
		argType := r.resolveExpr(arg, env)
		expectedType := rule.Args[i]
		if argType != expectedType {
			r.addArgTypeMissmatch(token, expectedType, argType)
			hasErrs = true
		}
	}

	kwArgs := make(map[string]ast.Expression)
	for _, kwExpr := range kwargs {
		kwArgs[kwExpr.Key.String()] = kwExpr.Value
	}

	for name, expectedType := range rule.KWArgs {
		kwArg, exists := kwArgs[name]
		if !exists {
			r.addArgsMissingKWArg(token, name)
			hasErrs = true
			continue
		}
		gotType := r.resolveExpr(kwArg, env)
		if expectedType != gotType {
			r.addArgTypeMissmatch(token, expectedType, gotType)
			hasErrs = true
		}
	}
	return !hasErrs
}

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
