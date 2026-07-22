package stdlib

import (
	"time"

	"github.com/MoroZvlg/tascript/registry"
)

var TimeID = registry.NewTypeID("Time")
var DurationID = registry.NewTypeID("Duration")

// Time is a point in time as Unix epoch milliseconds.
type Time int64

func (t Time) TypeID() registry.TypeID {
	return TimeID
}

// Duration is a signed length of time in milliseconds.
type Duration int64

func (d Duration) TypeID() registry.TypeID {
	return DurationID
}

const (
	msPerSecond int64 = 1000
	msPerMinute       = 60 * msPerSecond
	msPerHour         = 60 * msPerMinute
	msPerDay          = 24 * msPerHour
	msPerWeek         = 7 * msPerDay
)

func RegisterTime(reg *registry.Registry) {
	reg.RegisterType(TimeID, registry.ScalarShape)
	reg.RegisterType(DurationID, registry.ScalarShape)
	registerTimeOperators(reg)

	timeModule, _ := reg.RegisterModule("time")
	moduleType := timeModule.TypeID()

	registerDurationConst(reg, moduleType, "MILLISECOND", 1)
	registerDurationConst(reg, moduleType, "SECOND", msPerSecond)
	registerDurationConst(reg, moduleType, "MINUTE", msPerMinute)
	registerDurationConst(reg, moduleType, "HOUR", msPerHour)
	registerDurationConst(reg, moduleType, "DAY", msPerDay)
	registerDurationConst(reg, moduleType, "WEEK", msPerWeek)

	registerWeekdayConst(reg, moduleType, "SUNDAY", 0)
	registerWeekdayConst(reg, moduleType, "MONDAY", 1)
	registerWeekdayConst(reg, moduleType, "TUESDAY", 2)
	registerWeekdayConst(reg, moduleType, "WEDNESDAY", 3)
	registerWeekdayConst(reg, moduleType, "THURSDAY", 4)
	registerWeekdayConst(reg, moduleType, "FRIDAY", 5)
	registerWeekdayConst(reg, moduleType, "SATURDAY", 6)

	reg.RegisterCall(moduleType, "from_unix_ms", registry.CallRule{
		Args:     []registry.ParamRule{{Type: registry.IntegerID, Name: "ms"}},
		EvalType: TimeID,
		EvalFn: func(_ registry.Value, args map[string]registry.Value) (registry.Value, error) {
			return Time(int64(args["ms"].(registry.Integer))), nil
		},
	})

	registerTimeProperties(reg)
	registerDurationProperties(reg)

	reg.RegisterCall(TimeID, "truncate", registry.CallRule{
		Args:     []registry.ParamRule{{Type: DurationID, Name: "bucket"}},
		EvalType: TimeID,
		EvalFn: func(receiver registry.Value, args map[string]registry.Value) (registry.Value, error) {
			return evalTruncate(int64(receiver.(Time)), int64(args["bucket"].(Duration)))
		},
	})
}

func registerDurationConst(reg *registry.Registry, module registry.TypeID, name string, ms int64) {
	reg.RegisterMemberAccess(module, name, registry.MemberAccessRule{
		EvalType: DurationID,
		EvalFn: func(_ registry.Value) (registry.Value, error) {
			return Duration(ms), nil
		},
	})
}

func registerWeekdayConst(reg *registry.Registry, module registry.TypeID, name string, day int) {
	reg.RegisterMemberAccess(module, name, registry.MemberAccessRule{
		EvalType: registry.IntegerID,
		EvalFn: func(_ registry.Value) (registry.Value, error) {
			return registry.Integer(day), nil
		},
	})
}

func registerTimeProperties(reg *registry.Registry) {
	registerTimeComponent(reg, "unix_ms", func(t time.Time) int { return int(t.UnixMilli()) })
	registerTimeComponent(reg, "year", func(t time.Time) int { return t.Year() })
	registerTimeComponent(reg, "month", func(t time.Time) int { return int(t.Month()) })
	registerTimeComponent(reg, "day", func(t time.Time) int { return t.Day() })
	registerTimeComponent(reg, "weekday", func(t time.Time) int { return int(t.Weekday()) })
	registerTimeComponent(reg, "hour", func(t time.Time) int { return t.Hour() })
	registerTimeComponent(reg, "minute", func(t time.Time) int { return t.Minute() })
	registerTimeComponent(reg, "second", func(t time.Time) int { return t.Second() })
	registerTimeComponent(reg, "millisecond", func(t time.Time) int { return t.Nanosecond() / 1e6 })
}

func registerTimeComponent(reg *registry.Registry, name string, fn func(time.Time) int) {
	reg.RegisterMemberAccess(TimeID, name, registry.MemberAccessRule{
		EvalType: registry.IntegerID,
		EvalFn: func(receiver registry.Value) (registry.Value, error) {
			utc := time.UnixMilli(int64(receiver.(Time))).UTC()
			return registry.Integer(fn(utc)), nil
		},
	})
}

func registerDurationProperties(reg *registry.Registry) {
	reg.RegisterMemberAccess(DurationID, "milliseconds", registry.MemberAccessRule{
		EvalType: registry.IntegerID,
		EvalFn: func(receiver registry.Value) (registry.Value, error) {
			return registry.Integer(int64(receiver.(Duration))), nil
		},
	})
	registerDurationTotal(reg, "seconds", msPerSecond)
	registerDurationTotal(reg, "minutes", msPerMinute)
	registerDurationTotal(reg, "hours", msPerHour)
	registerDurationTotal(reg, "days", msPerDay)
	registerDurationTotal(reg, "weeks", msPerWeek)
	reg.RegisterMemberAccess(DurationID, "abs", registry.MemberAccessRule{
		EvalType: DurationID,
		EvalFn: func(receiver registry.Value) (registry.Value, error) {
			d := receiver.(Duration)
			if d < 0 {
				return -d, nil
			}
			return d, nil
		},
	})
}

func registerDurationTotal(reg *registry.Registry, name string, unitMs int64) {
	reg.RegisterMemberAccess(DurationID, name, registry.MemberAccessRule{
		EvalType: registry.FloatID,
		EvalFn: func(receiver registry.Value) (registry.Value, error) {
			return registry.Float(float64(receiver.(Duration)) / float64(unitMs)), nil
		},
	})
}

// evalTruncate floors t to the start of a fixed UTC bucket. The bucket must be
// positive, no larger than a day, and evenly divide a day so boundaries never
// drift across UTC midnights. Flooring is toward negative infinity.
func evalTruncate(t, bucket int64) (registry.Value, error) {
	if bucket <= 0 {
		return nil, registry.Error{Kind: registry.InvalidArgument, Message: "truncate bucket must be positive"}
	}
	if bucket > msPerDay {
		return nil, registry.Error{Kind: registry.InvalidArgument, Message: "truncate bucket must not exceed time.DAY"}
	}
	if msPerDay%bucket != 0 {
		return nil, registry.Error{Kind: registry.InvalidArgument, Message: "truncate bucket must evenly divide time.DAY"}
	}
	return Time(floorDiv(t, bucket) * bucket), nil
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}
