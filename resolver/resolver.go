package resolver

import (
	"fmt"
	"slices"

	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/resolved"
	"github.com/MoroZvlg/tascript/token"
)

type Symbol string

// Resolver doing both resolve and type check
type Resolver struct {
	prog          *ast.Program
	resolvedProg  *resolved.Program
	reg           *registry.Registry
	errs          []diag.Diagnostic
	currFn        string
	slotCount     int
	declaredNames map[Symbol]bool
}

func New(prog *ast.Program, reg *registry.Registry) *Resolver {
	return &Resolver{
		prog:          prog,
		resolvedProg:  &resolved.Program{},
		reg:           reg,
		declaredNames: make(map[Symbol]bool),
	}
}

func (r *Resolver) Diagnostics() []diag.Diagnostic {
	return r.errs
}

// Resolve walks the top-level declarations in source order: that walk IS the scoping rule
func (r *Resolver) Resolve() *resolved.Program {
	topLevelEnv := EnvFromRegistry(r.reg)
	r.collectDeclaredNames()

	for _, decl := range r.prog.Decls {
		switch typedDecl := decl.(type) {
		case *ast.ConstDecl:
			r.resolveConst(typedDecl, topLevelEnv)
		case *ast.InputDecl:
			r.resolveInput(typedDecl, topLevelEnv)
		case *ast.OutputDecl:
			r.resolveOutput(typedDecl, topLevelEnv)
		case *ast.KindDecl:
			r.resolveKindDecl(typedDecl, topLevelEnv)
		default:
			panic(fmt.Sprintf("unhandled ast declaration %T: ast grew a node the resolver never wired up", typedDecl))
		}
	}

	if r.prog.InitFn != nil {
		r.currFn = "Init"
		r.resolvedProg.InitFn = r.resolveFunc(r.prog.InitFn, NewEnclosedEnv(topLevelEnv))
	}

	r.currFn = "Run"
	r.resolvedProg.RunFn = r.resolveFunc(r.prog.RunFn, NewEnclosedEnv(topLevelEnv))

	return r.resolvedProg
}

