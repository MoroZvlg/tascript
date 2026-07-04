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

func (r *Resolver) Resolve() *resolved.Program {
	topLevelEnv := EnvFromRegistry(r.reg)

	r.resolvedProg.Consts = r.resolveConst(r.prog.Consts, topLevelEnv)
	r.resolvedProg.Inputs = r.resolveInputs(r.prog.Inputs, topLevelEnv)
	r.resolvedProg.Outputs = r.resolveOutputs(r.prog.Outputs, topLevelEnv)

	r.resolvedProg.RunFn = r.resolveFunc(r.prog.RunFn, NewEnclosedEnv(topLevelEnv))

	if r.prog.InitFn != nil {
		r.resolvedProg.InitFn = r.resolveFunc(r.prog.InitFn, NewEnclosedEnv(topLevelEnv))
	}

	if len(r.errs) > 0 {
		return r.resolvedProg
	}
	return r.resolvedProg
}

func (r *Resolver) resolveFunc(astFunc *ast.FunctionDecl, env *Env) *resolved.Function {
	resolvedFunc := &resolved.Function{Token: astFunc.Token}
	resolvedFunc.Body = r.resolveBlock(astFunc.Body, env)
	return resolvedFunc
}

func (r *Resolver) resolveBlock(astBlock *ast.BlockStmt, env *Env) *resolved.BlockStmt {
	resolvedStmts := make([]resolved.Statement, 0, len(astBlock.Stmts))
	for _, stmt := range astBlock.Stmts {
		resolvedStmts = append(resolvedStmts, r.resolveStmt(stmt, env))
	}
	return &resolved.BlockStmt{
		Token: astBlock.Token,
		Stmts: resolvedStmts,
	}
}

func (r *Resolver) resolveStmt(astStmt ast.Statement, env *Env) resolved.Statement {
	switch astStmtTyped := astStmt.(type) {
	case *ast.ExprStmt:
		return &resolved.ExprStmt{
			Token: astStmtTyped.Token,
			Expr:  r.resolveExpr(astStmtTyped.Expr, env),
		}
	case *ast.LetStmt:
		name := astStmtTyped.Name.String()
		exprVal := r.resolveExpr(astStmtTyped.Value, env)
		env.Set(Symbol(name), Binding{T: exprVal.Type(), Kind: KindLet})
		return &resolved.LetStmt{
			Token: astStmtTyped.Token,
			Name:  name,
			Value: exprVal,
			T:     exprVal.Type(),
		}
	case *ast.AssignStmt:
		value := r.resolveExpr(astStmtTyped.Value, env)
		switch target := astStmtTyped.Target.(type) {
		case *ast.IdentExpr:
			binding, exists := env.Get(Symbol(target.String()))
			if !exists {
				r.addUndefinedVar(target.Token)
				return &resolved.BadStmt{Token: target.Token}
			}
			if !binding.Assignable() {
				r.addNotAssignable(target.Token, binding.Kind)
				return &resolved.BadStmt{Token: target.Token}
			}
			if resolved.IsBadExpr(value) {
				return &resolved.BadStmt{Token: astStmtTyped.Token}
			}
			if binding.T != value.Type() {
				r.addTypeMissmatch(astStmtTyped.Token, binding.T, value.Type())
				return &resolved.BadStmt{Token: astStmtTyped.Token}
			}

			return &resolved.AssignNameStmt{
				Token:  astStmtTyped.Token,
				Target: target.String(),
				Value:  value,
				T:      binding.T,
			}
		default:
			r.addInvalidAssignTarget(astStmtTyped.Token)
			return &resolved.BadStmt{Token: astStmtTyped.Token}
		}
	case *ast.BlockStmt:
		return r.resolveBlock(astStmtTyped, NewEnclosedEnv(env))
	case *ast.IfStmt:
		return r.resolveIfStmt(astStmtTyped, env)
	case *ast.BadStmt:
		// parser already reported it
		return &resolved.BadStmt{Token: token.Token{Pos: astStmtTyped.From}}
	default:
		return &resolved.BadStmt{} // unreachable: all stmt types are handled above
	}
}

