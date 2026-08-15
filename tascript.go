// Package tascript is not safe for concurrent use: a Builder, the Executable it produces,
// and that Executable's state are single-threaded by contract. To run scripts in parallel,
// give each goroutine its own Builder and Executable.
package tascript

import (
	"errors"

	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/evaluator"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/parser"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/resolver"
	"github.com/MoroZvlg/tascript/stdlib"
	"github.com/MoroZvlg/tascript/token"
)

var ErrBuilderSpent = errors.New("a builder compiles one script: build another to compile again")

type Builder struct {
	reg   *registry.Registry
	spent bool
}

// TODO: make the stdlib modules opt-in (WithMath / WithTime) rather than always registered.
func NewBuilder() *Builder {
	reg := registry.DefaultRegistry()
	stdlib.Register(reg)

	return &Builder{reg: reg}
}

// Compile spends the builder whether or not the script compiles — a rejected script is retried on a fresh one.
func (b *Builder) Compile(src string) (*Executable, []diag.Diagnostic, error) {
	if b.spent {
		return nil, nil, ErrBuilderSpent
	}
	b.spent = true

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
	if b.spent {
		return id, ErrBuilderSpent
	}
	return id, b.reg.RegisterType(id, registry.ScalarShape)
}

func (b *Builder) RegisterBinary(tok token.TokenType, left, right registry.TypeID, rule registry.BinaryRule) error {
	if b.spent {
		return ErrBuilderSpent
	}
	return b.reg.RegisterBinary(tok, left, right, rule)
}

func (b *Builder) RegisterUnary(tok token.TokenType, right registry.TypeID, rule registry.UnaryRule) error {
	if b.spent {
		return ErrBuilderSpent
	}
	return b.reg.RegisterUnary(tok, right, rule)
}

func (b *Builder) RegisterMemberAccess(owner registry.TypeID, member string, rule registry.MemberAccessRule) error {
	if b.spent {
		return ErrBuilderSpent
	}
	return b.reg.RegisterMemberAccess(owner, member, rule)
}

func (b *Builder) RegisterCall(owner registry.TypeID, member string, rule registry.CallRule) error {
	if b.spent {
		return ErrBuilderSpent
	}
	return b.reg.RegisterCall(owner, member, rule)
}

func (b *Builder) RegisterModule(name string) (registry.Value, error) {
	if b.spent {
		return nil, ErrBuilderSpent
	}
	return b.reg.RegisterModule(name)
}

func (b *Builder) RegisterCoercion(from, to registry.TypeID, rule registry.CoerceRule) error {
	if b.spent {
		return ErrBuilderSpent
	}
	return b.reg.RegisterCoercion(from, to, rule)
}
