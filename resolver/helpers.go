package resolver

import (
	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/token"
)

func (r *Resolver) addDuplicateDeclaration(kwToken, identToken token.Token) {
	r.errs = append(r.errs, &diag.DuplicateDeclaration{
		At:      identToken.Pos,
		Keyword: kwToken.Literal,
		Name:    identToken.Literal,
	})
}

func (r *Resolver) addReservedName(identToken token.Token, kind BindingKind) {
	r.errs = append(r.errs, &diag.ReservedName{
		At:   identToken.Pos,
		Name: identToken.Literal,
		Kind: string(kind),
	})
}

func (r *Resolver) addInvalidBinaryOp(cmpToken token.Token, left, right registry.TypeID) {
	r.errs = append(r.errs, &diag.InvalidBinaryOperation{
		At:    cmpToken.Pos,
		Op:    cmpToken.Literal,
		Left:  left,
		Right: right,
	})
}

func (r *Resolver) addInvalidUnaryOp(opToken token.Token, right registry.TypeID) {
	r.errs = append(r.errs, &diag.InvalidUnaryOperation{
		At:    opToken.Pos,
		Op:    opToken.Literal,
		Right: right,
	})
}

func (r *Resolver) addUndefinedIdent(opToken token.Token) {
	r.errs = append(r.errs, &diag.UndefinedIdent{
		At:   opToken.Pos,
		Name: opToken.Literal,
	})
}

func (r *Resolver) addUndefinedVar(opToken token.Token) {
	r.errs = append(r.errs, &diag.UndefinedVar{
		At:   opToken.Pos,
		Name: opToken.Literal,
	})
}

func (r *Resolver) addInvalidAssignTarget(opToken token.Token) {
	r.errs = append(r.errs, &diag.InvalidAssignTarget{
		At: opToken.Pos,
	})
}

func (r *Resolver) addOutputNotReadable(opToken token.Token) {
	r.errs = append(r.errs, &diag.OutputNotReadable{
		At:   opToken.Pos,
		Name: opToken.Literal,
	})
}

func (r *Resolver) addInvalidEmitTarget(opToken token.Token) {
	r.errs = append(r.errs, &diag.InvalidEmitTarget{
		At: opToken.Pos,
	})
}

func (r *Resolver) addNotAssignable(opToken token.Token, kind BindingKind) {
	r.errs = append(r.errs, &diag.NotAssignable{
		At:   opToken.Pos,
		Name: opToken.Literal,
		Kind: string(kind),
	})
}

func (r *Resolver) addUndefinedType(opToken token.Token) {
	r.errs = append(r.errs, &diag.UndefinedType{
		At:   opToken.Pos,
		Name: opToken.Literal,
	})
}

func (r *Resolver) addUndefinedAttribute(opToken token.Token) {
	r.errs = append(r.errs, &diag.UndefinedAttribute{
		At:   opToken.Pos,
		Name: opToken.Literal,
	})
}

func (r *Resolver) addUndefinedMethod(opToken token.Token) {
	r.errs = append(r.errs, &diag.UndefinedMethod{
		At:   opToken.Pos,
		Name: opToken.Literal,
	})
}

func (r *Resolver) addNotIndexable(bracketToken token.Token, left registry.TypeID) {
	r.errs = append(r.errs, &diag.NotIndexable{
		At:   bracketToken.Pos,
		Left: left,
	})
}

func (r *Resolver) addUndefinedFunc(calleeToken token.Token) {
	r.errs = append(r.errs, &diag.UndefinedFunc{
		At:   calleeToken.Pos,
		Name: calleeToken.Literal,
	})
}

func (r *Resolver) addNotCallable(callToken token.Token) {
	r.errs = append(r.errs, &diag.NotCallable{
		At: callToken.Pos,
	})
}

func (r *Resolver) addEmitNotExpression(calleeToken token.Token) {
	r.errs = append(r.errs, &diag.EmitNotExpression{
		At: calleeToken.Pos,
	})
}

func (r *Resolver) addEmitOutsideRun(calleeToken token.Token) {
	r.errs = append(r.errs, &diag.EmitOutsideRun{
		At: calleeToken.Pos,
	})
}

func (r *Resolver) addArgCountMismatch(opToken token.Token, expected, got int) {
	r.errs = append(r.errs, &diag.ArgCountMismatch{
		At:       opToken.Pos,
		Expected: expected,
		Got:      got,
	})
}

func (r *Resolver) addArgsMissing(opToken token.Token, expected string) {
	r.errs = append(r.errs, &diag.ArgMissing{
		At:       opToken.Pos,
		Expected: expected,
	})
}

func (r *Resolver) addDuplicateArg(kwToken token.Token, name string) {
	r.errs = append(r.errs, &diag.ArgDuplicate{
		At:   kwToken.Pos,
		Name: name,
	})
}

func (r *Resolver) addUnknownKwarg(kwToken token.Token, name string) {
	r.errs = append(r.errs, &diag.ArgUnknown{
		At:   kwToken.Pos,
		Name: name,
	})
}

func (r *Resolver) addTypeMismatch(opToken token.Token, expected, got registry.TypeID) {
	r.errs = append(r.errs, &diag.TypeMismatch{
		At:       opToken.Pos,
		Expected: expected,
		Got:      got,
	})
}

func (r *Resolver) addStateUndeclared(fieldToken token.Token) {
	r.errs = append(r.errs, &diag.StateUndeclared{
		At:    fieldToken.Pos,
		Field: fieldToken.Literal,
	})
}

func (r *Resolver) addStateUninitialized(fieldToken token.Token) {
	r.errs = append(r.errs, &diag.StateUninitialized{
		At:    fieldToken.Pos,
		Field: fieldToken.Literal,
	})
}
