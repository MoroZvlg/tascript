package resolver

import (
	"fmt"

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
	currFn       string
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
		r.currFn = "Init"
		r.resolvedProg.InitFn = r.resolveFunc(r.prog.InitFn, NewEnclosedEnv(topLevelEnv))
	}
	r.checkStateInitialized()

	r.currFn = "Run"
	r.resolvedProg.RunFn = r.resolveFunc(r.prog.RunFn, NewEnclosedEnv(topLevelEnv))

	return r.resolvedProg
}

func (r *Resolver) resolveFunc(astFunc *ast.FunctionDecl, env *Env) *resolved.FunctionDecl {
	resolvedFunc := &resolved.FunctionDecl{Token: astFunc.Tok()}
	resolvedFunc.Body = r.resolveBlock(astFunc.Body, env)
	return resolvedFunc
}

func (r *Resolver) resolveBlock(astBlock *ast.BlockStmt, env *Env) *resolved.BlockStmt {
	resolvedStmts := make([]resolved.Statement, 0, len(astBlock.Stmts))
	for _, stmt := range astBlock.Stmts {
		resolvedStmts = append(resolvedStmts, r.resolveStmt(stmt, env))
	}
	return &resolved.BlockStmt{
		Token: astBlock.Tok(),
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
			Token: astStmtTyped.Tok(),
			Expr:  r.resolveExpr(astStmtTyped.Expr, env),
		}
	case *ast.LetStmt:
		name := astStmtTyped.Name.String()
		if binding, exists := env.Get(Symbol(name)); exists {
			if binding.Reserved() {
				r.addReservedName(astStmtTyped.Name.Tok(), binding.Kind)
			} else {
				r.addDuplicateDeclaration(astStmtTyped.Tok(), astStmtTyped.Name.Tok())
			}
			r.resolveExpr(astStmtTyped.Value, env)
			return &resolved.BadStmt{Token: astStmtTyped.Tok()}
		}
		exprVal := r.resolveExpr(astStmtTyped.Value, env)
		env.Set(Symbol(name), Binding{T: exprVal.Type(), Kind: KindLet})
		return &resolved.LetStmt{
			Token: astStmtTyped.Tok(),
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
				r.addUndefinedVar(target.Tok())
				return &resolved.BadStmt{Token: target.Tok()}
			}
			if !binding.Assignable() {
				r.addNotAssignable(target.Tok(), binding.Kind)
				return &resolved.BadStmt{Token: target.Tok()}
			}
			if isErrorType(value.Type(), binding.T) {
				return &resolved.BadStmt{Token: astStmtTyped.Tok()}
			}
			if binding.T != value.Type() {
				coerceRule, coerceExists := r.reg.LookupCoerce(value.Type(), binding.T)
				if !coerceExists {
					r.addTypeMismatch(astStmtTyped.Tok(), binding.T, value.Type())
					return &resolved.BadStmt{Token: astStmtTyped.Tok()}
				}
				value = &resolved.CoerceExpr{Inner: value, T: coerceRule.EvalType, EvalFn: coerceRule.EvalFn}
			}

			return &resolved.AssignNameStmt{
				Token:  astStmtTyped.Tok(),
				Target: target.String(),
				Value:  value,
				T:      binding.T,
			}
		case *ast.MemberAccessExpr:
			identExpr, ok := target.Object.(*ast.IdentExpr)
			if !ok {
				r.addInvalidAssignTarget(astStmtTyped.Tok())
				return &resolved.BadStmt{Token: astStmtTyped.Tok()}
			}

			if identExpr.Tok().Type != token.STATE {
				r.addInvalidAssignTarget(astStmtTyped.Tok())
				return &resolved.BadStmt{Token: astStmtTyped.Tok()}
			}

			fieldName := target.Member.String()
			var fieldDecl *resolved.StateField
			for _, field := range r.resolvedProg.State.Fields {
				if field.Name == fieldName {
					fieldDecl = field
					break
				}
			}

			if fieldDecl == nil {
				r.addStateUndeclared(target.Member.Tok())
				return &resolved.BadStmt{Token: astStmtTyped.Tok()}
			}

			if !isErrorType(value.Type(), fieldDecl.T) && fieldDecl.T != value.Type() {
				coerceRule, coerceExists := r.reg.LookupCoerce(value.Type(), fieldDecl.T)
				if !coerceExists {
					r.addTypeMismatch(astStmtTyped.Tok(), fieldDecl.T, value.Type())
					return &resolved.BadStmt{Token: astStmtTyped.Tok()}
				}
				value = &resolved.CoerceExpr{Inner: value, T: coerceRule.EvalType, EvalFn: coerceRule.EvalFn}
			}

			return &resolved.AssignStateStmt{
				Token:  astStmtTyped.Tok(),
				Target: fieldDecl,
				Value:  value,
				T:      fieldDecl.T,
			}

		default:
			r.addInvalidAssignTarget(astStmtTyped.Tok())
			return &resolved.BadStmt{Token: astStmtTyped.Tok()}
		}
	case *ast.BlockStmt:
		return r.resolveBlock(astStmtTyped, NewEnclosedEnv(env))
	case *ast.IfStmt:
		return r.resolveIfStmt(astStmtTyped, env)
	default:
		panic(fmt.Sprintf("unhandled ast statement %T: ast grew a node the resolver never wired up", astStmtTyped))
	}
}

