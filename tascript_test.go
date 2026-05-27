package tascript_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/MoroZvlg/tascript"
)

type wantEvent struct {
	Output string         `json:"output"`
	Value  any            `json:"value"`
	Data   map[string]any `json:"data"`
}

type wantDiag struct {
	Phase    string `json:"phase"`
	Category string `json:"category"`
}

func TestSlice0_Positive_Greeting(t *testing.T) {
	src := readFile(t, "testdata/slice0/positive/greeting.tas")
	prog, diags, err := tascript.Compile(src)
	if err != nil {
		t.Fatalf("compile: %v\ndiags: %#v", err, diags)
	}
	if got, want := prog.Outputs(), []string{"alerts"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Outputs() = %v, want %v", got, want)
	}
	if got := prog.Inputs(); len(got) != 0 {
		t.Errorf("Inputs() = %v, want []", got)
	}

	runner, err := tascript.Launch(prog, tascript.Wiring{})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if err := runner.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := runner.Step(); err != nil {
		t.Fatalf("step: %v", err)
	}

	got := runner.DrainEvents()
	want := loadEvents(t, "testdata/slice0/positive/greeting.events.json")
	if len(got) != len(want) {
		t.Fatalf("event count: got %d, want %d (events=%#v)", len(got), len(want), got)
	}
	for i := range got {
		if got[i].Output != want[i].Output {
			t.Errorf("event[%d].Output = %q, want %q", i, got[i].Output, want[i].Output)
		}
		if !reflect.DeepEqual(got[i].Value, want[i].Value) {
			t.Errorf("event[%d].Value = %#v, want %#v", i, got[i].Value, want[i].Value)
		}
		if !reflect.DeepEqual(got[i].Data, want[i].Data) {
			t.Errorf("event[%d].Data = %#v, want %#v", i, got[i].Data, want[i].Data)
		}
	}

	// Second Step → second event.
	if err := runner.Step(); err != nil {
		t.Fatalf("step 2: %v", err)
	}
	if got := runner.DrainEvents(); len(got) != 1 {
		t.Errorf("second step events = %d, want 1", len(got))
	}
}

func TestSlice0_InputDeclaration(t *testing.T) {
	src := []byte(`input btc: CandleSeries

output alerts {}

function Init() {}
function Run() {
  emit(alerts)
}
`)
	prog, diags, err := tascript.Compile(src)
	if err != nil {
		t.Fatalf("compile: %v (diags=%#v)", err, diags)
	}
	if got, want := prog.Inputs(), []string{"btc"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Inputs() = %v, want %v", got, want)
	}

	r, err := tascript.Launch(prog, tascript.Wiring{
		InputPorts: map[string]struct{}{"btc": {}},
	})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if err := r.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step: %v", err)
	}
	if got := r.DrainEvents(); len(got) != 1 || got[0].Output != "alerts" {
		t.Errorf("events = %#v, want one alerts event", got)
	}
}

func TestSlice0_NumericConstant(t *testing.T) {
	src := []byte(`THRESHOLD = 42

output alerts {
  t: Number
}

function Init() {}
function Run() {
  emit(alerts, t=THRESHOLD)
}
`)
	prog, diags, err := tascript.Compile(src)
	if err != nil {
		t.Fatalf("compile: %v (diags=%#v)", err, diags)
	}
	r, _ := tascript.Launch(prog, tascript.Wiring{})
	_ = r.Init()
	_ = r.Step()
	events := r.DrainEvents()
	if len(events) != 1 || events[0].Data["t"] != float64(42) {
		t.Errorf("events = %#v", events)
	}
}

func TestSlice0_ValueOutput(t *testing.T) {
	src := []byte(`output logs: String

function Init() {}
function Run() {
  emit(logs, "hello")
}
`)
	prog, diags, err := tascript.Compile(src)
	if err != nil {
		t.Fatalf("compile: %v (diags=%#v)", err, diags)
	}
	r, _ := tascript.Launch(prog, tascript.Wiring{})
	_ = r.Init()
	_ = r.Step()
	events := r.DrainEvents()
	if len(events) != 1 || events[0].Value != "hello" || len(events[0].Data) != 0 {
		t.Errorf("events = %#v, want one logs event with value \"hello\"", events)
	}
}

