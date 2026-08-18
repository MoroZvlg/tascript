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
)

var ErrBuilderSpent = errors.New("a builder compiles one script: build another to compile again")

type Builder struct {
	reg   *registry.Registry
	spent bool
}

func NewBuilder() *Builder {
	return &Builder{reg: registry.DefaultRegistry()}
}

// Compile seals the returned registry, so registration after that point is registry.ErrSealed.
func (b *Builder) Registry() *registry.Registry { return b.reg }

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
	b.reg.Seal()
	if len(r.Diagnostics()) > 0 {
		return nil, r.Diagnostics(), nil
	}

	return &Executable{eval: evaluator.New(resolvedProg, b.reg)}, nil, nil
}