// collectDeclaredNames feeds USE_BEFORE_DECLARATION: a name missing from the env
// during the walk but present here is declared further down.
func (r *Resolver) collectDeclaredNames() {
	for _, decl := range r.prog.Decls {
		switch typedDecl := decl.(type) {
		case *ast.ConstDecl:
			r.declaredNames[Symbol(typedDecl.Identifier.String())] = true
		case *ast.InputDecl:
			r.declaredNames[Symbol(typedDecl.Identifier.String())] = true
		case *ast.OutputDecl:
			r.declaredNames[Symbol(typedDecl.Identifier.String())] = true
		case *ast.KindDecl:
			kind, exists := r.reg.LookupDeclKind(typedDecl.Keyword)
			if !exists {
				continue
			}
			r.declaredNames[slotSymbol(kind, typedDecl.Identifier.String())] = true
		}
	}
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
		env.Set(Symbol(name), Binding{T: exprVal.Type(), Kind: KindLet, assignable: true})
		return &resolved.LetStmt{
			Token: astStmtTyped.Tok(),
			Name:  name,
			Value: exprVal,
			T:     exprVal.Type(),
		}
	case *ast.AssignStmt:
		value := r.resolveExpr(astStmtTyped.Value, env)
		binding, targetTok, ok := r.resolveAssignTarget(astStmtTyped, env)
		if !ok {
			return &resolved.BadStmt{Token: targetTok}
		}
		if !binding.Assignable() {
			r.addNotAssignable(targetTok, binding.KindLabel())
			return &resolved.BadStmt{Token: targetTok}
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
		if binding.Slot != nil {
			return &resolved.AssignSlotStmt{
				Token:  astStmtTyped.Tok(),
				Target: binding.Slot,
				Value:  value,
				T:      binding.T,
			}
		}
		return &resolved.AssignNameStmt{
			Token:  astStmtTyped.Tok(),
			Target: targetTok.Literal,
			Value:  value,
			T:      binding.T,
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

	args, valid := r.resolveArgs(call.Tok(), call.Args[1:], call.Kwargs, r.reg.EmitRule(binding.T), 1, env)
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

func (r *Resolver) resolveInput(in *ast.InputDecl, env *Env) {
	sym := Symbol(in.Identifier.String())
	if binding, exists := env.Get(sym); exists {
		if binding.Reserved() {
			r.addReservedName(in.Identifier.Tok(), binding.Kind)
		} else {
			r.addDuplicateDeclaration(in.Tok(), in.Identifier.Tok())
		}
		return
	}
	typeID := r.resolveTypeDecl(in.Type, "input", in.Identifier.String())
	env.Set(sym, Binding{T: typeID, Kind: KindInput})
	decl := &resolved.InputDecl{
		Token: in.Tok(),
		Name:  in.Identifier.String(),
		T:     typeID,
	}
	r.resolvedProg.Decls = append(r.resolvedProg.Decls, decl)
}

func (r *Resolver) resolveOutput(out *ast.OutputDecl, env *Env) {
	sym := Symbol(out.Identifier.String())
	if binding, exists := env.Get(sym); exists {
		if binding.Reserved() {
			r.addReservedName(out.Identifier.Tok(), binding.Kind)
		} else {
			r.addDuplicateDeclaration(out.Tok(), out.Identifier.Tok())
		}
		return
	}
	typeID := r.resolveTypeDecl(out.Type, "output", out.Identifier.String())
	env.Set(sym, Binding{T: typeID, Kind: KindOutput})
	decl := &resolved.OutputDecl{
		Token: out.Tok(),
		Name:  out.Identifier.String(),
		T:     typeID,
	}
	r.resolvedProg.Decls = append(r.resolvedProg.Decls, decl)
}

func (r *Resolver) resolveTypeDecl(typeDecl ast.TypeDecl, namespace, declName string) registry.TypeID {
	switch typed := typeDecl.(type) {
	case *ast.IdentExpr:
		typeID, exists := r.reg.LookupType(typed.String())
		if !exists {
			r.addUndefinedType(typed.Tok())
			return registry.ErrorTypeID
		}
		return typeID
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
			return registry.ErrorTypeID
		}
		name := namespace + "." + declName
		typeID, err := r.reg.RegisterScriptType(name, fields)
		if err != nil {
			r.addTypeRegistrationFail(typed.Tok(), name, err)
			return registry.ErrorTypeID
		}
		return typeID
	default:
		return registry.ErrorTypeID
	}
}

func (r *Resolver) resolveConst(c *ast.ConstDecl, env *Env) {
	sym := Symbol(c.Identifier.String())
	if binding, exists := env.Get(sym); exists {
		if binding.Reserved() {
			r.addReservedName(c.Identifier.Tok(), binding.Kind)
		} else {
			r.addDuplicateDeclaration(c.Tok(), c.Identifier.Tok())
		}
		return
	}
	constValue := r.resolveExpr(c.Value, env)
	env.Set(sym, Binding{T: constValue.Type(), Kind: KindConst})
	r.resolvedProg.Decls = append(r.resolvedProg.Decls, &resolved.ConstDecl{
		Token: c.Tok(),
		Name:  c.Identifier.String(),
		Value: constValue,
		T:     constValue.Type(),
	})
}

func (r *Resolver) resolveKindDecl(d *ast.KindDecl, env *Env) {
	kind, exists := r.reg.LookupDeclKind(d.Keyword)
	if !exists {
		r.addUnknownDeclKeyword(d.Tok())
		return
	}

	name := d.Identifier.String()
	sym := slotSymbol(kind, name)
	if binding, taken := env.Get(sym); taken {
		if binding.Reserved() {
			r.addReservedName(d.Identifier.Tok(), binding.Kind)
		} else {
			r.addDuplicateDeclaration(d.Tok(), d.Identifier.Tok())
		}
		return
	}

	switch kind.Initializer {
	case registry.InitializerRequired:
		if d.Value == nil {
			r.addInitializerRequired(d.Identifier.Tok(), kind.Word)
		}
	case registry.InitializerForbidden:
		if d.Value != nil {
			r.addInitializerForbidden(d.Identifier.Tok(), kind.Word)
		}
	case registry.InitializerOptional:
	}

	slotType := registry.ErrorTypeID
	if d.Type != nil {
		typeID, typeExists := r.reg.LookupType(d.Type.String())
		if !typeExists {
			r.addUndefinedType(d.Type.Tok())
			typeID = registry.ErrorTypeID
		}
		slotType = typeID
	}

	var initValue resolved.Expression
	if d.Value != nil {
		initValue = r.resolveExpr(d.Value, env)
	}

	switch {
	case d.Type != nil && initValue != nil:
		if !isErrorType(initValue.Type(), slotType) && initValue.Type() != slotType {
			coerceRule, canCoerce := r.reg.LookupCoerce(initValue.Type(), slotType)
			if !canCoerce {
				r.addTypeMismatch(d.Identifier.Tok(), slotType, initValue.Type())
				return
			}
			initValue = &resolved.CoerceExpr{Inner: initValue, T: coerceRule.EvalType, EvalFn: coerceRule.EvalFn}
		}
	case d.Type == nil && initValue != nil:
		slotType = initValue.Type()
	case d.Type == nil:
		r.addTypeRequired(d.Identifier.Tok(), kind.Word, name)
	}

	if len(kind.AllowedTypes) > 0 && !isErrorType(slotType) && !slices.Contains(kind.AllowedTypes, slotType) {
		r.addDeclTypeNotAllowed(d.Identifier.Tok(), kind.Word, slotType)
	}

	slot := &resolved.SlotDecl{
		Token: d.Identifier.Tok(),
		Kind:  kind.Word,
		Name:  name,
		T:     slotType,
		Init:  initValue,
		Index: r.slotCount,
	}
	r.slotCount++
	env.Set(sym, Binding{T: slotType, Kind: KindSlot, Slot: slot, assignable: kind.Assignable})
	r.resolvedProg.Decls = append(r.resolvedProg.Decls, slot)
}

func (r *Resolver) resolveAssignTarget(stmt *ast.AssignStmt, env *Env) (Binding, token.Token, bool) {
	switch target := stmt.Target.(type) {
	case *ast.IdentExpr:
		binding, exists := env.Get(Symbol(target.String()))
		if !exists {
			r.addUndefinedVar(target.Tok())
			return Binding{}, target.Tok(), false
		}
		return binding, target.Tok(), true
	case *ast.MemberAccessExpr:
		identExpr, isIdent := target.Object.(*ast.IdentExpr)
		if !isIdent {
			r.addInvalidAssignTarget(stmt.Tok())
			return Binding{}, stmt.Tok(), false
		}
		if binding, declared := env.Get(Symbol(identExpr.String())); !declared || binding.Kind != KindDeclWord {
			r.addInvalidAssignTarget(stmt.Tok())
			return Binding{}, stmt.Tok(), false
		}
		binding, declared := env.Get(namespacedSymbol(identExpr.String(), target.Member.String()))
		if !declared {
			r.addSlotUndeclared(target.Member.Tok(), identExpr.String())
			return Binding{}, stmt.Tok(), false
		}
		return binding, target.Member.Tok(), true
	default:
		r.addInvalidAssignTarget(stmt.Tok())
		return Binding{}, stmt.Tok(), false
	}
}

// a KindDeclWord binding exists only for a namespaced kind, so the key is always qualified
func (r *Resolver) resolveSlotRef(word, member *ast.IdentExpr, env *Env) resolved.Expression {
	sym := namespacedSymbol(word.String(), member.String())
	binding, exists := env.Get(sym)
	if !exists {
		if r.currFn == "" && r.declaredNames[sym] {
			r.addUseBeforeDeclaration(member.Tok())
		} else {
			r.addSlotUndeclared(member.Tok(), word.String())
		}
		return &resolved.BadExpr{Token: member.Tok()}
	}
	return &resolved.SlotRefExpr{Token: member.Tok(), Slot: binding.Slot}
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
			if r.currFn == "" && r.declaredNames[Symbol(typedExpr.String())] {
				r.addUseBeforeDeclaration(typedExpr.Tok())
			} else {
				r.addUndefinedIdent(typedExpr.Tok())
			}
			return &resolved.BadExpr{Token: typedExpr.Tok()}
		}
		if !binding.Readable() {
			r.addNotReadable(typedExpr.Tok(), binding.Kind)
			return &resolved.BadExpr{Token: typedExpr.Tok()}
		}
		if binding.Kind == KindInput && r.currFn != "Run" {
			r.addInputInInit(typedExpr.Tok())
			return &resolved.BadExpr{Token: typedExpr.Tok()}
		}
		if binding.Slot != nil {
			return &resolved.SlotRefExpr{Token: typedExpr.Tok(), Slot: binding.Slot}
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
		if isIdent {
			if binding, declared := env.Get(Symbol(identExpr.String())); declared && binding.Kind == KindDeclWord {
				return r.resolveSlotRef(identExpr, typedExpr.Member, env)
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

			args, valid := r.resolveArgs(typedExpr.Tok(), typedExpr.Args, typedExpr.Kwargs, rule, 0, env)
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

// leadingArgs are arguments the caller consumed before this point (emit's output name).
// They count toward nothing but the diagnostic, which must match what the user typed.
func (r *Resolver) resolveArgs(callToken token.Token, args []ast.Expression, kwargs []*ast.KwargsExpr, rule registry.CallRule, leadingArgs int, env *Env) ([]*resolved.CallArgExpr, bool) {
	if len(args)+len(kwargs) != len(rule.Args) {
		r.addArgCountMismatch(callToken, len(rule.Args)+leadingArgs, len(args)+len(kwargs)+leadingArgs)
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