func TestSlice0_CombinedOutput(t *testing.T) {
	src := []byte(`output price_alert: String {
  price: Number
}

function Init() {}
function Run() {
  emit(price_alert, "BTC up", price=100)
}
`)
	prog, diags, err := tascript.Compile(src)
	if err != nil {
		t.Fatalf("compile: %v (diags=%#v)", err, diags)
	}
	r, _ := tascript.Launch(prog, tascript.Wiring{})
	_ = r.Init()
	_ = r.Step()
	events := r.DrainEvents()
	if len(events) != 1 || events[0].Value != "BTC up" || events[0].Data["price"] != float64(100) {
		t.Errorf("events = %#v", events)
	}
}

func TestSlice0_Negative_MissingInit(t *testing.T) {
	src := readFile(t, "testdata/slice0/negative/missing_init.tas")
	_, diags, err := tascript.Compile(src)
	if err == nil {
		t.Fatalf("expected compile error, got nil")
	}
	want := loadDiags(t, "testdata/slice0/negative/missing_init.diagnostics.json")
	if !containsDiag(diags, want[0]) {
		t.Errorf("missing %s/%s in diagnostics %#v", want[0].Phase, want[0].Category, diags)
	}
}

func TestSlice0_Negative_ReservedKwarg(t *testing.T) {
	src := []byte(`output alerts {
  msg: String
}
function Init() {}
function Run() {
  emit(alerts, ts="now")
}
`)
	_, diags, err := tascript.Compile(src)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !containsDiag(diags, wantDiag{Phase: "parse", Category: "EMIT_RESERVED_KWARG"}) {
		t.Errorf("expected EMIT_RESERVED_KWARG, got %#v", diags)
	}
}

func TestSlice0_Negative_DuplicatePortName(t *testing.T) {
	src := []byte(`input btc: CandleSeries
input btc: Series
function Init() {}
function Run() {}
`)
	_, diags, err := tascript.Compile(src)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !containsDiag(diags, wantDiag{Phase: "parse", Category: "PORT_DUPLICATE"}) {
		t.Errorf("expected PORT_DUPLICATE, got %#v", diags)
	}
}

func TestSlice0_Negative_UnknownOutput(t *testing.T) {
	src := []byte(`output alerts {
  msg: String
}
function Init() {}
function Run() {
  emit(typo, msg="hi")
}
`)
	_, diags, err := tascript.Compile(src)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !containsDiag(diags, wantDiag{Phase: "parse", Category: "UNKNOWN_OUTPUT"}) {
		t.Errorf("expected UNKNOWN_OUTPUT, got %#v", diags)
	}
}

func TestSlice0_Negative_EmitPayloadExtraField(t *testing.T) {
	src := []byte(`output alerts {
  msg: String
}
function Init() {}
function Run() {
  emit(alerts, msg="hi", extra=1)
}
`)
	_, diags, err := tascript.Compile(src)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !containsDiag(diags, wantDiag{Phase: "parse", Category: "EMIT_PAYLOAD"}) {
		t.Errorf("expected EMIT_PAYLOAD, got %#v", diags)
	}
}

func TestSlice0_Negative_EmitPayloadMissingField(t *testing.T) {
	src := []byte(`output alerts {
  msg: String
}
function Init() {}
function Run() {
  emit(alerts)
}
`)
	_, diags, err := tascript.Compile(src)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !containsDiag(diags, wantDiag{Phase: "parse", Category: "EMIT_PAYLOAD"}) {
		t.Errorf("expected EMIT_PAYLOAD, got %#v", diags)
	}
}

func TestSlice0_Negative_EmitOutsideRun(t *testing.T) {
	src := []byte(`output alerts {}
function Init() {
  emit(alerts)
}
function Run() {}
`)
	_, diags, err := tascript.Compile(src)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !containsDiag(diags, wantDiag{Phase: "parse", Category: "EMIT_OUTSIDE_RUN"}) {
		t.Errorf("expected EMIT_OUTSIDE_RUN, got %#v", diags)
	}
}

