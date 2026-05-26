// Package tascript is the public entrypoint for the tascript DSL.
//
// Surface (slice 0):
//
//	prog, diags, err := tascript.Compile(src)
//	runner, err := tascript.Launch(prog, tascript.Wiring{})
//	runner.Init()
//	runner.Step()
//	events := runner.DrainEvents()
package tascript

import (
	"errors"

	"github.com/MoroZvlg/tascript/analysis"
	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/eval"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/parser"
	"github.com/MoroZvlg/tascript/token"
)

// Diagnostic re-exports diag.Diagnostic so callers can stay in tascript.*.
type Diagnostic = diag.Diagnostic

// Event is what a program emits via emit(...). Mirror of eval.Event and the
// §2 event record: { output, ts, value, data }. In slice 0 Ts is unset and
// Value is nil for structured outputs.
type Event struct {
	Output string
	Value  any
	Data   map[string]any
}

// Program is a compiled tascript module.
type Program struct {
	ast *ast.Program
}

// Inputs returns the declared input port names.
func (p *Program) Inputs() []string {
	var out []string
	for _, d := range p.ast.Decls {
		if in, ok := d.(*ast.InputDecl); ok {
			out = append(out, in.Name)
		}
	}
	return out
}

// Outputs returns the declared output port names.
func (p *Program) Outputs() []string {
	var out []string
	for _, d := range p.ast.Decls {
		if o, ok := d.(*ast.OutputDecl); ok {
			out = append(out, o.Name)
		}
	}
	return out
}

// Wiring is the host-side configuration passed to [Launch]. Slice 0 only
// knows that input ports may be enumerated. Real DataSource / Sink types
// land in later slices.
type Wiring struct {
	// InputPorts is the optional set of input port names the host has
	// prepared. Slice 0 does not require it to be non-empty; later slices
	// will validate against the program's declared ports (INPUT_NOT_WIRED).
	InputPorts map[string]struct{}
}

// Compile lexes, parses, and validates source. It returns the compiled
// Program along with any diagnostics. A non-nil error indicates the
// program is unusable (the diagnostics describe why).
func Compile(src []byte) (*Program, []Diagnostic, error) {
	return CompileFile(src, "")
}

// CompileFile is [Compile] with a filename used in diagnostic locations.
func CompileFile(src []byte, file string) (*Program, []Diagnostic, error) {
	lx := lexer.New(src, file)
	toks := lx.Tokenize()
	var diags []Diagnostic
	for _, le := range lx.Errors() {
		diags = append(diags, Diagnostic{
			Phase:    diag.PhaseParse,
			Category: diag.CatTopLevelForm,
			Pos:      le.Pos,
			Msg:      le.Msg,
		})
	}

	ps := parser.New(toks)
	prog := ps.Parse()
	diags = append(diags, ps.Diagnostics()...)
	diags = append(diags, analysis.Analyze(prog)...)

	if len(diags) > 0 {
		return nil, diags, errors.New("tascript: compilation failed")
	}
	return &Program{ast: prog}, diags, nil
}

// Runner is a launched program ready to be driven by the host.
type Runner struct {
	prog *Program
	eng  *eval.Engine
}

// Launch validates wiring against the program and returns a Runner.
func Launch(p *Program, w Wiring) (*Runner, error) {
	if p == nil {
		return nil, errors.New("tascript.Launch: nil program")
	}
	r := &Runner{prog: p, eng: eval.New(p.ast)}
	if d := r.eng.Prepare(); d != nil {
		return nil, *d
	}
	_ = w // slice 0: wiring fields are advisory only
	return r, nil
}

// Init executes the program's Init() function.
func (r *Runner) Init() error {
	if d := r.eng.RunInit(); d != nil {
		return *d
	}
	return nil
}

// Step executes the program's Run() function once.
func (r *Runner) Step() error {
	if d := r.eng.RunStep(); d != nil {
		return *d
	}
	return nil
}

// DrainEvents returns all events emitted since the last drain (or since
// Launch). The buffer is cleared by this call.
func (r *Runner) DrainEvents() []Event {
	raw := r.eng.DrainEvents()
	out := make([]Event, len(raw))
	for i, ev := range raw {
		data := make(map[string]any, len(ev.Data))
		for k, v := range ev.Data {
			data[k] = v
		}
		out[i] = Event{Output: ev.Output, Value: ev.Value, Data: data}
	}
	return out
}

// Pos re-export so callers can introspect Diagnostic.Pos without importing token.
type Pos = token.Pos
