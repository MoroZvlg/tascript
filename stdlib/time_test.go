package stdlib_test

import (
	"errors"
	"testing"

	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/stdlib"
)

// 2026-07-22T13:45:30.500Z, a Wednesday.
const refMs = 1784727930500

func TestTimeModuleDurationConstants(t *testing.T) {
	reg := registry.DefaultRegistry()
	stdlib.Register(reg)
	module := reg.Modules["time"]

	tests := []struct {
		name string
		want stdlib.Duration
	}{
		{"MILLISECOND", 1},
		{"SECOND", 1000},
		{"MINUTE", 60 * 1000},
		{"HOUR", 60 * 60 * 1000},
		{"DAY", 24 * 60 * 60 * 1000},
		{"WEEK", 7 * 24 * 60 * 60 * 1000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, ok := reg.LookupMemberAccess(module.TypeID(), tt.name)
			if !ok {
				t.Fatal("constant is not registered")
			}
			if rule.EvalType != stdlib.DurationID {
				t.Fatalf("type = %v, want Duration", rule.EvalType)
			}
			got, err := rule.EvalFn(module)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("value = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTimeModuleWeekdayConstants(t *testing.T) {
	reg := registry.DefaultRegistry()
	stdlib.Register(reg)
	module := reg.Modules["time"]

	tests := []struct {
		name string
		want registry.Integer
	}{
		{"SUNDAY", 0}, {"MONDAY", 1}, {"TUESDAY", 2}, {"WEDNESDAY", 3},
		{"THURSDAY", 4}, {"FRIDAY", 5}, {"SATURDAY", 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, ok := reg.LookupMemberAccess(module.TypeID(), tt.name)
			if !ok {
				t.Fatal("constant is not registered")
			}
			if rule.EvalType != registry.IntegerID {
				t.Fatalf("type = %v, want Integer", rule.EvalType)
			}
			got, _ := rule.EvalFn(module)
			if got != tt.want {
				t.Fatalf("value = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTimeProperties(t *testing.T) {
	reg := registry.DefaultRegistry()
	stdlib.Register(reg)
	ref := stdlib.Time(refMs)

	tests := []struct {
		prop string
		want registry.Integer
	}{
		{"unix_ms", refMs},
		{"year", 2026},
		{"month", 7},
		{"day", 22},
		{"weekday", 3}, // Wednesday
		{"hour", 13},
		{"minute", 45},
		{"second", 30},
		{"millisecond", 500},
	}
	for _, tt := range tests {
		t.Run(tt.prop, func(t *testing.T) {
			rule, ok := reg.LookupMemberAccess(stdlib.TimeID, tt.prop)
			if !ok {
				t.Fatal("property is not registered")
			}
			got, err := rule.EvalFn(ref)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("%s = %v, want %v", tt.prop, got, tt.want)
			}
		})
	}
}

func TestDurationProperties(t *testing.T) {
	reg := registry.DefaultRegistry()
	stdlib.Register(reg)
	d := stdlib.Duration(90 * 60 * 1000) // 90 minutes

	tests := []struct {
		prop string
		want registry.Value
	}{
		{"milliseconds", registry.Integer(90 * 60 * 1000)},
		{"seconds", registry.Float(5400)},
		{"minutes", registry.Float(90)},
		{"hours", registry.Float(1.5)},
		{"days", registry.Float(1.5 / 24)},
		{"weeks", registry.Float(1.5 / 24 / 7)},
	}
	for _, tt := range tests {
		t.Run(tt.prop, func(t *testing.T) {
			rule, ok := reg.LookupMemberAccess(stdlib.DurationID, tt.prop)
			if !ok {
				t.Fatal("property is not registered")
			}
			got, _ := rule.EvalFn(d)
			if got != tt.want {
				t.Fatalf("%s = %v, want %v", tt.prop, got, tt.want)
			}
		})
	}
}

func TestDurationAbs(t *testing.T) {
	reg := registry.DefaultRegistry()
	stdlib.Register(reg)
	rule, _ := reg.LookupMemberAccess(stdlib.DurationID, "abs")
	for _, in := range []stdlib.Duration{-5, 5, 0} {
		got, _ := rule.EvalFn(in)
		want := in
		if want < 0 {
			want = -want
		}
		if got != want {
			t.Fatalf("abs(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestTimeFromUnixMs(t *testing.T) {
	reg := registry.DefaultRegistry()
	stdlib.Register(reg)
	rule, ok := reg.LookupCall(reg.Modules["time"].TypeID(), "from_unix_ms")
	if !ok {
		t.Fatal("from_unix_ms is not registered")
	}
	if rule.EvalType != stdlib.TimeID {
		t.Fatalf("type = %v, want Time", rule.EvalType)
	}
	got, err := rule.EvalFn(nil, map[string]registry.Value{"ms": registry.Integer(refMs)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != stdlib.Time(refMs) {
		t.Fatalf("got %v, want %v", got, stdlib.Time(refMs))
	}
}

func TestTimeTruncate(t *testing.T) {
	reg := registry.DefaultRegistry()
	stdlib.Register(reg)
	rule, _ := reg.LookupCall(stdlib.TimeID, "truncate")
	day := int64(24 * 60 * 60 * 1000)

	tests := []struct {
		name    string
		t       stdlib.Time
		bucket  stdlib.Duration
		want    stdlib.Time
		wantErr bool
	}{
		{"floor to day", stdlib.Time(refMs), stdlib.Duration(day), stdlib.Time(1784678400000), false},
		{"floor negative toward -inf", stdlib.Time(-3600000), stdlib.Duration(day), stdlib.Time(-day), false},
		{"6h bucket", stdlib.Time(refMs), stdlib.Duration(6 * 60 * 60 * 1000), stdlib.Time(1784721600000), false},
		{"zero bucket", stdlib.Time(refMs), stdlib.Duration(0), 0, true},
		{"negative bucket", stdlib.Time(refMs), stdlib.Duration(-1000), 0, true},
		{"larger than day", stdlib.Time(refMs), stdlib.Duration(day + 1), 0, true},
		{"non-dividing bucket", stdlib.Time(refMs), stdlib.Duration(7 * 60 * 60 * 1000), 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rule.EvalFn(tt.t, map[string]registry.Value{"bucket": tt.bucket})
			if tt.wantErr {
				var re registry.Error
				if !errors.As(err, &re) {
					t.Fatalf("want registry.Error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