func TestSlice1_CandleSourceFeedsRun(t *testing.T) {
	src := []byte(`input btc: CandleSeries

output ticks {
  price: Number
  volume: Number
  mid: Number
  adjusted: Number
}

function Init() {}
function Run() {
  emit(ticks,
       price=btc.closes,
       volume=btc.volumes,
       mid=btc.hl2,
       adjusted=btc.closes + 2 * 3)
}
`)
	prog, diags, err := tascript.Compile(src)
	if err != nil {
		t.Fatalf("compile: %v (diags=%#v)", err, diags)
	}
	r, err := tascript.Launch(prog, tascript.Wiring{
		DataSources: map[string]tascript.DataSource{
			"btc": &sliceSource{candles: []tascript.Candle{
				{Open: 90, High: 110, Low: 80, Close: 100, Volume: 5},
				{Open: 95, High: 120, Low: 90, Close: 115, Volume: 7},
			}},
		},
	})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if err := r.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	got := r.DrainEvents()
	if len(got) != 1 {
		t.Fatalf("step 1 events = %#v, want one event", got)
	}
	if got[0].Data["price"] != float64(100) ||
		got[0].Data["volume"] != float64(5) ||
		got[0].Data["mid"] != float64(95) ||
		got[0].Data["adjusted"] != float64(106) {
		t.Fatalf("step 1 data = %#v", got[0].Data)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step 2: %v", err)
	}
	got = r.DrainEvents()
	if len(got) != 1 {
		t.Fatalf("step 2 events = %#v, want one event", got)
	}
	if got[0].Data["price"] != float64(115) ||
		got[0].Data["volume"] != float64(7) ||
		got[0].Data["mid"] != float64(105) ||
		got[0].Data["adjusted"] != float64(121) {
		t.Fatalf("step 2 data = %#v", got[0].Data)
	}
}

func TestSlice2_IfComparisonsAndLogicalOperators(t *testing.T) {
	src := []byte(`THRESHOLD = 110

input btc: CandleSeries

output alerts {
  price: Number
}

function Init() {}
function Run() {
  if (btc.closes > THRESHOLD && !false) {
    emit(alerts, price=btc.closes)
  }
}
`)
	prog, diags, err := tascript.Compile(src)
	if err != nil {
		t.Fatalf("compile: %v (diags=%#v)", err, diags)
	}
	r, err := tascript.Launch(prog, tascript.Wiring{
		DataSources: map[string]tascript.DataSource{
			"btc": &sliceSource{candles: []tascript.Candle{
				{High: 105, Low: 95, Close: 100},
				{High: 120, Low: 100, Close: 115},
			}},
		},
	})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if err := r.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	if got := r.DrainEvents(); len(got) != 0 {
		t.Fatalf("step 1 events = %#v, want none", got)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step 2: %v", err)
	}
	got := r.DrainEvents()
	if len(got) != 1 || got[0].Output != "alerts" || got[0].Data["price"] != float64(115) {
		t.Fatalf("step 2 events = %#v, want one alert at 115", got)
	}
}

func TestSlice3_StateAndMathMinMax(t *testing.T) {
	src := []byte(`COOLDOWN_BARS = 2

input btc: CandleSeries

output alerts {
  price: Number
}

function Init() {
  state.cooldown = 0
}
function Run() {
  state.cooldown = math.max(0, state.cooldown - 1)
  if (btc.closes > 100 && state.cooldown == 0) {
    emit(alerts, price=btc.closes)
    state.cooldown = COOLDOWN_BARS
  }
}
`)
	prog, diags, err := tascript.Compile(src)
	if err != nil {
		t.Fatalf("compile: %v (diags=%#v)", err, diags)
	}
	r, err := tascript.Launch(prog, tascript.Wiring{
		DataSources: map[string]tascript.DataSource{
			"btc": &sliceSource{candles: []tascript.Candle{
				{Close: 101},
				{Close: 105},
				{Close: 110},
			}},
		},
	})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if err := r.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	got := r.DrainEvents()
	if len(got) != 1 || got[0].Data["price"] != float64(101) {
		t.Fatalf("step 1 events = %#v, want one alert at 101", got)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step 2: %v", err)
	}
	if got := r.DrainEvents(); len(got) != 0 {
		t.Fatalf("step 2 events = %#v, want none during cooldown", got)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step 3: %v", err)
	}
	got = r.DrainEvents()
	if len(got) != 1 || got[0].Data["price"] != float64(110) {
		t.Fatalf("step 3 events = %#v, want one alert at 110", got)
	}
}

