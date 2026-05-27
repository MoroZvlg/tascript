// Package tascript is the public entrypoint for the tascript DSL.
//
// Surface:
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
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/token"
)

// Diagnostic re-exports diag.Diagnostic so callers can stay in tascript.*.
type Diagnostic = diag.Diagnostic

// Candle is a single OHLCV bar fed by a host DataSource.
type Candle = eval.Candle

// DataSource produces one candle per Runner.Step for a declared input port.
type DataSource = eval.DataSource

type Registry = registry.Registry
type TypeSpec = registry.TypeSpec
type HelperSpec = registry.HelperSpec
type IndicatorSpec = registry.IndicatorSpec
type HelperFunc = registry.HelperFunc
type HelperLookbackFunc = registry.HelperLookbackFunc
type Indicator = registry.Indicator
type IndicatorFactory = registry.IndicatorFactory
type ScalarIndicator = registry.ScalarIndicator
type ScalarIndicatorFactory = registry.ScalarIndicatorFactory
type Value = registry.Value
type Tuple = registry.Tuple

type Sink interface {
	Emit(Event) error
}

type Config struct {
	Registry       *registry.Registry
	ResourceLimits ResourceLimits
}

type ResourceLimits struct {
	MaxHistoryIndex        int
	MaxDiagnostics         int
	MaxStringLiteralLength int
	MaxRuntimeStringLength int
	MaxEmitKwargs          int
	MaxIdentLength         int
	MaxExprDepth           int
	MaxSourceBytes         int
}

func DefaultConfig() Config {
	return Config{
		Registry: registry.Default(),
		ResourceLimits: ResourceLimits{
			MaxHistoryIndex:        5000,
			MaxDiagnostics:         100,
			MaxStringLiteralLength: 4096,
			MaxRuntimeStringLength: 4096,
			MaxEmitKwargs:          32,
			MaxIdentLength:         128,
			MaxExprDepth:           64,
			MaxSourceBytes:         256 * 1024,
		},
	}
}

func NewRegistry() *registry.Registry {
	return registry.New()
}

// Event is what a program emits via emit(...). Mirror of eval.Event and the
// §2 event record: { output, ts, value, data }. Ts is not exposed yet and
// Value is nil for structured outputs.
type Event struct {
	Output string
	Value  any
	Data   map[string]any
}

