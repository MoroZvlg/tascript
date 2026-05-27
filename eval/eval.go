// Package eval drives a compiled tascript program: it walks the AST,
// resolves identifiers, and produces Events. It does NOT manage data
// sources or sinks — those live in the host runtime.
package eval

import (
	"fmt"
	"math"

	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/token"
)

// Value is the union of runtime values supported by the implemented slices.
type Value any

// Candle is a single OHLCV bar fed by the host runtime.
type Candle = registry.Candle

// DataSource produces one candle per Runner.Step for an input port.
type DataSource interface {
	NextCandle() (Candle, error)
}

type Options struct {
	MaxStringLength int
}

type CandleSeries struct {
	current Candle
	ready   bool
	seq     int
	history []Candle
}

func (cs *CandleSeries) push(c Candle) {
	cs.current = c
	cs.ready = true
	cs.seq++
	cs.history = append(cs.history, c)
}

func (cs *CandleSeries) candleAt(n int) (Candle, bool) {
	if n < 0 || n >= len(cs.history) {
		return Candle{}, false
	}
	return cs.history[len(cs.history)-1-n], true
}

type numberSeries struct {
	source *CandleSeries
	field  string
}

func (s *numberSeries) valueAt(n int) (float64, bool) {
	c, ok := s.source.candleAt(n)
	if !ok {
		return 0, false
	}
	switch s.field {
	case "open", "opens":
		return c.Open, true
	case "high", "highs":
		return c.High, true
	case "low", "lows":
		return c.Low, true
	case "close", "closes":
		return c.Close, true
	case "volume", "volumes":
		return c.Volume, true
	case "hl2":
		return (c.High + c.Low) / 2, true
	case "hlc3":
		return (c.High + c.Low + c.Close) / 3, true
	default:
		return 0, false
	}
}

func (s *numberSeries) Current() (float64, error) {
	v, ok := s.valueAt(0)
	if !ok {
		return 0, fmt.Errorf("Series has no current value")
	}
	return v, nil
}

func (s *numberSeries) History(n int) (float64, error) {
	v, ok := s.valueAt(n)
	if !ok {
		return 0, fmt.Errorf("Series history[%d] is out of range", n)
	}
	return v, nil
}

// Event is a single emit() call's product. Mirrors the public tascript.Event
// and the §2 event record: { output, ts, value, data }. Timestamp support is
// not exposed yet and Value is nil for structured outputs.
type Event struct {
	Output string
	Value  Value
	Data   map[string]Value
}

type Sink interface {
	Emit(Event) error
}

// Engine carries per-program execution state: top-level bindings and the
// outbound event buffer.
type Engine struct {
	prog       *ast.Program
	constBinds map[string]Value
	inputTypes map[string]string
	inputs     map[string]*CandleSeries
	sources    map[string]DataSource
	sinks      map[string]Sink
	state      map[string]Value
	registry   *registry.Registry
	indicators map[string]*indicatorState
	options    Options
	events     []Event
}

type indicatorState struct {
	receiverKey    string
	candleReceiver *CandleSeries
	seriesReceiver registry.Series
	candleInstance registry.Indicator
	scalarInstance registry.ScalarIndicator
	argsKey        string
	lastSeq        int
	value          Value
	history        []Value
	ready          bool
}

type indicatorSeries struct {
	state *indicatorState
}

type indicatorTuple struct {
	state *indicatorState
}

type tupleElementSeries struct {
	state *indicatorState
	index int
}

func (s *indicatorSeries) Current() (float64, error) {
	if s == nil || s.state == nil || !s.state.ready {
		return 0, fmt.Errorf("indicator has no current value")
	}
	return numericIndicatorValue(s.state.value)
}

func (s *indicatorSeries) History(n int) (float64, error) {
	if s == nil || s.state == nil || n < 0 || n >= len(s.state.history) {
		return 0, fmt.Errorf("indicator history[%d] is out of range", n)
	}
	return numericIndicatorValue(s.state.history[len(s.state.history)-1-n])
}

func (s *tupleElementSeries) Current() (float64, error) {
	if s == nil || s.state == nil || !s.state.ready {
		return 0, fmt.Errorf("tuple element has no current value")
	}
	return tupleElement(s.state.value, s.index)
}