func TestSlice4_HistoryIndexing(t *testing.T) {
	src := []byte(`input btc: CandleSeries

output ticks {
  curr: Number
  prev: Number
  prev_close: Number
}

function Init() {
  state.seen = 0
}
function Run() {
  if (state.seen > 0) {
    emit(ticks, curr=btc.closes[0], prev=btc.closes[1], prev_close=btc[1].close)
  }
  state.seen = state.seen + 1
}
`)
	prog, diags, err := tascript.Compile(src)
	if err != nil {
		t.Fatalf("compile: %v (diags=%#v)", err, diags)
	}
	r, err := tascript.Launch(prog, tascript.Wiring{
		DataSources: map[string]tascript.DataSource{
			"btc": &sliceSource{candles: []tascript.Candle{
				{Close: 100},
				{Close: 115},
			}},
		},
	})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if err := r.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	if got := r.DrainEvents(); len(got) != 0 {
		t.Fatalf("step 1 events = %#v, want none", got)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step 2: %v", err)
	}
	got := r.DrainEvents()
	if len(got) != 1 {
		t.Fatalf("step 2 events = %#v, want one event", got)
	}
	if got[0].Data["curr"] != float64(115) ||
		got[0].Data["prev"] != float64(100) ||
		got[0].Data["prev_close"] != float64(100) {
		t.Fatalf("step 2 data = %#v", got[0].Data)
	}
}

func TestSlice4_Negative_HistoryOutOfRange(t *testing.T) {
	src := []byte(`input btc: CandleSeries

output ticks {
  prev: Number
}

function Init() {}
function Run() {
  emit(ticks, prev=btc.closes[1])
}
`)
	prog, diags, err := tascript.Compile(src)
	if err != nil {
		t.Fatalf("compile: %v (diags=%#v)", err, diags)
	}
	r, err := tascript.Launch(prog, tascript.Wiring{
		DataSources: map[string]tascript.DataSource{
			"btc": &sliceSource{candles: []tascript.Candle{{Close: 100}}},
		},
	})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if err := r.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	err = r.Step()
	if err == nil {
		t.Fatalf("expected history runtime error")
	}
	d, ok := err.(tascript.Diagnostic)
	if !ok || d.Category != "HISTORY_OUT_OF_RANGE" {
		t.Fatalf("step error = %#v, want HISTORY_OUT_OF_RANGE diagnostic", err)
	}
}

func TestSlice4_Negative_DynamicHistoryIndex(t *testing.T) {
	src := []byte(`input btc: CandleSeries

output ticks {
  prev: Number
}

function Init() {
  state.idx = 1
}
function Run() {
  emit(ticks, prev=btc.closes[state.idx])
}
`)
	_, diags, err := tascript.Compile(src)
	if err == nil {
		t.Fatalf("expected compile error")
	}
	if !containsDiag(diags, wantDiag{Phase: "parse", Category: "TOP_LEVEL_FORM"}) {
		t.Fatalf("expected TOP_LEVEL_FORM for dynamic history index, got %#v", diags)
	}
}

func TestSlice4_Negative_HelperHistoryLimit(t *testing.T) {
	src := []byte(`input btc: CandleSeries

output alerts {
  high: Number
}

function Init() {}
function Run() {
  emit(alerts, high=ta.highest(btc.closes, 6002))
}
`)
	_, diags, err := tascript.Compile(src)
	if err == nil {
		t.Fatalf("expected compile failure")
	}
	if !containsDiag(diags, wantDiag{Phase: "parse", Category: "HISTORY_LIMIT"}) {
		t.Fatalf("expected HISTORY_LIMIT for helper lookback, got %#v", diags)
	}
}

func TestDefaultHelpers_TAUsesSeriesHistory(t *testing.T) {
	src := []byte(`input btc: CandleSeries

output alerts {
  price: Number
  high: Number
}

function Init() {
  state.seen = 0
}
function Run() {
  if (state.seen > 0 && ta.crossover(btc.closes, 110)) {
    emit(alerts, price=btc.closes, high=ta.highest(btc.closes, 2))
  }
  state.seen = state.seen + 1
}
`)
	prog, diags, err := tascript.Compile(src)
	if err != nil {
		t.Fatalf("compile: %v (diags=%#v)", err, diags)
	}
	r, err := tascript.Launch(prog, tascript.Wiring{
		DataSources: map[string]tascript.DataSource{
			"btc": &sliceSource{candles: []tascript.Candle{
				{Close: 100},
				{Close: 115},
			}},
		},
	})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if err := r.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	if got := r.DrainEvents(); len(got) != 0 {
		t.Fatalf("step 1 events = %#v, want none", got)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step 2: %v", err)
	}
	got := r.DrainEvents()
	if len(got) != 1 {
		t.Fatalf("step 2 events = %#v, want one event", got)
	}
	if got[0].Data["price"] != float64(115) || got[0].Data["high"] != float64(115) {
		t.Fatalf("step 2 data = %#v", got[0].Data)
	}
}