// Program is a compiled tascript module.
type Program struct {
	ast      *ast.Program
	registry *registry.Registry
	limits   ResourceLimits
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

// Wiring is the host-side configuration passed to [Launch].
type Wiring struct {
	// InputPorts is the optional set of input port names the host has
	// prepared. DataSources carries real candle feeds; InputPorts is useful
	// for custom input types and placeholder ports that the program does not
	// read as CandleSeries values yet.
	InputPorts map[string]struct{}

	// DataSources maps declared input port names to candle sources.
	DataSources map[string]DataSource

	// Sinks maps declared output port names to host destinations. When nil,
	// outputs are collected only in the runner's in-memory event buffer.
	Sinks map[string]Sink

	// StrictOutputWiring requires every declared output to have a sink.
	StrictOutputWiring bool
}

// Compile lexes, parses, and validates source. It returns the compiled
// Program along with any diagnostics. A non-nil error indicates the
// program is unusable (the diagnostics describe why).
func Compile(src []byte) (*Program, []Diagnostic, error) {
	return CompileFile(src, "")
}

// CompileFile is [Compile] with a filename used in diagnostic locations.
func CompileFile(src []byte, file string) (*Program, []Diagnostic, error) {
	return CompileFileWithConfig(src, file, DefaultConfig())
}

func CompileWithConfig(src []byte, cfg Config) (*Program, []Diagnostic, error) {
	return CompileFileWithConfig(src, "", cfg)
}

func CompileFileWithConfig(src []byte, file string, cfg Config) (*Program, []Diagnostic, error) {
	cfg = normalizeConfig(cfg)
	reg := cfg.Registry.Clone()
	if cfg.ResourceLimits.MaxSourceBytes > 0 && len(src) > cfg.ResourceLimits.MaxSourceBytes {
		diags := []Diagnostic{{
			Phase: diag.PhaseParse, Category: diag.CatSourceSizeLimit,
			Pos: token.Pos{File: file, Line: 1, Column: 1},
			Msg: "source file exceeds configured size limit",
		}}
		return nil, diags, errors.New("tascript: compilation failed")
	}
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
	diags = append(diags, tokenLimitDiagnostics(toks, cfg.ResourceLimits)...)

	ps := parser.New(toks)
	prog := ps.Parse()
	diags = append(diags, ps.Diagnostics()...)
	diags = append(diags, analysis.Analyze(prog, reg, analysis.Options{
		MaxHistoryIndex: cfg.ResourceLimits.MaxHistoryIndex,
		MaxDiagnostics:  cfg.ResourceLimits.MaxDiagnostics,
		MaxEmitKwargs:   cfg.ResourceLimits.MaxEmitKwargs,
		MaxExprDepth:    cfg.ResourceLimits.MaxExprDepth,
	})...)
	diags = trimDiagnostics(diags, cfg.ResourceLimits.MaxDiagnostics)

	if len(diags) > 0 {
		return nil, diags, errors.New("tascript: compilation failed")
	}
	return &Program{ast: prog, registry: reg, limits: cfg.ResourceLimits}, diags, nil
}

func normalizeConfig(cfg Config) Config {
	def := DefaultConfig()
	if cfg.Registry == nil {
		cfg.Registry = def.Registry
	}
	if cfg.ResourceLimits.MaxHistoryIndex == 0 {
		cfg.ResourceLimits.MaxHistoryIndex = def.ResourceLimits.MaxHistoryIndex
	}
	if cfg.ResourceLimits.MaxDiagnostics == 0 {
		cfg.ResourceLimits.MaxDiagnostics = def.ResourceLimits.MaxDiagnostics
	}
	if cfg.ResourceLimits.MaxStringLiteralLength == 0 {
		cfg.ResourceLimits.MaxStringLiteralLength = def.ResourceLimits.MaxStringLiteralLength
	}
	if cfg.ResourceLimits.MaxRuntimeStringLength == 0 {
		cfg.ResourceLimits.MaxRuntimeStringLength = def.ResourceLimits.MaxRuntimeStringLength
	}
	if cfg.ResourceLimits.MaxEmitKwargs == 0 {
		cfg.ResourceLimits.MaxEmitKwargs = def.ResourceLimits.MaxEmitKwargs
	}
	if cfg.ResourceLimits.MaxIdentLength == 0 {
		cfg.ResourceLimits.MaxIdentLength = def.ResourceLimits.MaxIdentLength
	}
	if cfg.ResourceLimits.MaxExprDepth == 0 {
		cfg.ResourceLimits.MaxExprDepth = def.ResourceLimits.MaxExprDepth
	}
	if cfg.ResourceLimits.MaxSourceBytes == 0 {
		cfg.ResourceLimits.MaxSourceBytes = def.ResourceLimits.MaxSourceBytes
	}
	return cfg
}

func tokenLimitDiagnostics(toks []token.Token, limits ResourceLimits) []Diagnostic {
	var diags []Diagnostic
	for _, tok := range toks {
		switch tok.Kind {
		case token.IDENT:
			if limits.MaxIdentLength > 0 && len(tok.Literal) > limits.MaxIdentLength {
				diags = append(diags, Diagnostic{
					Phase: diag.PhaseParse, Category: diag.CatIdentLimit,
					Pos: tok.Pos, Msg: "identifier exceeds configured length limit",
				})
			}
		case token.STRING:
			if limits.MaxStringLiteralLength > 0 && len(tok.Literal) > limits.MaxStringLiteralLength {
				diags = append(diags, Diagnostic{
					Phase: diag.PhaseParse, Category: diag.CatStringLimit,
					Pos: tok.Pos, Msg: "string literal exceeds configured length limit",
				})
			}
		}
	}
	return diags
}

func trimDiagnostics(diags []Diagnostic, limit int) []Diagnostic {
	if limit <= 0 || len(diags) <= limit {
		return diags
	}
	return diags[:limit]
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
	if d := validateWiring(p, w); d != nil {
		return nil, *d
	}
	r := &Runner{prog: p, eng: eval.New(p.ast, w.DataSources, adaptSinks(w.Sinks), p.registry, eval.Options{
		MaxStringLength: p.limits.MaxRuntimeStringLength,
	})}
	if d := r.eng.Prepare(); d != nil {
		return nil, *d
	}
	return r, nil
}

type sinkAdapter struct {
	sink Sink
}

func (s sinkAdapter) Emit(ev eval.Event) error {
	return s.sink.Emit(convertEvent(ev))
}

func adaptSinks(sinks map[string]Sink) map[string]eval.Sink {
	if len(sinks) == 0 {
		return nil
	}
	out := make(map[string]eval.Sink, len(sinks))
	for name, sink := range sinks {
		if sink != nil {
			out[name] = sinkAdapter{sink: sink}
		}
	}
	return out
}

func validateWiring(p *Program, w Wiring) *Diagnostic {
	for _, d := range p.ast.Decls {
		in, ok := d.(*ast.InputDecl)
		if !ok {
			continue
		}
		if w.DataSources[in.Name] != nil {
			continue
		}
		if _, ok := w.InputPorts[in.Name]; ok {
			continue
		}
		return &Diagnostic{
			Phase: diag.PhaseLaunch, Category: diag.CatInputNotWired,
			Pos: in.NamePos, Msg: "input " + in.Name + " is not wired",
		}
	}

	if w.Sinks == nil && !w.StrictOutputWiring {
		return nil
	}
	for _, d := range p.ast.Decls {
		out, ok := d.(*ast.OutputDecl)
		if !ok {
			continue
		}
		if w.Sinks[out.Name] != nil {
			continue
		}
		return &Diagnostic{
			Phase: diag.PhaseLaunch, Category: diag.CatOutputNotWired,
			Pos: out.NamePos, Msg: "output " + out.Name + " is not wired",
		}
	}
	return nil
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
		out[i] = convertEvent(ev)
	}
	return out
}

func convertEvent(ev eval.Event) Event {
	data := make(map[string]any, len(ev.Data))
	for k, v := range ev.Data {
		data[k] = v
	}
	return Event{Output: ev.Output, Value: ev.Value, Data: data}
}

// Pos re-export so callers can introspect Diagnostic.Pos without importing token.
type Pos = token.Pos
