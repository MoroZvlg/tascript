// Package tascript is not safe for concurrent use: a Builder, the Executable it produces,
// and that Executable's state are single-threaded by contract. To run scripts in parallel,
// give each goroutine its own Builder and Executable.
package tascript

import (
	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/evaluator"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/parser"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/resolver"
	"github.com/MoroZvlg/tascript/stdlib"
	"github.com/MoroZvlg/tascript/token"
)

type Builder struct {
	reg *registry.Registry
}

// TODO: make the stdlib modules opt-in (WithMath / WithTime) rather than always registered.
func NewBuilder() *Builder {
	reg := registry.DefaultRegistry()
	stdlib.Register(reg)

	return &Builder{reg: reg}
}

// TODO: one builder, one script, one executable, unenforced on both counts. A second
// Compile corrupts synthesized port types, and any Register* after Compile mutates the
// live executable's vocabulary — builder and executable share one registry. Enforce via
// the error return.
func (b *Builder) Compile(src string) (*Executable, []diag.Diagnostic, error) {
	p := parser.New(lexer.New(src))
	prog := p.Parse()
	if len(p.Diagnostics()) > 0 {
		return nil, p.Diagnostics(), nil
	}

	r := resolver.New(prog, b.reg)
	resolvedProg := r.Resolve()
	if len(r.Diagnostics()) > 0 {
		return nil, r.Diagnostics(), nil
	}

	return &Executable{eval: evaluator.New(resolvedProg, b.reg)}, nil, nil
}

func (b *Builder) RegisterType(customType string) (registry.TypeID, error) {
	id := registry.NewTypeID(customType)
	return id, b.reg.RegisterType(id, registry.ScalarShape)
}

func (b *Builder) RegisterBinary(tok token.TokenType, left, right registry.TypeID, rule registry.BinaryRule) error {
	return b.reg.RegisterBinary(tok, left, right, rule)
}

func (b *Builder) RegisterUnary(tok token.TokenType, right registry.TypeID, rule registry.UnaryRule) error {
	return b.reg.RegisterUnary(tok, right, rule)
}

func (b *Builder) RegisterMemberAccess(owner registry.TypeID, member string, rule registry.MemberAccessRule) error {
	return b.reg.RegisterMemberAccess(owner, member, rule)
}

func (b *Builder) RegisterCall(owner registry.TypeID, member string, rule registry.CallRule) error {
	return b.reg.RegisterCall(owner, member, rule)
}

func (b *Builder) RegisterModule(name string) (registry.Value, error) {
	return b.reg.RegisterModule(name)
}

func (b *Builder) RegisterCoercion(from, to registry.TypeID, rule registry.CoerceRule) error {
	return b.reg.RegisterCoercion(from, to, rule)
}