func TestLaunch_InputNotWired(t *testing.T) {
	src := []byte(`input btc: CandleSeries

output alerts {}

function Init() {}
function Run() {
  emit(alerts)
}
`)
	prog, diags, err := tascript.Compile(src)
	if err != nil {
		t.Fatalf("compile: %v (diags=%#v)", err, diags)
	}
	_, err = tascript.Launch(prog, tascript.Wiring{})
	d, ok := err.(tascript.Diagnostic)
	if !ok || string(d.Category) != "INPUT_NOT_WIRED" || string(d.Phase) != "launch" {
		t.Fatalf("launch error = %#v, want INPUT_NOT_WIRED diagnostic", err)
	}
}

func TestLaunch_OutputSinkWiring(t *testing.T) {
	src := []byte(`output alerts {
  message: String
}

function Init() {}
function Run() {
  emit(alerts, message="hi")
}
`)
	prog, diags, err := tascript.Compile(src)
	if err != nil {
		t.Fatalf("compile: %v (diags=%#v)", err, diags)
	}
	_, err = tascript.Launch(prog, tascript.Wiring{StrictOutputWiring: true})
	d, ok := err.(tascript.Diagnostic)
	if !ok || string(d.Category) != "OUTPUT_NOT_WIRED" || string(d.Phase) != "launch" {
		t.Fatalf("launch error = %#v, want OUTPUT_NOT_WIRED diagnostic", err)
	}

	sink := &recordSink{}
	r, err := tascript.Launch(prog, tascript.Wiring{
		StrictOutputWiring: true,
		Sinks:              map[string]tascript.Sink{"alerts": sink},
	})
	if err != nil {
		t.Fatalf("launch with sink: %v", err)
	}
	if err := r.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step: %v", err)
	}
	if len(sink.events) != 1 || sink.events[0].Data["message"] != "hi" {
		t.Fatalf("sink events = %#v", sink.events)
	}
}

func TestConfig_CustomTypesAndHelpers(t *testing.T) {
	reg := tascript.NewRegistry()
	must(t, reg.RegisterType(tascript.TypeSpec{Name: "Number", Value: true, Field: true}))
	must(t, reg.RegisterType(tascript.TypeSpec{Name: "String", Value: true, Field: true}))
	must(t, reg.RegisterType(tascript.TypeSpec{Name: "Bool", Value: true, Field: true}))
	must(t, reg.RegisterType(tascript.TypeSpec{Name: "Score", Value: true, Field: true}))
	must(t, reg.RegisterHelper(tascript.HelperSpec{
		Namespace: "custom",
		Name:      "double",
		MinArgs:   1,
		MaxArgs:   1,
		Eval: func(args []tascript.Value) (tascript.Value, error) {
			return args[0].(float64) * 2, nil
		},
	}))

	src := []byte(`output scored {
  score: Score
}

function Init() {}
function Run() {
  emit(scored, score=custom.double(21))
}
`)
	prog, diags, err := tascript.CompileWithConfig(src, tascript.Config{Registry: reg})
	if err != nil {
		t.Fatalf("compile: %v (diags=%#v)", err, diags)
	}
	r, err := tascript.Launch(prog, tascript.Wiring{})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if err := r.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step: %v", err)
	}
	got := r.DrainEvents()
	if len(got) != 1 || got[0].Data["score"] != float64(42) {
		t.Fatalf("events = %#v, want score 42", got)
	}
}

func TestConfig_CustomIndicator(t *testing.T) {
	cfg := tascript.DefaultConfig()
	must(t, cfg.Registry.RegisterIndicator(tascript.IndicatorSpec{
		Name:    "offsetClose",
		MinArgs: 1,
		MaxArgs: 1,
		Build: func(args []tascript.Value) (tascript.Indicator, error) {
			return &offsetCloseIndicator{offset: args[0].(float64)}, nil
		},
	}))

	src := []byte(`input btc: CandleSeries

output alerts {
  first: Number
  second: Number
}

function Init() {}
function Run() {
  emit(alerts, first=btc.offsetClose(5), second=btc.offsetClose(5))
}
`)
	prog, diags, err := tascript.CompileWithConfig(src, cfg)
	if err != nil {
		t.Fatalf("compile: %v (diags=%#v)", err, diags)
	}
	indicatorCalls = 0
	r, err := tascript.Launch(prog, tascript.Wiring{
		DataSources: map[string]tascript.DataSource{
			"btc": &sliceSource{candles: []tascript.Candle{{Close: 100}}},
		},
	})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if err := r.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step: %v", err)
	}
	got := r.DrainEvents()
	if len(got) != 1 || got[0].Data["first"] != float64(105) || got[0].Data["second"] != float64(105) {
		t.Fatalf("events = %#v, want offset close values", got)
	}
	if indicatorCalls != 1 {
		t.Fatalf("indicator calls = %d, want one memoized call per tick", indicatorCalls)
	}
}

