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
	resolvedProg := &resolved.Program{
		// NOTE: we need empty
		State: &resolved.State{
			Fields: make([]*resolved.StateField, 0),
		},
	}
	return &Resolver{
		prog:         prog,
		resolvedProg: resolvedProg,
		reg:          reg,
	}
}

func (r *Resolver) Diagnostics() []diag.Diagnostic {
	return r.errs
}

func (r *Resolver) Resolve() *resolved.Program {
	topLevelEnv := EnvFromRegistry(r.reg)

	r.resolvedProg.Consts = r.resolveConst(r.prog.Consts, topLevelEnv)
	r.resolveState(r.prog.StateFields, topLevelEnv)

	// NOTE: decl resolve order IS the scoping rule: consts and state initializers resolve before inputs/outputs
	// enter the env, so referencing dynamic data in them will fail.
	r.resolvedProg.Inputs = r.resolveInputs(r.prog.Inputs, topLevelEnv)
	r.resolvedProg.Outputs = r.resolveOutputs(r.prog.Outputs, topLevelEnv)

	if r.prog.InitFn != nil {
		r.resolvedProg.InitFn = r.resolveFunc(r.prog.InitFn, NewEnclosedEnv(topLevelEnv))
	}
	r.checkStateInitialized()

	r.resolvedProg.RunFn = r.resolveFunc(r.prog.RunFn, NewEnclosedEnv(topLevelEnv))

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
		if call, ok := astStmtTyped.Expr.(*ast.CallExpr); ok && isEmitCall(call) {
			return r.resolveEmit(call, env)
		}
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
				coerceRule, coerceExists := r.reg.LookupCoerce(value.Type(), binding.T)
				if !coerceExists {
					r.addTypeMissmatch(astStmtTyped.Token, binding.T, value.Type())
					return &resolved.BadStmt{Token: astStmtTyped.Token}
				}
				value = &resolved.CoerceExpr{Inner: value, T: coerceRule.EvalType, EvalFn: coerceRule.EvalFn}
			}

			return &resolved.AssignNameStmt{
				Token:  astStmtTyped.Token,
				Target: target.String(),
				Value:  value,
				T:      binding.T,
			}
		case *ast.MemberAccessExpr:
			identExpr, ok := target.Object.(*ast.IdentExpr)
			if !ok {
				r.addInvalidAssignTarget(astStmtTyped.Token)
				return &resolved.BadStmt{Token: astStmtTyped.Token}
			}

			if identExpr.Token.Type != token.STATE {
				r.addInvalidAssignTarget(astStmtTyped.Token)
				return &resolved.BadStmt{Token: astStmtTyped.Token}
			}

			fieldName := target.Method.String()
			var fieldDecl *resolved.StateField
			for _, field := range r.resolvedProg.State.Fields {
				if field.Name == fieldName {
					fieldDecl = field
					break
				}
			}

			if fieldDecl == nil {
				r.addStateUndeclared(target.Method.Token)
				return &resolved.BadStmt{Token: astStmtTyped.Token}
			}

			if fieldDecl.T != value.Type() {
				coerceRule, coerceExists := r.reg.LookupCoerce(value.Type(), fieldDecl.T)
				if !coerceExists {
					r.addTypeMissmatch(astStmtTyped.Token, fieldDecl.T, value.Type())
					return &resolved.BadStmt{Token: astStmtTyped.Token}
				}
				value = &resolved.CoerceExpr{Inner: value, T: coerceRule.EvalType, EvalFn: coerceRule.EvalFn}
			}

			return &resolved.AssignStateStmt{
				Token:  astStmtTyped.Token,
				Target: fieldDecl,
				Value:  value,
				T:      fieldDecl.T,
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

func isEmitCall(call *ast.CallExpr) bool {
	callee, ok := call.Callee.(*ast.IdentExpr)
	return ok && callee.String() == "emit"
}

func (r *Resolver) resolveEmit(call *ast.CallExpr, env *Env) resolved.Statement {
	if len(call.Args) == 0 {
		r.addInvalidEmitTarget(call.Token)
		return &resolved.BadStmt{Token: call.Token}
	}
	target, ok := call.Args[0].(*ast.IdentExpr)
	if !ok {
		r.addInvalidEmitTarget(call.Token)
		return &resolved.BadStmt{Token: call.Token}
	}
	binding, exists := env.Get(Symbol(target.String()))
	if !exists {
		r.addUndefinedIdent(target.Token)
		return &resolved.BadStmt{Token: target.Token}
	}
	if binding.Kind != KindOutput {
		r.addInvalidEmitTarget(target.Token)
		return &resolved.BadStmt{Token: target.Token}
	}

	// TODO: resolveArgs wants the arg's own token; ast.Expression exposes no Tok(), so arg-level
	// diagnostics point at `(` until it does
	args, valid := r.resolveArgs(call.Token, call.Args[1:], call.Kwargs, r.reg.EmitRule(binding.T), env)
	if !valid {
		return &resolved.BadStmt{Token: call.Token}
	}
	return &resolved.EmitStmt{
		Token:  call.Token,
		Output: target.String(),
		Args:   args,
		T:      binding.T,
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

func (r *Resolver) resolveLogical(expr *ast.InfixExpr, left, right resolved.Expression) resolved.Expression {
	for _, operand := range []resolved.Expression{left, right} {
		if !resolved.IsBadExpr(operand) && operand.Type() != registry.BoolID {
			r.addTypeMissmatch(expr.Token, registry.BoolID, operand.Type())
		}
	}
	return &resolved.LogicalExpr{Token: expr.Token, Left: left, Right: right}
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
			continue
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

func (r *Resolver) resolveState(fieldEntries []*ast.StateFieldDecl, env *Env) {
	fields := make([]*resolved.StateField, 0, len(fieldEntries))
	seen := make(map[string]bool, len(fieldEntries))
	for _, fieldEntry := range fieldEntries {
		name := fieldEntry.Identifier.String()
		if seen[name] {
			r.addDuplicateDeclaration(fieldEntry.Token, fieldEntry.Identifier.Token)
			continue
		}
		seen[name] = true

		typeID, exists := r.reg.LookupType(fieldEntry.Type.String())
		if !exists {
			typeToken := fieldEntry.Token
			if typeIdent, ok := fieldEntry.Type.(*ast.IdentExpr); ok {
				typeToken = typeIdent.Token
			}
			r.addUndefinedType(typeToken)
			continue
		}
		field := &resolved.StateField{
			Token: fieldEntry.Identifier.Token,
			Name:  name,
			T:     typeID,
		}
		if fieldEntry.Value != nil {
			initValue := r.resolveExpr(fieldEntry.Value, env)
			if resolved.IsBadExpr(initValue) {
				continue // already reported
			}
			if initValue.Type() != typeID {
				coerceRule, coerceExists := r.reg.LookupCoerce(initValue.Type(), typeID)
				if !coerceExists {
					r.addTypeMissmatch(fieldEntry.Identifier.Token, typeID, initValue.Type())
					continue
				}
				initValue = &resolved.CoerceExpr{Inner: initValue, T: coerceRule.EvalType, EvalFn: coerceRule.EvalFn}
			}
			field.InitValue = initValue
		}
		// NOTE: do not add fields to State here. In such case it next state decl may reference prev state decl
		fields = append(fields, field)
	}
	r.resolvedProg.State.Fields = fields
}

// checkStateInitialized: every state field MUST have a declaration initializer
// or be definitely assigned in Init() in every execution path.
func (r *Resolver) checkStateInitialized() {
	assigned := make(map[*resolved.StateField]bool)
	if r.resolvedProg.InitFn != nil {
		assigned = definitelyAssignedState(r.resolvedProg.InitFn.Body)
	}
	for _, field := range r.resolvedProg.State.Fields {
		if field.InitValue == nil && !assigned[field] {
			r.addStateUninitialized(field.Token)
		}
	}
}

// definitelyAssignedState returns the state fields assigned on every execution path through stmt.
// Deliberately conservative: a statement kind without a rule here(including any future stmts) — contributes nothing.
func definitelyAssignedState(stmt resolved.Statement) map[*resolved.StateField]bool {
	assigned := make(map[*resolved.StateField]bool)
	switch typedStmt := stmt.(type) {
	case *resolved.AssignStateStmt:
		assigned[typedStmt.Target] = true
	case *resolved.BlockStmt:
		for _, inner := range typedStmt.Stmts {
			for field, _ := range definitelyAssignedState(inner) {
				assigned[field] = true
			}
		}
	case *resolved.IfStmt:
		if typedStmt.Else == nil {
			break // no else: the branch may not run, nothing is definite
		}
		inElse := definitelyAssignedState(typedStmt.Else)                      // else branch
		for field, _ := range definitelyAssignedState(typedStmt.Consequence) { // if branch
			if _, exists := inElse[field]; exists {
				assigned[field] = true
			}
		}
	}
	return assigned
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
		if binding.Kind == KindOutput {
			r.addOutputNotReadable(typedExpr.Token)
			return &resolved.BadExpr{Token: typedExpr.Token}
		}
		return &resolved.IdentExpr{Token: typedExpr.Token, T: binding.T}
	case *ast.InfixExpr:
		errsBefore := len(r.errs)
		left := r.resolveExpr(typedExpr.Left, env)
		right := r.resolveExpr(typedExpr.Right, env)
		if typedExpr.Token.Type == token.AND || typedExpr.Token.Type == token.OR {
			return r.resolveLogical(typedExpr, left, right)
		}
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
		identExpr, isIdent := typedExpr.Object.(*ast.IdentExpr)
		if isIdent && identExpr.Token.Type == token.STATE {
			fieldName := typedExpr.Method.String()
			var fieldDecl *resolved.StateField

			for _, field := range r.resolvedProg.State.Fields {
				if field.Name == fieldName {
					fieldDecl = field
					break
				}
			}

			if fieldDecl == nil {
				r.addStateUndeclared(typedExpr.Method.Token)
				return &resolved.BadExpr{Token: typedExpr.Token}
			}

			return &resolved.StateAccessExpr{
				Token:  typedExpr.Token,
				Method: fieldName,
				T:      fieldDecl.T,
			}
		}

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
	case *ast.IndexExpr:
		errsBefore := len(r.errs)
		left := r.resolveExpr(typedExpr.Left, env)
		r.resolveExpr(typedExpr.Index, env)
		if len(r.errs) == errsBefore {
			r.addNotIndexable(typedExpr.Token, left.Type())
		}
		return &resolved.BadExpr{Token: typedExpr.Token}
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

			// TODO: resolveArgs wants the arg's own token; ast.Expression exposes no Tok(), so
			// arg-level diagnostics point at `(` until it does
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
		case *ast.IdentExpr:
			if isEmitCall(typedExpr) {
				// if we got here - emit was used inside Expression. otherwise it will be parsed by separate case branch
				r.addEmitNotExpression(callee.Token)
			} else {
				r.addUndefinedFunc(callee.Token)
			}
			return &resolved.BadExpr{Token: typedExpr.Token}
		default:
			r.addNotCallable(typedExpr.Token)
			return &resolved.BadExpr{Token: typedExpr.Token}
		}

	default:
		return &resolved.BadExpr{} // unreachable. we know all ast types. otherwise error on prev phase
	}
}

func (r *Resolver) resolveArgs(argToken token.Token, args []ast.Expression, kwargs []*ast.KwargsExpr, rule registry.CallRule, env *Env) ([]*resolved.CallArgExpr, bool) {
	if len(args)+len(kwargs) != len(rule.Args) {
		r.addArgsNumberMismatch(argToken, len(rule.Args), len(args)+len(kwargs))
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
			r.addTypeMissmatch(argToken, argRule.Type, resolvedArg.Type())
		}

		resolvedArgs[i] = &resolved.CallArgExpr{
			Token: argToken,
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
			r.addUnknownKwarg(kwArg.Token, kwArg.Key.String())
			hasErrs = true
			continue
		}

		if resolvedIdx[argRuleIdx] {
			r.addDuplicateArg(kwArg.Token, argRule.Name)
			hasErrs = true
			continue
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
			r.addTypeMissmatch(argToken, argRule.Type, resolvedArg.Type())
		}

		resolvedArgs[argRuleIdx] = &resolved.CallArgExpr{
			Token: argToken,
			Name:  argRule.Name,
			Value: resolvedArg,
			T:     argRule.Type,
		}
	}

	for i, isResolved := range resolvedIdx {
		if !isResolved {
			unresolvedRule := rule.Args[i]
			r.addArgsMissing(argToken, unresolvedRule.Name)
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

func (r *Resolver) addOutputNotReadable(opToken token.Token) {
	r.errs = append(r.errs, &diag.OutputNotReadable{
		Phase: diag.PhaseCheck,
		Token: opToken,
	})
}

func (r *Resolver) addInvalidEmitTarget(opToken token.Token) {
	r.errs = append(r.errs, &diag.InvalidEmitTarget{
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

func (r *Resolver) addNotIndexable(bracketToken token.Token, left registry.TypeID) {
	r.errs = append(r.errs, &diag.NotIndexable{
		Phase: diag.PhaseCheck,
		Pos:   bracketToken.Pos,
		Left:  left,
	})
}

func (r *Resolver) addUndefinedFunc(calleeToken token.Token) {
	r.errs = append(r.errs, &diag.UndefinedFunc{
		Phase: diag.PhaseCheck,
		Token: calleeToken,
	})
}

func (r *Resolver) addNotCallable(callToken token.Token) {
	r.errs = append(r.errs, &diag.NotCallable{
		Phase: diag.PhaseCheck,
		Pos:   callToken.Pos,
	})
}

func (r *Resolver) addEmitNotExpression(calleeToken token.Token) {
	r.errs = append(r.errs, &diag.EmitNotExpression{
		Phase: diag.PhaseCheck,
		Token: calleeToken,
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

func (r *Resolver) addDuplicateArg(kwToken token.Token, name string) {
	r.errs = append(r.errs, &diag.DuplicateArg{
		Phase: diag.PhaseCheck,
		Token: kwToken,
		Name:  name,
	})
}

func (r *Resolver) addUnknownKwarg(kwToken token.Token, name string) {
	r.errs = append(r.errs, &diag.UnknownKwarg{
		Phase: diag.PhaseCheck,
		Token: kwToken,
		Name:  name,
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

func (r *Resolver) addStateUndeclared(fieldToken token.Token) {
	r.errs = append(r.errs, &diag.StateUndeclared{
		Phase: diag.PhaseCheck,
		Token: fieldToken,
	})
}

func (r *Resolver) addStateUninitialized(fieldToken token.Token) {
	r.errs = append(r.errs, &diag.StateUninitialized{
		Phase: diag.PhaseCheck,
		Token: fieldToken,
	})
}
