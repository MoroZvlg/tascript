package stdlib_test

import (
	"errors"
	"math"
	"testing"

	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/evaluator"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/parser"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/resolver"
	"github.com/MoroZvlg/tascript/stdlib"
)

func runExpr(t *testing.T, expr string) (registry.Value, error) {
	t.Helper()
	p := parser.New(lexer.New("function Run() {\n" + expr + "\n}"))
	prog := p.Parse()
	if len(p.Diagnostics()) > 0 {
		t.Fatalf("parser diagnostics: %v", p.Diagnostics())
	}
	reg := registry.DefaultRegistry()
	stdlib.Register(reg)
	resolv := resolver.New(prog, reg)
	resolvedProg := resolv.Resolve()
	if len(resolv.Diagnostics()) > 0 {
		t.Fatalf("resolver diagnostics: %v", resolv.Diagnostics())
	}
	ev := evaluator.New(resolvedProg, reg)
	if _, err := ev.EvalInit(); err != nil {
		t.Fatalf("init error: %v", err)
	}
	return ev.EvalRun(nil)
}

func TestTime_Pipeline(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want registry.Value
	}{
		// construction + properties (2026-07-22T13:45:30.500Z)
		{"unix_ms round trip", `time.from_unix_ms(1784727930500).unix_ms`, registry.Integer(1784727930500)},
		{"year", `time.from_unix_ms(1784727930500).year`, registry.Integer(2026)},
		{"weekday", `time.from_unix_ms(1784727930500).weekday`, registry.Integer(3)},
		{"weekday vs constant", `time.from_unix_ms(1784727930500).weekday == time.WEDNESDAY`, registry.Bool(true)},
		// Time - Time -> Duration
		{"time diff minutes", `(time.from_unix_ms(90000) - time.from_unix_ms(0)).seconds`, registry.Float(90)},
		// Time +/- Duration -> Time
		{"time plus duration", `(time.from_unix_ms(0) + time.HOUR).unix_ms`, registry.Integer(3600000)},
		{"duration plus time", `(time.HOUR + time.from_unix_ms(0)).unix_ms`, registry.Integer(3600000)},
		{"time minus duration", `(time.from_unix_ms(3600000) - time.HOUR).unix_ms`, registry.Integer(0)},
		// Duration arithmetic and scaling
		{"scalar times duration", `(90 * time.MINUTE).minutes`, registry.Float(90)},
		{"duration times scalar", `(time.MINUTE * 90).minutes`, registry.Float(90)},
		{"float scale rounds ms", `(time.SECOND * 1.5).milliseconds`, registry.Integer(1500)},
		{"duration sum", `(time.HOUR + time.MINUTE).minutes`, registry.Float(61)},
		{"duration difference", `(time.HOUR - time.MINUTE).minutes`, registry.Float(59)},
		{"duration over duration ratio", `time.HOUR / time.MINUTE`, registry.Float(60)},
		{"duration over scalar", `(time.HOUR / 2).minutes`, registry.Float(30)},
		{"negate duration", `(-time.HOUR).hours`, registry.Float(-1)},
		{"abs duration", `(-time.HOUR).abs.hours`, registry.Float(1)},
		// comparisons
		{"duration compare", `time.HOUR > time.MINUTE`, registry.Bool(true)},
		{"duration equality", `time.HOUR == 60 * time.MINUTE`, registry.Bool(true)},
		{"time compare", `time.from_unix_ms(1) < time.from_unix_ms(2)`, registry.Bool(true)},
		{"time equality", `time.from_unix_ms(5) == time.from_unix_ms(5)`, registry.Bool(true)},
		// truncate floors to UTC midnight
		{"truncate to day", `time.from_unix_ms(1784727930500).truncate(time.DAY).unix_ms`, registry.Integer(1784678400000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runExpr(t, tt.expr)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestMath_Pipeline(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want registry.Value
	}{
		{"const PI", `math.PI`, registry.Float(math.Pi)},
		{"const E", `math.E`, registry.Float(math.E)},
		{"sqrt", `math.sqrt(2.0)`, registry.Float(math.Sqrt2)},
		{"abs", `math.abs(-3.5)`, registry.Float(3.5)},
		{"floor", `math.floor(2.9)`, registry.Float(2)},
		{"ceil", `math.ceil(2.1)`, registry.Float(3)},
		{"round", `math.round(2.5)`, registry.Float(3)},
		{"trunc", `math.trunc(-2.9)`, registry.Float(-2)},
		{"sign negative", `math.sign(-42.0)`, registry.Float(-1)},
		{"log", `math.log(math.E)`, registry.Float(1)},
		{"max", `math.max(3.0, 7.0)`, registry.Float(7)},
		{"min", `math.min(3.0, 7.0)`, registry.Float(3)},
		{"pow", `math.pow(2.0, 10.0)`, registry.Float(1024)},
		{"int coerces to float arg", `math.sqrt(4)`, registry.Float(2)},
		{"nested calls", `math.max(math.abs(-1.0), math.sqrt(9.0))`, registry.Float(3)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runExpr(t, tt.expr)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestStdlib_Traps_Pipeline(t *testing.T) {
	tests := []struct {
		name string
		expr string
		kind registry.ErrorKind
	}{
		{"sqrt of a negative", `math.sqrt(-1.0)`, registry.InvalidArgument},
		{"sqrt of a coerced negative", `math.sqrt(-1)`, registry.InvalidArgument},
		{"log of zero", `math.log(0.0)`, registry.InvalidArgument},
		{"log of a negative", `math.log(-1.0)`, registry.InvalidArgument},
		{"truncate to a non-divisor bucket", `time.from_unix_ms(0).truncate(7 * time.HOUR)`, registry.InvalidArgument},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runExpr(t, tt.expr)

			var failure diag.RuntimeFailure
			if !errors.As(err, &failure) {
				t.Fatalf("expected diag.RuntimeFailure, got %T: %v", err, err)
			}
			if failure.Kind != tt.kind {
				t.Errorf("kind = %s, want %s", failure.Kind, tt.kind)
			}
		})
	}
}