func TestDefaultIndicator_EMAHistory(t *testing.T) {
	src := []byte(`input btc: CandleSeries

output alerts {
  curr: Number
  prev: Number
}

function Init() {
  state.seen = 0
}
function Run() {
  if (state.seen > 0) {
    emit(alerts, curr=btc.ema(3), prev=btc.ema(3)[1])
  }
  state.seen = state.seen + 1
}
`)
	prog, diags, err := tascript.Compile(src)
	if err != nil {
		t.Fatalf("compile: %v (diags=%#v)", err, diags)
	}
	r, err := tascript.Launch(prog, tascript.Wiring{
		DataSources: map[string]tascript.DataSource{
			"btc": &sliceSource{candles: []tascript.Candle{
				{Close: 10},
				{Close: 14},
			}},
		},
	})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if err := r.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step 2: %v", err)
	}
	got := r.DrainEvents()
	if len(got) != 1 {
		t.Fatalf("events = %#v, want one", got)
	}
	if got[0].Data["curr"] != float64(12) || got[0].Data["prev"] != float64(10) {
		t.Fatalf("EMA data = %#v, want curr=12 prev=10", got[0].Data)
	}
}

func TestDefaultIndicator_ScalarSeriesReceiver(t *testing.T) {
	src := []byte(`input btc: CandleSeries

output alerts {
  curr: Number
  prev: Number
}

function Init() {
  state.seen = 0
}
function Run() {
  if (state.seen > 0) {
    emit(alerts, curr=btc.closes.ema(1).ema(1), prev=btc.closes.ema(1).ema(1)[1])
  }
  state.seen = state.seen + 1
}
`)
	prog, diags, err := tascript.Compile(src)
	if err != nil {
		t.Fatalf("compile: %v (diags=%#v)", err, diags)
	}
	r, err := tascript.Launch(prog, tascript.Wiring{
		DataSources: map[string]tascript.DataSource{
			"btc": &sliceSource{candles: []tascript.Candle{
				{Close: 10},
				{Close: 14},
			}},
		},
	})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if err := r.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step 2: %v", err)
	}
	got := r.DrainEvents()
	if len(got) != 1 {
		t.Fatalf("events = %#v, want one", got)
	}
	if got[0].Data["curr"] != float64(14) || got[0].Data["prev"] != float64(10) {
		t.Fatalf("chained EMA data = %#v, want curr=14 prev=10", got[0].Data)
	}
}

func TestDefaultIndicator_TupleOutputHistory(t *testing.T) {
	src := []byte(`input btc: CandleSeries

output alerts {
  upper: Number
  middle: Number
  lower: Number
  prevUpper: Number
}

function Init() {
  state.seen = 0
}
function Run() {
  if (state.seen > 0) {
    emit(alerts,
      upper=btc.bb(2, 1)[0],
      middle=btc.bb(2, 1)[1],
      lower=btc.bb(2, 1)[2],
      prevUpper=btc.bb(2, 1)[0][1])
  }
  state.seen = state.seen + 1
}
`)
	prog, diags, err := tascript.Compile(src)
	if err != nil {
		t.Fatalf("compile: %v (diags=%#v)", err, diags)
	}
	r, err := tascript.Launch(prog, tascript.Wiring{
		DataSources: map[string]tascript.DataSource{
			"btc": &sliceSource{candles: []tascript.Candle{
				{Close: 10},
				{Close: 14},
			}},
		},
	})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if err := r.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step 2: %v", err)
	}
	got := r.DrainEvents()
	if len(got) != 1 {
		t.Fatalf("events = %#v, want one", got)
	}
	want := map[string]any{
		"upper":     float64(14),
		"middle":    float64(12),
		"lower":     float64(10),
		"prevUpper": float64(10),
	}
	if !reflect.DeepEqual(got[0].Data, want) {
		t.Fatalf("BB data = %#v, want %#v", got[0].Data, want)
	}
}