func (s *tupleElementSeries) History(n int) (float64, error) {
	if s == nil || s.state == nil || n < 0 || n >= len(s.state.history) {
		return 0, fmt.Errorf("tuple element history[%d] is out of range", n)
	}
	return tupleElement(s.state.history[len(s.state.history)-1-n], s.index)
}

func New(prog *ast.Program, sources map[string]DataSource, sinks map[string]Sink, reg *registry.Registry, opts Options) *Engine {
	return &Engine{
		prog:       prog,
		constBinds: map[string]Value{},
		inputTypes: map[string]string{},
		inputs:     map[string]*CandleSeries{},
		sources:    sources,
		sinks:      sinks,
		state:      map[string]Value{},
		registry:   reg.Clone(),
		indicators: map[string]*indicatorState{},
		options:    opts,
	}
}

// Prepare evaluates the top-level declarations (constants, input ports).
// Called once before [Engine.RunInit].
func (e *Engine) Prepare() *diag.Diagnostic {
	for _, d := range e.prog.Decls {
		switch x := d.(type) {
		case *ast.ConstDecl:
			v, err := e.evalLiteral(x.Value)
			if err != nil {
				return err
			}
			e.constBinds[x.Name] = v
		case *ast.InputDecl:
			e.inputTypes[x.Name] = x.Type
			if x.Type == "CandleSeries" {
				e.inputs[x.Name] = &CandleSeries{}
			}
		}
	}
	return nil
}

// RunInit executes the program's Init() function exactly once.
func (e *Engine) RunInit() *diag.Diagnostic { return e.runFunc(e.prog.Init) }

// RunStep executes the program's Run() function once.
func (e *Engine) RunStep() *diag.Diagnostic {
	for name, src := range e.sources {
		series, ok := e.inputs[name]
		if !ok {
			continue
		}
		c, err := src.NextCandle()
		if err != nil {
			return &diag.Diagnostic{
				Phase: diag.PhaseRuntime, Category: diag.CatTypeMismatch,
				Msg: fmt.Sprintf("input %q failed to produce candle: %v", name, err),
			}
		}
		series.push(c)
	}
	if err := e.advanceIndicators(e.prog.Run.Body); err != nil {
		return err
	}
	return e.runFunc(e.prog.Run)
}

// DrainEvents returns and clears the emit() buffer.
func (e *Engine) DrainEvents() []Event {
	out := e.events
	e.events = nil
	return out
}