func isEmitCall(call *ast.CallExpr) bool {
	callee, ok := call.Callee.(*ast.IdentExpr)
	return ok && callee.String() == "emit"
}

func (r *Resolver) resolveEmit(call *ast.CallExpr, env *Env) resolved.Statement {
	if len(call.Args) == 0 {
		r.addInvalidEmitTarget(call.Tok())
		return &resolved.BadStmt{Token: call.Tok()}
	}
	target, ok := call.Args[0].(*ast.IdentExpr)
	if !ok {
		r.addInvalidEmitTarget(call.Tok())
		return &resolved.BadStmt{Token: call.Tok()}
	}
	binding, exists := env.Get(Symbol(target.String()))
	if !exists {
		r.addUndefinedIdent(target.Tok())
		return &resolved.BadStmt{Token: target.Tok()}
	}
	if binding.Kind != KindOutput {
		r.addInvalidEmitTarget(target.Tok())
		return &resolved.BadStmt{Token: target.Tok()}
	}

	if r.currFn != "Run" {
		r.addEmitOutsideRun(call.Tok())
	}

	args, valid := r.resolveArgs(call.Tok(), call.Args[1:], call.Kwargs, r.reg.EmitRule(binding.T), env)
	if !valid {
		return &resolved.BadStmt{Token: call.Tok()}
	}
	return &resolved.EmitStmt{
		Token:  call.Tok(),
		Output: target.String(),
		Args:   args,
		T:      binding.T,
	}
}

func (r *Resolver) resolveIfStmt(astStmt *ast.IfStmt, env *Env) resolved.Statement {
	condition := r.resolveExpr(astStmt.Condition, env)
	if !isErrorType(condition.Type()) && condition.Type() != registry.BoolID {
		r.addTypeMismatch(astStmt.Condition.Tok(), registry.BoolID, condition.Type())
	}

	consequence := r.resolveBlock(astStmt.Consequence, NewEnclosedEnv(env))

	var elseStmt resolved.Statement
	if astStmt.Else != nil {
		elseStmt = r.resolveStmt(astStmt.Else, env)
	}

	return &resolved.IfStmt{
		Token:       astStmt.Tok(),
		Condition:   condition,
		Consequence: consequence,
		Else:        elseStmt,
	}
}