func TestResourceLimits_CompileDiagnostics(t *testing.T) {
	t.Run("source size", func(t *testing.T) {
		cfg := tascript.DefaultConfig()
		cfg.ResourceLimits.MaxSourceBytes = 8
		_, diags, err := tascript.CompileWithConfig([]byte(`output alerts {}
function Init() {}
function Run() { emit(alerts) }
`), cfg)
		if err == nil || !containsDiag(diags, wantDiag{Phase: "parse", Category: "SOURCE_SIZE_LIMIT"}) {
			t.Fatalf("diags = %#v, err = %v; want SOURCE_SIZE_LIMIT", diags, err)
		}
	})

	t.Run("string literal", func(t *testing.T) {
		cfg := tascript.DefaultConfig()
		cfg.ResourceLimits.MaxStringLiteralLength = 2
		_, diags, err := tascript.CompileWithConfig([]byte(`output alerts {
  message: String
}
function Init() {}
function Run() {
  emit(alerts, message="hello")
}
`), cfg)
		if err == nil || !containsDiag(diags, wantDiag{Phase: "parse", Category: "STRING_LIMIT"}) {
			t.Fatalf("diags = %#v, err = %v; want STRING_LIMIT", diags, err)
		}
	})

	t.Run("identifier", func(t *testing.T) {
		cfg := tascript.DefaultConfig()
		cfg.ResourceLimits.MaxIdentLength = 3
		_, diags, err := tascript.CompileWithConfig([]byte(`output alerts {}
function Init() {}
function Run() {
  emit(alerts)
}
`), cfg)
		if err == nil || !containsDiag(diags, wantDiag{Phase: "parse", Category: "IDENT_LIMIT"}) {
			t.Fatalf("diags = %#v, err = %v; want IDENT_LIMIT", diags, err)
		}
	})

	t.Run("emit kwargs", func(t *testing.T) {
		cfg := tascript.DefaultConfig()
		cfg.ResourceLimits.MaxEmitKwargs = 1
		_, diags, err := tascript.CompileWithConfig([]byte(`output alerts {
  a: Number
  b: Number
}
function Init() {}
function Run() {
  emit(alerts, a=1, b=2)
}
`), cfg)
		if err == nil || !containsDiag(diags, wantDiag{Phase: "parse", Category: "KWARG_LIMIT"}) {
			t.Fatalf("diags = %#v, err = %v; want KWARG_LIMIT", diags, err)
		}
	})

	t.Run("expression depth", func(t *testing.T) {
		cfg := tascript.DefaultConfig()
		cfg.ResourceLimits.MaxExprDepth = 2
		_, diags, err := tascript.CompileWithConfig([]byte(`output alerts {
  value: Number
}
function Init() {}
function Run() {
  emit(alerts, value=1 + 2 * 3)
}
`), cfg)
		if err == nil || !containsDiag(diags, wantDiag{Phase: "parse", Category: "DEPTH_LIMIT"}) {
			t.Fatalf("diags = %#v, err = %v; want DEPTH_LIMIT", diags, err)
		}
	})
}

func TestResourceLimits_RuntimeStringValue(t *testing.T) {
	cfg := tascript.DefaultConfig()
	cfg.ResourceLimits.MaxRuntimeStringLength = 2
	must(t, cfg.Registry.RegisterHelper(tascript.HelperSpec{
		Namespace: "custom",
		Name:      "long",
		MinArgs:   0,
		MaxArgs:   0,
		Eval: func(args []tascript.Value) (tascript.Value, error) {
			return "hello", nil
		},
	}))

	prog, diags, err := tascript.CompileWithConfig([]byte(`output alerts {
  message: String
}
function Init() {}
function Run() {
  emit(alerts, message=custom.long())
}
`), cfg)
	if err != nil {
		t.Fatalf("compile: %v (diags=%#v)", err, diags)
	}
	r, err := tascript.Launch(prog, tascript.Wiring{})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if err := r.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	err = r.Step()
	d, ok := err.(tascript.Diagnostic)
	if !ok || string(d.Category) != "STRING_LIMIT" || string(d.Phase) != "runtime" {
		t.Fatalf("step error = %#v, want runtime STRING_LIMIT diagnostic", err)
	}
}

