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
		if exists {
			return rule.EvalType
		}
		if len(r.errs) == errsBefore {
			r.addUndefinedAttribute(typedExpr.Method.Token)
		}
		return registry.UnknownTypeID
	default:
		return registry.UnknownTypeID // unreachable. we know all ast types. otherwise error on prev phase
	}
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