func (r *Resolver) resolveIfStmt(astStmt *ast.IfStmt, env *Env) resolved.Statement {
	condition := r.resolveExpr(astStmt.Condition, env)
	if !resolved.IsBadExpr(condition) && condition.Type() != registry.BoolID {
		// TODO: point at the condition, not the `if` keyword. Needs Tok() on ast.Expression.
		r.addTypeMissmatch(astStmt.Token, registry.BoolID, condition.Type())
	}

	consequence := r.resolveBlock(astStmt.Consequence, NewEnclosedEnv(env))

	var elseStmt resolved.Statement
	if astStmt.Else != nil {
		elseStmt = r.resolveStmt(astStmt.Else, env)
	}

	return &resolved.IfStmt{
		Token:       astStmt.Token,
		Condition:   condition,
		Consequence: consequence,
		Else:        elseStmt,
	}
}

func (r *Resolver) resolveInputs(inputs []*ast.InputDecl, env *Env) []*resolved.InputDecl {
	resolvedInputs := make([]*resolved.InputDecl, 0, len(inputs))
	for _, in := range inputs {
		sym := Symbol(in.Identifier.String())
		if _, exists := env.Get(sym); exists {
			r.addDuplicateDeclaration(in.Token, in.Identifier.Token)
			continue
		}
		typeID, ok := r.resolveTypeDecl(in.Type, "input", in.Identifier.String())
		if !ok {
			continue
		}
		env.Set(sym, Binding{T: typeID, Kind: KindInput})
		resolvedInputs = append(resolvedInputs, &resolved.InputDecl{
			Token: in.Token,
			Name:  in.Identifier.String(),
			T:     typeID,
		})
	}
	return resolvedInputs
}

func (r *Resolver) resolveOutputs(outputs []*ast.OutputDecl, env *Env) []*resolved.OutputDecl {
	resolvedOutputs := make([]*resolved.OutputDecl, 0, len(outputs))
	for _, out := range outputs {
		sym := Symbol(out.Identifier.String())
		if _, exists := env.Get(sym); exists {
			r.addDuplicateDeclaration(out.Token, out.Identifier.Token)
			continue
		}
		typeID, ok := r.resolveTypeDecl(out.Type, "output", out.Identifier.String())
		if !ok {
			continue
		}
		// TODO: outputs are emit-only by design, but reading one (`let x = alert`)
		// still resolves cleanly and dies at runtime. Add a KindOutput read check when emit lands.
		env.Set(sym, Binding{T: typeID, Kind: KindOutput})
		resolvedOutputs = append(resolvedOutputs, &resolved.OutputDecl{
			Token: out.Token,
			Name:  out.Identifier.String(),
			T:     typeID,
		})
	}
	return resolvedOutputs
}

func (r *Resolver) resolveTypeDecl(typeDecl ast.TypeDecl, namespace, declName string) (registry.TypeID, bool) {
	switch typed := typeDecl.(type) {
	case *ast.IdentExpr:
		typeID, exists := r.reg.LookupType(typed.String())
		if !exists {
			r.addUndefinedType(typed.Token)
			return registry.UnknownTypeID, false
		}
		return typeID, true
	case *ast.TypeExpr:
		fields := make([]registry.FieldDef, 0, len(typed.Fields))
		seen := make(map[string]bool)
		ok := true
		for _, field := range typed.Fields {
			if seen[field.Name.String()] {
				r.addDuplicateDeclaration(typed.Token, field.Name.Token)
				ok = false
				continue
			}
			seen[field.Name.String()] = true
			fieldType, exists := r.reg.LookupType(field.Type.String())
			if !exists {
				r.addUndefinedType(field.Type.Token)
				ok = false
				continue
			}
			fields = append(fields, registry.FieldDef{Name: field.Name.String(), Type: fieldType})
		}
		if !ok {
			return registry.UnknownTypeID, false
		}
		typeID, err := r.reg.RegisterScriptType(namespace+"."+declName, fields)
		if err != nil {
			// unreachable: duplicate decl names are caught by the env check first
			return registry.UnknownTypeID, false
		}
		return typeID, true
	default:
		return registry.UnknownTypeID, false
	}
}

