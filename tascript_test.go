package tascript_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

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

// --- helpers ---

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