func (e *Engine) runFunc(fn *ast.FuncDecl) *diag.Diagnostic {
	if fn == nil {
		return nil
	}
	for _, s := range fn.Body {
		if err := e.runStmt(s); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) advanceIndicators(stmts []ast.Stmt) *diag.Diagnostic {
	for _, s := range stmts {
		if err := e.advanceStmtIndicators(s); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) advanceStmtIndicators(s ast.Stmt) *diag.Diagnostic {
	switch x := s.(type) {
	case *ast.EmitStmt:
		if x.Value != nil {
			if err := e.advanceExprIndicators(x.Value); err != nil {
				return err
			}
		}
		for _, kw := range x.Kwargs {
			if err := e.advanceExprIndicators(kw.Value); err != nil {
				return err
			}
		}
	case *ast.AssignStmt:
		return e.advanceExprIndicators(x.Value)
	case *ast.IfStmt:
		if err := e.advanceExprIndicators(x.Condition); err != nil {
			return err
		}
		if err := e.advanceIndicators(x.Consequence); err != nil {
			return err
		}
		return e.advanceIndicators(x.Alternative)
	case *ast.ExprStmt:
		return e.advanceExprIndicators(x.Expr)
	}
	return nil
}

func (e *Engine) advanceExprIndicators(x ast.Expr) *diag.Diagnostic {
	switch v := x.(type) {
	case *ast.UnaryExpr:
		return e.advanceExprIndicators(v.Right)
	case *ast.BinaryExpr:
		if err := e.advanceExprIndicators(v.Left); err != nil {
			return err
		}
		return e.advanceExprIndicators(v.Right)
	case *ast.MemberExpr:
		return e.advanceExprIndicators(v.Object)
	case *ast.IndexExpr:
		return e.advanceExprIndicators(v.Object)
	case *ast.CallExpr:
		if e.isIndicatorCall(v) {
			if _, err := e.evalCall(v); err != nil {
				return err
			}
		}
		for _, arg := range v.Args {
			if err := e.advanceExprIndicators(arg.Value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *Engine) isIndicatorCall(x *ast.CallExpr) bool {
	member, ok := x.Callee.(*ast.MemberExpr)
	if !ok {
		return false
	}
	spec, ok := e.registry.Indicator(member.Name)
	return ok && (spec.Build != nil || spec.BuildScalar != nil)
}

func (e *Engine) runStmt(s ast.Stmt) *diag.Diagnostic {
	switch x := s.(type) {
	case *ast.EmitStmt:
		return e.runEmit(x)
	case *ast.IfStmt:
		return e.runIf(x)
	case *ast.AssignStmt:
		return e.runAssign(x)
	default:
		return &diag.Diagnostic{
			Phase: diag.PhaseRuntime, Category: diag.CatNotImplemented,
			Pos: s.Pos(), Msg: fmt.Sprintf("statement %T not supported in this slice", s),
		}
	}
}

func (e *Engine) runEmit(em *ast.EmitStmt) *diag.Diagnostic {
	ev := Event{Output: em.Output, Data: map[string]Value{}}
	if em.Value != nil {
		v, err := e.evalExpr(em.Value)
		if err != nil {
			return err
		}
		ev.Value, err = e.scalarValue(v, em.Value.Pos())
		if err != nil {
			return err
		}
	}
	for _, kw := range em.Kwargs {
		v, err := e.evalExpr(kw.Value)
		if err != nil {
			return err
		}
		ev.Data[kw.Name], err = e.scalarValue(v, kw.Value.Pos())
		if err != nil {
			return err
		}
	}
	e.events = append(e.events, ev)
	if sink := e.sinks[em.Output]; sink != nil {
		if err := sink.Emit(ev); err != nil {
			return &diag.Diagnostic{
				Phase: diag.PhaseRuntime, Category: diag.CatEmitPayload,
				Pos: em.CallPos, Msg: fmt.Sprintf("output %q sink failed: %v", em.Output, err),
			}
		}
	}
	return nil
}

func (e *Engine) runAssign(stmt *ast.AssignStmt) *diag.Diagnostic {
	member, ok := stmt.Target.(*ast.MemberExpr)
	if !ok {
		return &diag.Diagnostic{
			Phase: diag.PhaseRuntime, Category: diag.CatNotImplemented,
			Pos: stmt.Target.Pos(), Msg: "only state.* assignment is supported in this slice",
		}
	}
	if !isIdent(member.Object, "state") {
		return &diag.Diagnostic{
			Phase: diag.PhaseRuntime, Category: diag.CatNotImplemented,
			Pos: stmt.Target.Pos(), Msg: "only state.* assignment is supported in this slice",
		}
	}
	val, err := e.evalExpr(stmt.Value)
	if err != nil {
		return err
	}
	e.state[member.Name] = val
	return nil
}

func (e *Engine) runIf(stmt *ast.IfStmt) *diag.Diagnostic {
	cond, err := e.evalExpr(stmt.Condition)
	if err != nil {
		return err
	}
	b, ok := asBool(cond)
	if !ok {
		return typeErr(stmt.Condition.Pos(), "if condition must be Bool")
	}
	body := stmt.Alternative
	if b {
		body = stmt.Consequence
	}
	for _, s := range body {
		if err := e.runStmt(s); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) evalExpr(x ast.Expr) (Value, *diag.Diagnostic) {
	switch v := x.(type) {
	case *ast.NumberLit:
		return v.Val, nil
	case *ast.StringLit:
		return v.Val, nil
	case *ast.BoolLit:
		return v.Val, nil
	case *ast.Ident:
		if val, ok := e.constBinds[v.Name]; ok {
			return val, nil
		}
		if input, ok := e.inputs[v.Name]; ok {
			return input, nil
		}
		return nil, &diag.Diagnostic{
			Phase: diag.PhaseRuntime, Category: diag.CatTypeMismatch,
			Pos: v.P, Msg: fmt.Sprintf("undefined identifier %q", v.Name),
		}
	case *ast.UnaryExpr:
		return e.evalUnary(v)
	case *ast.BinaryExpr:
		return e.evalBinary(v)
	case *ast.MemberExpr:
		return e.evalMember(v)
	case *ast.CallExpr:
		return e.evalCall(v)
	case *ast.IndexExpr:
		return e.evalIndex(v)
	}
	return nil, &diag.Diagnostic{
		Phase: diag.PhaseRuntime, Category: diag.CatNotImplemented,
		Pos: x.Pos(), Msg: fmt.Sprintf("expression %T not supported in this slice", x),
	}
}

func (e *Engine) evalUnary(x *ast.UnaryExpr) (Value, *diag.Diagnostic) {
	right, err := e.evalExpr(x.Right)
	if err != nil {
		return nil, err
	}
	switch x.Op {
	case token.MINUS:
		n, ok := asNumber(right)
		if !ok {
			return nil, typeErr(x.OpPos, "unary '-' requires Number")
		}
		return -n, nil
	case token.BANG:
		b, ok := asBool(right)
		if !ok {
			return nil, typeErr(x.OpPos, "unary '!' requires Bool")
		}
		return !b, nil
	default:
		return nil, &diag.Diagnostic{
			Phase: diag.PhaseRuntime, Category: diag.CatNotImplemented,
			Pos: x.OpPos, Msg: fmt.Sprintf("unary operator %s not supported in this slice", x.Op),
		}
	}
}

func (e *Engine) evalBinary(x *ast.BinaryExpr) (Value, *diag.Diagnostic) {
	switch x.Op {
	case token.AND:
		return e.evalAnd(x)
	case token.OR:
		return e.evalOr(x)
	}

	left, err := e.evalExpr(x.Left)
	if err != nil {
		return nil, err
	}
	right, err := e.evalExpr(x.Right)
	if err != nil {
		return nil, err
	}
	switch x.Op {
	case token.EQ, token.NEQ:
		return evalEquality(x.Op, x.OpPos, left, right)
	case token.LT, token.LTE, token.GT, token.GTE:
		return evalCompare(x.Op, x.OpPos, left, right)
	}

	l, ok := asNumber(left)
	if !ok {
		return nil, typeErr(x.OpPos, "left operand must be Number")
	}
	r, ok := asNumber(right)
	if !ok {
		return nil, typeErr(x.OpPos, "right operand must be Number")
	}
	switch x.Op {
	case token.PLUS:
		return l + r, nil
	case token.MINUS:
		return l - r, nil
	case token.ASTERISK:
		return l * r, nil
	case token.SLASH:
		return l / r, nil
	case token.PERCENT:
		return math.Mod(l, r), nil
	default:
		return nil, &diag.Diagnostic{
			Phase: diag.PhaseRuntime, Category: diag.CatNotImplemented,
			Pos: x.OpPos, Msg: fmt.Sprintf("binary operator %s not supported in this slice", x.Op),
		}
	}
}

func (e *Engine) evalAnd(x *ast.BinaryExpr) (Value, *diag.Diagnostic) {
	left, err := e.evalExpr(x.Left)
	if err != nil {
		return nil, err
	}
	l, ok := asBool(left)
	if !ok {
		return nil, typeErr(x.OpPos, "left operand of && must be Bool")
	}
	if !l {
		return false, nil
	}
	right, err := e.evalExpr(x.Right)
	if err != nil {
		return nil, err
	}
	r, ok := asBool(right)
	if !ok {
		return nil, typeErr(x.OpPos, "right operand of && must be Bool")
	}
	return r, nil
}

func (e *Engine) evalOr(x *ast.BinaryExpr) (Value, *diag.Diagnostic) {
	left, err := e.evalExpr(x.Left)
	if err != nil {
		return nil, err
	}
	l, ok := asBool(left)
	if !ok {
		return nil, typeErr(x.OpPos, "left operand of || must be Bool")
	}
	if l {
		return true, nil
	}
	right, err := e.evalExpr(x.Right)
	if err != nil {
		return nil, err
	}
	r, ok := asBool(right)
	if !ok {
		return nil, typeErr(x.OpPos, "right operand of || must be Bool")
	}
	return r, nil
}

func (e *Engine) evalMember(x *ast.MemberExpr) (Value, *diag.Diagnostic) {
	if isIdent(x.Object, "state") {
		val, ok := e.state[x.Name]
		if !ok {
			return nil, &diag.Diagnostic{
				Phase: diag.PhaseRuntime, Category: diag.CatStateUnset,
				Pos: x.NamePos, Msg: fmt.Sprintf("state.%s is unset", x.Name),
			}
		}
		return val, nil
	}

	obj, err := e.evalExpr(x.Object)
	if err != nil {
		return nil, err
	}
	if c, ok := obj.(Candle); ok {
		return candleMember(c, x.Name, x.NamePos)
	}
	series, ok := obj.(*CandleSeries)
	if !ok {
		return nil, typeErr(x.NamePos, fmt.Sprintf("member %q requires CandleSeries receiver", x.Name))
	}
	if !series.ready {
		return nil, &diag.Diagnostic{
			Phase: diag.PhaseRuntime, Category: diag.CatTypeMismatch,
			Pos: x.NamePos, Msg: "CandleSeries has no current candle; wire a DataSource before reading it",
		}
	}
	switch x.Name {
	case "opens", "highs", "lows", "closes", "volumes", "hl2", "hlc3":
		return &numberSeries{source: series, field: x.Name}, nil
	case "open", "high", "low", "close", "volume":
		return candleMember(series.current, x.Name, x.NamePos)
	default:
		return nil, typeErr(x.NamePos, fmt.Sprintf("unknown CandleSeries member %q", x.Name))
	}
}

func (e *Engine) evalIndex(x *ast.IndexExpr) (Value, *diag.Diagnostic) {
	idxValue, err := e.evalExpr(x.Index)
	if err != nil {
		return nil, err
	}
	idxNumber, ok := asNumber(idxValue)
	if !ok || idxNumber != math.Trunc(idxNumber) || idxNumber < 0 {
		return nil, typeErr(x.Index.Pos(), "history index must be a non-negative integer")
	}
	idx := int(idxNumber)
	obj, err := e.evalExpr(x.Object)
	if err != nil {
		return nil, err
	}
	switch v := obj.(type) {
	case *CandleSeries:
		c, ok := v.candleAt(idx)
		if !ok {
			return nil, historyErr(x.LPos, fmt.Sprintf("CandleSeries history[%d] is out of range", idx))
		}
		return c, nil
	case registry.Series:
		n, seriesErr := v.History(idx)
		if seriesErr != nil {
			return nil, historyErr(x.LPos, seriesErr.Error())
		}
		return n, nil
	case *indicatorTuple:
		n, tupleErr := v.element(idx)
		if tupleErr != nil {
			return nil, typeErr(x.LPos, tupleErr.Error())
		}
		return n, nil
	case registry.Tuple:
		if idx >= len(v) {
			return nil, typeErr(x.LPos, fmt.Sprintf("Tuple index %d is out of range", idx))
		}
		return v[idx], nil
	default:
		return nil, typeErr(x.LPos, "indexing requires Series, CandleSeries, or Tuple")
	}
}

func (e *Engine) evalCall(x *ast.CallExpr) (Value, *diag.Diagnostic) {
	member, ok := x.Callee.(*ast.MemberExpr)
	if !ok {
		return nil, &diag.Diagnostic{
			Phase: diag.PhaseRuntime, Category: diag.CatNotImplemented,
			Pos: x.Pos(), Msg: "only registered namespace helpers are supported in this slice",
		}
	}
	ns, ok := member.Object.(*ast.Ident)
	if ok {
		if spec, ok := e.registry.Helper(ns.Name, member.Name); ok && spec.Eval != nil {
			return e.evalHelper(x, spec)
		}
	}

	if spec, ok := e.registry.Indicator(member.Name); ok && (spec.Build != nil || spec.BuildScalar != nil) {
		return e.evalIndicator(x, member, spec)
	}

	return nil, &diag.Diagnostic{
		Phase: diag.PhaseRuntime, Category: diag.CatNotImplemented,
		Pos: x.Pos(), Msg: fmt.Sprintf("call %s is not registered", member.Name),
	}
}

func (e *Engine) evalHelper(x *ast.CallExpr, spec registry.HelperSpec) (Value, *diag.Diagnostic) {
	if err := registry.ValidateArgCount(spec.Namespace+"."+spec.Name, spec.MinArgs, spec.MaxArgs, len(x.Args)); err != nil {
		return nil, typeErr(x.LPos, err.Error())
	}
	args := make([]registry.Value, 0, len(x.Args))
	for _, arg := range x.Args {
		if arg.Name != "" {
			return nil, typeErr(arg.NamePos, fmt.Sprintf("%s.%s does not accept keyword arguments", spec.Namespace, spec.Name))
		}
		val, err := e.evalExpr(arg.Value)
		if err != nil {
			return nil, err
		}
		args = append(args, val)
	}
	out, callErr := spec.Eval(args)
	if callErr != nil {
		return nil, typeErr(x.LPos, callErr.Error())
	}
	return out, nil
}

func (e *Engine) evalIndicator(x *ast.CallExpr, member *ast.MemberExpr, spec registry.IndicatorSpec) (Value, *diag.Diagnostic) {
	receiverVal, err := e.evalExpr(member.Object)
	if err != nil {
		return nil, err
	}
	if err := registry.ValidateArgCount(spec.Name, spec.MinArgs, spec.MaxArgs, len(x.Args)); err != nil {
		return nil, typeErr(x.LPos, err.Error())
	}
	args := make([]registry.Value, 0, len(x.Args))
	for _, arg := range x.Args {
		if arg.Name != "" {
			return nil, typeErr(arg.NamePos, fmt.Sprintf("indicator %s does not accept keyword arguments in this slice", spec.Name))
		}
		val, err := e.evalExpr(arg.Value)
		if err != nil {
			return nil, err
		}
		args = append(args, val)
	}

	argsKey := fmt.Sprintf("%#v", args)
	key, kind, keyErr := receiverKey(receiverVal)
	if keyErr != nil {
		return nil, typeErr(member.NamePos, fmt.Sprintf("indicator %s receiver is not supported: %v", spec.Name, keyErr))
	}
	key = fmt.Sprintf("%s:%s:%s", key, spec.Name, argsKey)
	state := e.indicators[key]
	if state == nil || state.receiverKey != key || state.argsKey != argsKey {
		state = &indicatorState{
			receiverKey: key,
			argsKey:     argsKey,
			lastSeq:     -1,
		}
		switch kind {
		case "candle":
			receiver := receiverVal.(*CandleSeries)
			if !receiver.ready {
				return nil, typeErr(member.NamePos, fmt.Sprintf("indicator %s receiver has no current candle", spec.Name))
			}
			if spec.Build == nil {
				return nil, typeErr(member.NamePos, fmt.Sprintf("indicator %s is not callable on CandleSeries", spec.Name))
			}
			instance, buildErr := spec.Build(args)
			if buildErr != nil {
				return nil, typeErr(x.LPos, buildErr.Error())
			}
			state.candleReceiver = receiver
			state.candleInstance = instance
		case "series":
			series := receiverVal.(registry.Series)
			if !spec.Scalar || spec.BuildScalar == nil {
				return nil, typeErr(member.NamePos, fmt.Sprintf("indicator %s is not callable on Series", spec.Name))
			}
			instance, buildErr := spec.BuildScalar(args)
			if buildErr != nil {
				return nil, typeErr(x.LPos, buildErr.Error())
			}
			state.seriesReceiver = series
			state.scalarInstance = instance
		}
		e.indicators[key] = state
	}
	seq, seqErr := state.seq()
	if seqErr != nil {
		return nil, typeErr(member.NamePos, fmt.Sprintf("indicator %s receiver is not ready: %v", spec.Name, seqErr))
	}
	if state.lastSeq != seq {
		value, nextErr := state.next()
		if nextErr != nil {
			return nil, typeErr(x.LPos, nextErr.Error())
		}
		state.value = value
		state.history = append(state.history, value)
		state.ready = true
		state.lastSeq = seq
	}
	if !state.ready {
		return nil, typeErr(x.LPos, fmt.Sprintf("indicator %s has no value yet", spec.Name))
	}
	if isTupleValue(state.value) {
		return &indicatorTuple{state: state}, nil
	}
	return &indicatorSeries{state: state}, nil
}

func (s *indicatorState) seq() (int, error) {
	if s.candleReceiver != nil {
		if !s.candleReceiver.ready {
			return 0, fmt.Errorf("CandleSeries has no current candle")
		}
		return s.candleReceiver.seq, nil
	}
	if s.seriesReceiver != nil {
		return seriesSeq(s.seriesReceiver)
	}
	return 0, fmt.Errorf("missing receiver")
}

func (s *indicatorState) next() (Value, error) {
	if s.candleReceiver != nil {
		return s.candleInstance.NextCandle(s.candleReceiver.current)
	}
	if s.seriesReceiver != nil {
		n, err := s.seriesReceiver.Current()
		if err != nil {
			return nil, err
		}
		return s.scalarInstance.NextNumber(n)
	}
	return nil, fmt.Errorf("missing receiver")
}

func receiverKey(v Value) (string, string, error) {
	switch x := v.(type) {
	case *CandleSeries:
		return fmt.Sprintf("candle:%p", x), "candle", nil
	case *numberSeries:
		return fmt.Sprintf("series:candle:%p:%s", x.source, x.field), "series", nil
	case *indicatorSeries:
		return fmt.Sprintf("series:indicator:%p", x.state), "series", nil
	case *tupleElementSeries:
		return fmt.Sprintf("series:tuple:%p:%d", x.state, x.index), "series", nil
	case registry.Series:
		return "", "", fmt.Errorf("custom Series receivers need a stable runtime identity")
	default:
		return "", "", fmt.Errorf("got %T", v)
	}
}

func seriesSeq(s registry.Series) (int, error) {
	switch x := s.(type) {
	case *numberSeries:
		if !x.source.ready {
			return 0, fmt.Errorf("Series has no current value")
		}
		return x.source.seq, nil
	case *indicatorSeries:
		if x.state == nil || !x.state.ready {
			return 0, fmt.Errorf("indicator Series has no current value")
		}
		return x.state.lastSeq, nil
	case *tupleElementSeries:
		if x.state == nil || !x.state.ready {
			return 0, fmt.Errorf("tuple element Series has no current value")
		}
		return x.state.lastSeq, nil
	default:
		return 0, fmt.Errorf("Series receiver does not expose a sequence")
	}
}

func asNumber(v Value) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case registry.Series:
		n, err := x.Current()
		return n, err == nil
	default:
		return 0, false
	}
}

func asBool(v Value) (bool, bool) {
	b, ok := v.(bool)
	return b, ok
}

func evalEquality(op token.Kind, pos token.Pos, left, right Value) (Value, *diag.Diagnostic) {
	switch l := left.(type) {
	case float64:
		r, ok := right.(float64)
		if !ok {
			return nil, typeErr(pos, "equality operands must have the same type")
		}
		return applyEquality(op, l == r), nil
	case string:
		r, ok := right.(string)
		if !ok {
			return nil, typeErr(pos, "equality operands must have the same type")
		}
		return applyEquality(op, l == r), nil
	case bool:
		r, ok := right.(bool)
		if !ok {
			return nil, typeErr(pos, "equality operands must have the same type")
		}
		return applyEquality(op, l == r), nil
	default:
		return nil, typeErr(pos, "equality operands must be scalar values")
	}
}

func evalCompare(op token.Kind, pos token.Pos, left, right Value) (Value, *diag.Diagnostic) {
	l, ok := asNumber(left)
	if !ok {
		return nil, typeErr(pos, "comparison operands must be Number")
	}
	r, ok := asNumber(right)
	if !ok {
		return nil, typeErr(pos, "comparison operands must be Number")
	}
	switch op {
	case token.LT:
		return l < r, nil
	case token.LTE:
		return l <= r, nil
	case token.GT:
		return l > r, nil
	case token.GTE:
		return l >= r, nil
	default:
		return nil, typeErr(pos, fmt.Sprintf("unsupported comparison operator %s", op))
	}
}

func applyEquality(op token.Kind, equal bool) bool {
	if op == token.NEQ {
		return !equal
	}
	return equal
}

func isIdent(x ast.Expr, name string) bool {
	id, ok := x.(*ast.Ident)
	return ok && id.Name == name
}

func typeErr(pos token.Pos, msg string) *diag.Diagnostic {
	return &diag.Diagnostic{Phase: diag.PhaseRuntime, Category: diag.CatTypeMismatch, Pos: pos, Msg: msg}
}

func historyErr(pos token.Pos, msg string) *diag.Diagnostic {
	return &diag.Diagnostic{Phase: diag.PhaseRuntime, Category: diag.CatHistoryOutOfRange, Pos: pos, Msg: msg}
}

func numericIndicatorValue(v Value) (float64, error) {
	n, ok := v.(float64)
	if !ok {
		return 0, fmt.Errorf("indicator value must be Number")
	}
	return n, nil
}

func isTupleValue(v Value) bool {
	switch v.(type) {
	case registry.Tuple, []registry.Value, []Value:
		return true
	default:
		return false
	}
}

func (t *indicatorTuple) element(index int) (Value, error) {
	if t == nil || t.state == nil || !t.state.ready {
		return nil, fmt.Errorf("Tuple has no current value")
	}
	tuple, err := asTuple(t.state.value)
	if err != nil {
		return nil, err
	}
	if index >= len(tuple) {
		return nil, fmt.Errorf("Tuple index %d is out of range", index)
	}
	return &tupleElementSeries{state: t.state, index: index}, nil
}

func tupleElement(v Value, index int) (float64, error) {
	tuple, err := asTuple(v)
	if err != nil {
		return 0, err
	}
	if index >= len(tuple) {
		return 0, fmt.Errorf("Tuple index %d is out of range", index)
	}
	n, ok := tuple[index].(float64)
	if !ok {
		return 0, fmt.Errorf("Tuple element %d must be Number", index)
	}
	return n, nil
}

func asTuple(v Value) (registry.Tuple, error) {
	switch x := v.(type) {
	case registry.Tuple:
		return x, nil
	case []registry.Value:
		return registry.Tuple(x), nil
	case []Value:
		out := make(registry.Tuple, len(x))
		for i, v := range x {
			out[i] = v
		}
		return out, nil
	default:
		return nil, fmt.Errorf("value is not a Tuple")
	}
}

func candleMember(c Candle, name string, pos token.Pos) (Value, *diag.Diagnostic) {
	switch name {
	case "open":
		return c.Open, nil
	case "high":
		return c.High, nil
	case "low":
		return c.Low, nil
	case "close":
		return c.Close, nil
	case "volume":
		return c.Volume, nil
	default:
		return nil, typeErr(pos, fmt.Sprintf("unknown Candle member %q", name))
	}
}

func (e *Engine) scalarValue(v Value, pos token.Pos) (Value, *diag.Diagnostic) {
	switch x := v.(type) {
	case registry.Series:
		n, err := x.Current()
		if err != nil {
			return nil, historyErr(pos, err.Error())
		}
		return n, nil
	case string:
		if e.options.MaxStringLength > 0 && len(x) > e.options.MaxStringLength {
			return nil, &diag.Diagnostic{
				Phase: diag.PhaseRuntime, Category: diag.CatStringLimit,
				Pos: pos, Msg: "string value exceeds configured length limit",
			}
		}
		return x, nil
	case *CandleSeries, Candle, *indicatorTuple, registry.Tuple:
		return nil, typeErr(pos, "value is not serializable in emit payload")
	default:
		return v, nil
	}
}

func (e *Engine) evalLiteral(x ast.Expr) (Value, *diag.Diagnostic) {
	switch v := x.(type) {
	case *ast.NumberLit:
		return v.Val, nil
	case *ast.StringLit:
		return v.Val, nil
	}
	return nil, &diag.Diagnostic{
		Phase: diag.PhaseParse, Category: diag.CatTopLevelForm,
		Pos: x.Pos(), Msg: "top-level constant must be a literal in this slice",
	}
}