func (r *Resolver) resolveLogical(expr *ast.InfixExpr, left, right resolved.Expression) resolved.Expression {
	for _, operand := range []resolved.Expression{left, right} {
		if !isErrorType(operand.Type()) && operand.Type() != registry.BoolID {
			r.addTypeMismatch(expr.Tok(), registry.BoolID, operand.Type())
		}
	}
	return &resolved.LogicalExpr{Token: expr.Tok(), Left: left, Right: right}
}

func (r *Resolver) resolveInputs(inputs []*ast.InputDecl, env *Env) []*resolved.InputDecl {
	resolvedInputs := make([]*resolved.InputDecl, 0, len(inputs))
	for _, in := range inputs {
		sym := Symbol(in.Identifier.String())
		if binding, exists := env.Get(sym); exists {
			if binding.Reserved() {
				r.addReservedName(in.Identifier.Tok(), binding.Kind)
			} else {
				r.addDuplicateDeclaration(in.Tok(), in.Identifier.Tok())
			}

			continue
		}
		typeID, _ := r.resolveTypeDecl(in.Type, "input", in.Identifier.String())
		env.Set(sym, Binding{T: typeID, Kind: KindInput})
		resolvedInputs = append(resolvedInputs, &resolved.InputDecl{
			Token: in.Tok(),
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
		if binding, exists := env.Get(sym); exists {
			if binding.Reserved() {
				r.addReservedName(out.Identifier.Tok(), binding.Kind)
			} else {
				r.addDuplicateDeclaration(out.Tok(), out.Identifier.Tok())
			}

			continue
		}
		typeID, _ := r.resolveTypeDecl(out.Type, "output", out.Identifier.String())
		env.Set(sym, Binding{T: typeID, Kind: KindOutput})
		resolvedOutputs = append(resolvedOutputs, &resolved.OutputDecl{
			Token: out.Tok(),
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
			r.addUndefinedType(typed.Tok())
			return registry.ErrorTypeID, false
		}
		return typeID, true
	case *ast.TypeExpr:
		fields := make([]registry.FieldDef, 0, len(typed.Fields))
		seen := make(map[string]bool)
		ok := true
		for _, field := range typed.Fields {
			if seen[field.Name.String()] {
				r.addDuplicateDeclaration(typed.Tok(), field.Name.Tok())
				ok = false
				continue
			}
			seen[field.Name.String()] = true
			fieldType, exists := r.reg.LookupType(field.Type.String())
			if !exists {
				r.addUndefinedType(field.Type.Tok())
				ok = false
				continue
			}
			fields = append(fields, registry.FieldDef{Name: field.Name.String(), Type: fieldType})
		}
		if !ok {
			return registry.ErrorTypeID, false
		}
		typeID, err := r.reg.RegisterScriptType(namespace+"."+declName, fields)
		if err != nil {
			// unreachable: duplicate decl names are caught by the env check first
			return registry.ErrorTypeID, false
		}
		return typeID, true
	default:
		return registry.ErrorTypeID, false
	}
}

func (r *Resolver) resolveConst(consts []*ast.ConstDecl, env *Env) []*resolved.ConstDecl {
	resolvedConsts := make([]*resolved.ConstDecl, 0)
	for _, c := range consts {
		sym := Symbol(c.Identifier.String())
		if binding, exists := env.Get(sym); exists {
			if binding.Reserved() {
				r.addReservedName(c.Identifier.Tok(), binding.Kind)
			} else {
				r.addDuplicateDeclaration(c.Tok(), c.Identifier.Tok())
			}

			continue
		}
		constValue := r.resolveExpr(c.Value, env)
		env.Set(sym, Binding{T: constValue.Type(), Kind: KindConst})
		resolvedConsts = append(resolvedConsts, &resolved.ConstDecl{
			Token: c.Tok(),
			Name:  c.Identifier.String(),
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
			r.addDuplicateDeclaration(fieldEntry.Tok(), fieldEntry.Identifier.Tok())
			continue
		}
		seen[name] = true

		typeID, exists := r.reg.LookupType(fieldEntry.Type.String())
		if !exists {
			typeToken := fieldEntry.Tok()
			if typeIdent, ok := fieldEntry.Type.(*ast.IdentExpr); ok {
				typeToken = typeIdent.Tok()
			}
			r.addUndefinedType(typeToken)
			typeID = registry.ErrorTypeID
		}
		field := &resolved.StateField{
			Token: fieldEntry.Identifier.Tok(),
			Name:  name,
			T:     typeID,
		}
		if fieldEntry.Value != nil {
			initValue := r.resolveExpr(fieldEntry.Value, env)
			if !isErrorType(initValue.Type(), typeID) && initValue.Type() != typeID {
				coerceRule, coerceExists := r.reg.LookupCoerce(initValue.Type(), typeID)
				if !coerceExists {
					r.addTypeMismatch(fieldEntry.Identifier.Tok(), typeID, initValue.Type())
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

func isErrorType(types ...registry.TypeID) bool {
	for _, t := range types {
		if t == registry.ErrorTypeID {
			return true
		}
	}
	return false
}

func (r *Resolver) newErrorExpr(tok token.Token) resolved.Expression {
	if len(r.errs) == 0 {
		panic("error type with no diagnostic behind it: the type is reserved, so the resolver minted one instead of recovering from a reported error")
	}
	return &resolved.BadExpr{Token: tok}
}

func (r *Resolver) resolveExpr(expr ast.Expression, env *Env) resolved.Expression {
	switch typedExpr := expr.(type) {
	case *ast.IntegerExpr:
		return &resolved.IntegerExpr{Token: typedExpr.Tok(), Value: typedExpr.Value, T: registry.IntegerID}
	case *ast.FloatExpr:
		return &resolved.FloatExpr{Token: typedExpr.Tok(), Value: typedExpr.Value, T: registry.FloatID}
	case *ast.StringExpr:
		return &resolved.StringExpr{Token: typedExpr.Tok(), Value: typedExpr.Value, T: registry.StringID}
	case *ast.BooleanExpr:
		return &resolved.BooleanExpr{Token: typedExpr.Tok(), Value: typedExpr.Value, T: registry.BoolID}
	case *ast.IdentExpr:
		binding, exists := env.Get(Symbol(typedExpr.String()))
		if !exists {
			r.addUndefinedIdent(typedExpr.Tok())
			return &resolved.BadExpr{Token: typedExpr.Tok()}
		}
		if !binding.Readable() {
			r.addNotReadable(typedExpr.Tok(), binding.Kind)
			return &resolved.BadExpr{Token: typedExpr.Tok()}
		}
		return &resolved.IdentExpr{Token: typedExpr.Tok(), T: binding.T}
	case *ast.InfixExpr:
		errsBefore := len(r.errs)
		left := r.resolveExpr(typedExpr.Left, env)
		right := r.resolveExpr(typedExpr.Right, env)
		if typedExpr.Tok().Type == token.AND || typedExpr.Tok().Type == token.OR {
			return r.resolveLogical(typedExpr, left, right)
		}
		if isErrorType(left.Type(), right.Type()) {
			return r.newErrorExpr(typedExpr.Tok())
		}
		binaryRule, exists := r.reg.LookupBinary(typedExpr.Tok().Type, left.Type(), right.Type())
		if !exists {
			if len(r.errs) == errsBefore {
				r.addInvalidBinaryOp(typedExpr.Tok(), left.Type(), right.Type())
			}
			return &resolved.BadExpr{Token: typedExpr.Tok()}
		}
		return &resolved.InfixExpr{Token: typedExpr.Tok(), Left: left, Right: right, T: binaryRule.EvalType, EvalFn: binaryRule.EvalFn}
	case *ast.PrefixExpr:
		errsBefore := len(r.errs)
		right := r.resolveExpr(typedExpr.Right, env)
		if isErrorType(right.Type()) {
			return r.newErrorExpr(typedExpr.Tok())
		}
		unaryRule, exists := r.reg.LookupUnary(typedExpr.Tok().Type, right.Type())
		if !exists {
			if len(r.errs) == errsBefore {
				r.addInvalidUnaryOp(typedExpr.Tok(), right.Type())
			}
			return &resolved.BadExpr{Token: typedExpr.Tok()}
		}
		return &resolved.PrefixExpr{Token: typedExpr.Tok(), Right: right, T: unaryRule.EvalType, EvalFn: unaryRule.EvalFn}

	case *ast.MemberAccessExpr:
		errsBefore := len(r.errs)
		identExpr, isIdent := typedExpr.Object.(*ast.IdentExpr)
		if isIdent && identExpr.Tok().Type == token.STATE {
			fieldName := typedExpr.Member.String()
			var fieldDecl *resolved.StateField

			for _, field := range r.resolvedProg.State.Fields {
				if field.Name == fieldName {
					fieldDecl = field
					break
				}
			}

			if fieldDecl == nil {
				r.addStateUndeclared(typedExpr.Member.Tok())
				return &resolved.BadExpr{Token: typedExpr.Tok()}
			}

			return &resolved.StateAccessExpr{
				Token: typedExpr.Tok(),
				Field: fieldName,
				T:     fieldDecl.T,
			}
		}

		resolvedExpr := r.resolveReceiver(typedExpr.Object, env)
		if isErrorType(resolvedExpr.Type()) {
			return r.newErrorExpr(typedExpr.Tok())
		}
		rule, exists := r.reg.LookupMemberAccess(resolvedExpr.Type(), typedExpr.Member.String())
		if !exists {
			if len(r.errs) == errsBefore {
				r.addUndefinedAttribute(typedExpr.Member.Tok())
			}
			return &resolved.BadExpr{Token: typedExpr.Tok()}
		}
		return &resolved.MemberAccessExpr{
			Token:     typedExpr.Tok(),
			Object:    resolvedExpr,
			Attribute: typedExpr.Member.String(),
			T:         rule.EvalType,
			EvalFn:    rule.EvalFn,
		}
	case *ast.IndexExpr:
		errsBefore := len(r.errs)
		left := r.resolveExpr(typedExpr.Left, env)
		r.resolveExpr(typedExpr.Index, env)
		if isErrorType(left.Type()) {
			return r.newErrorExpr(typedExpr.Tok())
		}
		if len(r.errs) == errsBefore {
			r.addNotIndexable(typedExpr.Tok(), left.Type())
		}
		return &resolved.BadExpr{Token: typedExpr.Tok()}
	case *ast.CallExpr:
		switch callee := typedExpr.Callee.(type) {
		case *ast.MemberAccessExpr:
			errsBefore := len(r.errs)
			resolvedExpr := r.resolveReceiver(callee.Object, env)
			if isErrorType(resolvedExpr.Type()) {
				return r.newErrorExpr(typedExpr.Tok())
			}

			rule, callExists := r.reg.LookupCall(resolvedExpr.Type(), callee.Member.String())
			if !callExists {
				if len(r.errs) == errsBefore {
					r.addUndefinedMethod(callee.Member.Tok())
				}
				return &resolved.BadExpr{Token: typedExpr.Tok()}
			}

			args, valid := r.resolveArgs(typedExpr.Tok(), typedExpr.Args, typedExpr.Kwargs, rule, env)
			if valid {
				return &resolved.MethodCallExpr{
					Token:    typedExpr.Tok(),
					Receiver: resolvedExpr,
					Method:   callee.Member.String(),
					Args:     args,
					T:        rule.EvalType,
					EvalFn:   rule.EvalFn,
				}
			}
			return &resolved.BadExpr{Token: typedExpr.Tok()}
		case *ast.IdentExpr:
			if isEmitCall(typedExpr) {
				// if we got here - emit was used inside Expression. otherwise it will be parsed by separate case branch
				r.addEmitNotExpression(callee.Tok())
			} else {
				r.addUndefinedFunc(callee.Tok())
			}
			return &resolved.BadExpr{Token: typedExpr.Tok()}
		default:
			r.addNotCallable(typedExpr.Tok())
			return &resolved.BadExpr{Token: typedExpr.Tok()}
		}

	default:
		panic(fmt.Sprintf("unhandled ast expression %T: ast grew a node the resolver never wired up", typedExpr))
	}
}

func (r *Resolver) resolveReceiver(expr ast.Expression, env *Env) resolved.Expression {
	identExpr, isIdent := expr.(*ast.IdentExpr)
	if !isIdent {
		return r.resolveExpr(expr, env)
	}
	binding, exists := env.Get(Symbol(identExpr.String()))
	if !exists || binding.Kind != KindModule {
		return r.resolveExpr(expr, env)
	}
	return &resolved.IdentExpr{Token: identExpr.Tok(), T: binding.T}
}

func (r *Resolver) resolveArgs(callToken token.Token, args []ast.Expression, kwargs []*ast.KwargsExpr, rule registry.CallRule, env *Env) ([]*resolved.CallArgExpr, bool) {
	if len(args)+len(kwargs) != len(rule.Args) {
		r.addArgCountMismatch(callToken, len(rule.Args), len(args)+len(kwargs))
		return nil, false
	}
	resolvedIdx := make([]bool, len(rule.Args))
	resolvedArgs := make([]*resolved.CallArgExpr, len(args)+len(kwargs))
	hasErrs := false

	for i, arg := range args {
		argRule := rule.Args[i]
		resolvedIdx[i] = true
		resolvedArg := r.resolveExpr(arg, env)
		typeMatches := resolvedArg.Type() == argRule.Type
		if !typeMatches && !argRule.Exact {
			if coerceRule, canCoerce := r.reg.LookupCoerce(resolvedArg.Type(), argRule.Type); canCoerce {
				resolvedArg = &resolved.CoerceExpr{Inner: resolvedArg, T: coerceRule.EvalType, EvalFn: coerceRule.EvalFn}
				typeMatches = true
			}
		}

		if !typeMatches && !isErrorType(resolvedArg.Type(), argRule.Type) {
			hasErrs = true
			r.addTypeMismatch(arg.Tok(), argRule.Type, resolvedArg.Type())
		}

		resolvedArgs[i] = &resolved.CallArgExpr{
			Token: arg.Tok(),
			Name:  argRule.Name,
			Value: resolvedArg,
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
			r.addUnknownKwarg(kwArg.Tok(), kwArg.Key.String())
			hasErrs = true
			continue
		}

		if resolvedIdx[argRuleIdx] {
			r.addDuplicateArg(kwArg.Tok(), argRule.Name)
			hasErrs = true
			continue
		}

		resolvedIdx[argRuleIdx] = true
		resolvedArg := r.resolveExpr(kwArg.Value, env)
		typeMatches := resolvedArg.Type() == argRule.Type
		if !typeMatches && !argRule.Exact {
			if coerceRule, canCoerce := r.reg.LookupCoerce(resolvedArg.Type(), argRule.Type); canCoerce {
				resolvedArg = &resolved.CoerceExpr{Inner: resolvedArg, T: coerceRule.EvalType, EvalFn: coerceRule.EvalFn}
				typeMatches = true
			}
		}

		if !typeMatches && !isErrorType(resolvedArg.Type(), argRule.Type) {
			hasErrs = true
			r.addTypeMismatch(kwArg.Tok(), argRule.Type, resolvedArg.Type())
		}

		resolvedArgs[argRuleIdx] = &resolved.CallArgExpr{
			Token: kwArg.Tok(),
			Name:  argRule.Name,
			Value: resolvedArg,
		}
	}

	for i, isResolved := range resolvedIdx {
		if !isResolved {
			unresolvedRule := rule.Args[i]
			r.addArgsMissing(callToken, unresolvedRule.Name)
			hasErrs = true
		}
	}

	return resolvedArgs, !hasErrs
}
