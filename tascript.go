package tascript

import (
	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/evaluator"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/parser"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/resolver"
	"github.com/MoroZvlg/tascript/token"
)

type Engine struct {
	prog *ast.Program
	reg  *registry.Registry
}

func NewEngine(src string) (*Engine, []diag.Diagnostic) {
	p := parser.New(lexer.New(src))
	prog := p.Parse()
	if len(p.Diagnostics()) > 0 {
		return nil, p.Diagnostics()
	}

	reg := registry.DefaultRegistry()

	return &Engine{
		prog: prog,
		reg:  reg,
	}, nil
}

func (e *Engine) Compile() (*Executable, []diag.Diagnostic) {
	r := resolver.New(e.prog, e.reg)
	r.Resolve()

	if len(r.Diagnostics()) > 0 {
		return nil, r.Diagnostics()
	}

	eval := evaluator.New(e.prog, e.reg)
	return &Executable{eval: eval}, nil
}

func (e *Engine) RegisterType(customType string) (registry.TypeID, error) {
	return e.reg.RegisterType(customType)
}

func (e *Engine) RegisterBinary(tok token.TokenType, left, right registry.TypeID, rule registry.BinaryRule) error {
	return e.reg.RegisterBinary(tok, left, right, rule)
}