func TestTimeDuration_RuntimeSupport(t *testing.T) {
	first := utcMS(2026, time.May, 26, 0, 0, 0, 0)
	second := utcMS(2026, time.May, 27, 0, 0, 0, 0)

	src := []byte(`COOLDOWN = 30 * time.MINUTE

input btc: CandleSeries

output alerts {
  weekday: Number
  elapsedMinutes: Number
  cooldown: Duration
  at: Time
  shifted: Time
  tsMS: Number
  durationMS: Number
}

function Init() {}
function Run() {
  if (btc.timestamps[0].weekday == 3) {
    emit(alerts,
      weekday=btc.timestamps[0].weekday,
      elapsedMinutes=(btc.timestamps[0] - btc[1].ts).minutes,
      cooldown=COOLDOWN,
      at=btc.timestamps[0],
      shifted=btc.timestamps[0] - time.MINUTE,
      tsMS=btc[0].ts.unix_ms,
      durationMS=COOLDOWN.unix_ms)
  }
}
`)
	prog, diags, err := tascript.Compile(src)
	if err != nil {
		t.Fatalf("compile: %v (diags=%#v)", err, diags)
	}
	r, err := tascript.Launch(prog, tascript.Wiring{
		DataSources: map[string]tascript.DataSource{
			"btc": &sliceSource{candles: []tascript.Candle{
				{Close: 10, Ts: first},
				{Close: 11, Ts: second},
			}},
		},
	})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if err := r.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	if got := r.DrainEvents(); len(got) != 0 {
		t.Fatalf("step 1 events = %#v, want none", got)
	}
	if err := r.Step(); err != nil {
		t.Fatalf("step 2: %v", err)
	}
	got := r.DrainEvents()
	if len(got) != 1 {
		t.Fatalf("events = %#v, want one", got)
	}
	want := map[string]any{
		"weekday":        float64(3),
		"elapsedMinutes": float64(1440),
		"cooldown":       int64(30 * 60 * 1000),
		"at":             second,
		"shifted":        second - int64(60*1000),
		"tsMS":           float64(second),
		"durationMS":     float64(30 * 60 * 1000),
	}
	if !reflect.DeepEqual(got[0].Data, want) {
		t.Fatalf("time data = %#v, want %#v", got[0].Data, want)
	}
}

func TestStaticTypes_EmitPayloadAndOperators(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			name: "emit field mismatch",
			src: `output alerts {
  price: Number
}
function Init() {}
function Run() {
  emit(alerts, price="expensive")
}
`,
		},
		{
			name: "if condition mismatch",
			src: `output alerts {}
function Init() {}
function Run() {
  if (1) {
    emit(alerts)
  }
}
`,
		},
		{
			name: "time arithmetic mismatch",
			src: `input btc: CandleSeries
output alerts {
  bad: Time
}
function Init() {}
function Run() {
  emit(alerts, bad=btc.timestamps[0] + 1)
}
`,
		},
		{
			name: "duration to time field",
			src: `output alerts {
  at: Time
}
function Init() {}
function Run() {
  emit(alerts, at=time.MINUTE)
}
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, diags, err := tascript.Compile([]byte(tc.src))
			if err == nil {
				t.Fatalf("expected compile failure")
			}
			if !containsDiag(diags, wantDiag{Phase: "parse", Category: "TYPE_MISMATCH"}) {
				t.Fatalf("diags = %#v, want TYPE_MISMATCH", diags)
			}
		})
	}
}

// --- helpers ---

var indicatorCalls int

type offsetCloseIndicator struct {
	offset float64
}

func (i *offsetCloseIndicator) NextCandle(c tascript.Candle) (tascript.Value, error) {
	indicatorCalls++
	return c.Close + i.offset, nil
}

type sliceSource struct {
	candles []tascript.Candle
	cursor  int
}

func (s *sliceSource) NextCandle() (tascript.Candle, error) {
	if s.cursor >= len(s.candles) {
		return tascript.Candle{}, errors.New("no more candles")
	}
	c := s.candles[s.cursor]
	s.cursor++
	return c, nil
}

type recordSink struct {
	events []tascript.Event
}

func (s *recordSink) Emit(ev tascript.Event) error {
	s.events = append(s.events, ev)
	return nil
}

func utcMS(year int, month time.Month, day, hour, minute, second, millisecond int) int64 {
	return time.Date(year, month, day, hour, minute, second, millisecond*int(time.Millisecond), time.UTC).UnixMilli()
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

func loadEvents(t *testing.T, path string) []wantEvent {
	t.Helper()
	var out []wantEvent
	if err := json.Unmarshal(readFile(t, path), &out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return out
}

func loadDiags(t *testing.T, path string) []wantDiag {
	t.Helper()
	var out []wantDiag
	if err := json.Unmarshal(readFile(t, path), &out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return out
}

func containsDiag(diags []tascript.Diagnostic, w wantDiag) bool {
	for _, d := range diags {
		if string(d.Phase) == w.Phase && string(d.Category) == w.Category {
			return true
		}
	}
	return false
}