func (r *Resolver) resolveConst(consts []*ast.ConstDecl, env *Env) []*resolved.ConstDecl {
	resolvedConsts := make([]*resolved.ConstDecl, 0)
	for _, c := range consts {
		sym := Symbol(c.Identifier.String())
		if _, exists := env.Get(sym); exists {
			r.addDuplicateDeclaration(c.Token, c.Identifier.Token) // How to add existing
			return resolvedConsts
		}
		constValue := r.resolveExpr(c.Value, env)
		env.Set(sym, Binding{T: constValue.Type(), Kind: KindConst})
		resolvedConsts = append(resolvedConsts, &resolved.ConstDecl{
			Token: c.Token,
			Name:  &resolved.IdentExpr{Token: c.Identifier.Token, T: constValue.Type()},
			Value: constValue,
			T:     constValue.Type(),
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
	case *ast.BooleanExpr:
		return &resolved.BooleanExpr{Token: typedExpr.Token, Value: typedExpr.Value, T: registry.BoolID}
	case *ast.IdentExpr:
		binding, exists := env.Get(Symbol(typedExpr.String()))
		if !exists {
			r.addUndefinedIdent(typedExpr.Token)
			return &resolved.BadExpr{Token: typedExpr.Token}
		}
		return &resolved.IdentExpr{Token: typedExpr.Token, T: binding.T}
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

	case *ast.MemberAccessExpr:
		errsBefore := len(r.errs)
		resolvedExpr := r.resolveExpr(typedExpr.Object, env)
		rule, exists := r.reg.LookupMemberAccess(resolvedExpr.Type(), typedExpr.Method.String())
		if !exists {
			if len(r.errs) == errsBefore {
				r.addUndefinedAttribute(typedExpr.Method.Token)
			}
			return &resolved.BadExpr{Token: typedExpr.Token}
		}
		return &resolved.MemberAccessExpr{
			Token:  typedExpr.Token,
			Object: resolvedExpr,
			Method: typedExpr.Method.String(),
			T:      rule.EvalType,
			EvalFn: rule.EvalFn,
		}
	case *ast.CallExpr:
		switch callee := typedExpr.Callee.(type) {
		case *ast.MemberAccessExpr:
			errsBefore := len(r.errs)
			resolvedExpr := r.resolveExpr(callee.Object, env)

			rule, callExists := r.reg.LookupCall(resolvedExpr.Type(), callee.Method.String())
			if !callExists {
				if len(r.errs) == errsBefore {
					r.addUndefinedMethod(callee.Method.Token)
				}
				return &resolved.BadExpr{Token: typedExpr.Token}
			}

			args, valid := r.resolveArgs(typedExpr.Token, typedExpr.Args, typedExpr.Kwargs, rule, env)
			if valid {
				return &resolved.MethodCallExpr{
					Token:    typedExpr.Token,
					Receiver: resolvedExpr,
					Method:   callee.Method.String(),
					Args:     args,
					T:        rule.EvalType,
					EvalFn:   rule.EvalFn,
				}
			}
			return &resolved.BadExpr{Token: typedExpr.Token}
		default:
			return &resolved.BadExpr{Token: typedExpr.Token}
		}

	default:
		return &resolved.BadExpr{} // unreachable. we know all ast types. otherwise error on prev phase
	}
}

func (r *Resolver) resolveArgs(token token.Token, args []ast.Expression, kwargs []*ast.KwargsExpr, rule registry.CallRule, env *Env) ([]*resolved.CallArgExpr, bool) {
	if len(args)+len(kwargs) != len(rule.Args) {
		r.addArgsNumberMismatch(token, len(rule.Args), len(args)+len(kwargs))
		return nil, false
	}
	resolvedIdx := make([]bool, len(rule.Args))
	resolvedArgs := make([]*resolved.CallArgExpr, len(args)+len(kwargs))
	hasErrs := false

	for i, arg := range args {
		argRule := rule.Args[i]
		resolvedIdx[i] = true
		resolvedArg := r.resolveExpr(arg, env)
		ok := resolvedArg.Type() == argRule.Type
		if !ok && !argRule.Exact {
			if coerceRule, canCoerce := r.reg.LookupCoerce(resolvedArg.Type(), argRule.Type); canCoerce {
				resolvedArg = &resolved.CoerceExpr{Inner: resolvedArg, T: coerceRule.EvalType, EvalFn: coerceRule.EvalFn}
				ok = true
			}
		}

		if !ok {
			hasErrs = true
			r.addTypeMissmatch(token, argRule.Type, resolvedArg.Type())
		}

		resolvedArgs[i] = &resolved.CallArgExpr{
			Token: token, // TODO: we need arg token not an call token....
			Name:  argRule.Name,
			Value: resolvedArg,
			T:     argRule.Type,
		}
	}

	for _, kwArg := range kwargs {
		var argRule *registry.ParamRule
		var argRuleIdx int

		for i, ar := range rule.Args {
			if kwArg.Key.String() == ar.Name {
				argRule = &ar
				argRuleIdx = i
				break
			}
		}

		if argRule == nil {
			continue // NOTE: allows to pass extra KWargs. simply skips it
		}

		resolvedIdx[argRuleIdx] = true
		resolvedArg := r.resolveExpr(kwArg.Value, env)
		ok := resolvedArg.Type() == argRule.Type
		if !ok && !argRule.Exact {
			if coerceRule, canCoerce := r.reg.LookupCoerce(resolvedArg.Type(), argRule.Type); canCoerce {
				resolvedArg = &resolved.CoerceExpr{Inner: resolvedArg, T: coerceRule.EvalType, EvalFn: coerceRule.EvalFn}
				ok = true
			}
		}

		if !ok {
			hasErrs = true
			r.addTypeMissmatch(token, argRule.Type, resolvedArg.Type())
		}

		resolvedArgs[argRuleIdx] = &resolved.CallArgExpr{
			Token: token, // TODO: we need arg token not an call token....
			Name:  argRule.Name,
			Value: resolvedArg,
			T:     argRule.Type,
		}
	}

	for i, isResolved := range resolvedIdx {
		if !isResolved {
			unresolvedRule := rule.Args[i]
			r.addArgsMissing(token, unresolvedRule.Name)
			hasErrs = true
		}
	}

	return resolvedArgs, !hasErrs
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

func (r *Resolver) addUndefinedVar(opToken token.Token) {
	r.errs = append(r.errs, &diag.UndefinedVar{
		Phase: diag.PhaseCheck,
		Token: opToken,
	})
}

func (r *Resolver) addInvalidAssignTarget(opToken token.Token) {
	r.errs = append(r.errs, &diag.InvalidAssignTarget{
		Phase: diag.PhaseCheck,
		Token: opToken,
	})
}

func (r *Resolver) addNotAssignable(opToken token.Token, kind BindingKind) {
	r.errs = append(r.errs, &diag.NotAssignable{
		Phase: diag.PhaseCheck,
		Token: opToken,
		Kind:  string(kind),
	})
}

func (r *Resolver) addUndefinedType(opToken token.Token) {
	r.errs = append(r.errs, &diag.UndefinedType{
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

func (r *Resolver) addArgsMissing(opToken token.Token, expected string) {
	r.errs = append(r.errs, &diag.MissingArg{
		Phase:    diag.PhaseCheck,
		Token:    opToken,
		Expected: expected,
	})
}

func (r *Resolver) addTypeMissmatch(opToken token.Token, expected, got registry.TypeID) {
	r.errs = append(r.errs, &diag.TypeMissmatch{
		Phase:    diag.PhaseCheck,
		Token:    opToken,
		Expected: expected,
		Got:      got,
	})
}
