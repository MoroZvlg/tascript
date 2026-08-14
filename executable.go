package tascript

import (
	"errors"
	"fmt"

	"github.com/MoroZvlg/tascript/evaluator"
	"github.com/MoroZvlg/tascript/registry"
)

type Stage int

const (
	StageCreated Stage = iota
	StageInitialized
	StageFailed
)

func (s Stage) String() string {
	switch s {
	case StageCreated:
		return "created"
	case StageInitialized:
		return "initialized"
	case StageFailed:
		return "failed"
	}
	return "unknown"
}

var (
	ErrNotInitialized = errors.New("Init must succeed before Run")
	ErrInitRepeated   = errors.New("Init has already run")
	ErrBindTooLate    = errors.New("inputs must be bound before Init")
)

type Executable struct {
	eval  *evaluator.Evaluator
	stage Stage
}

func (e *Executable) Stage() Stage { return e.stage }

// BindInput keeps a bound pointer live: the host may keep mutating it between ticks.
// A bound value, and anything coerced on the way in, is the snapshot taken here.
func (e *Executable) BindInput(name string, value registry.Value) error {
	if e.stage != StageCreated {
		return fmt.Errorf("%w (stage %s)", ErrBindTooLate, e.stage)
	}
	return e.eval.BindInput(name, value)
}

// BindOutput must cover every declared output before Init, or Init fails.
func (e *Executable) BindOutput(name string, sink registry.Sink) error {
	if e.stage != StageCreated {
		return fmt.Errorf("%w (stage %s)", ErrBindTooLate, e.stage)
	}
	return e.eval.BindOutput(name, sink)
}

// Init runs the load phase.
// An error is fatal: the program never reached a valid initial state and can not be run.
func (e *Executable) Init() error {
	if e.stage != StageCreated {
		return fmt.Errorf("%w (stage %s)", ErrInitRepeated, e.stage)
	}

	if _, err := e.eval.EvalInit(); err != nil {
		e.stage = StageFailed
		return err
	}
	e.stage = StageInitialized
	return nil
}

// Run executes one tick.
// Error stops execution but do not prevent calling Run once again. State will be rollback
func (e *Executable) Run() error {
	if e.stage != StageInitialized {
		return fmt.Errorf("%w (stage %s)", ErrNotInitialized, e.stage)
	}
	_, err := e.eval.EvalRun()
	return err
}
