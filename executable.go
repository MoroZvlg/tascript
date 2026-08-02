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
)

type Executable struct {
	eval  *evaluator.Evaluator
	stage Stage
}

func (e *Executable) Stage() Stage { return e.stage }

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
// TODO: result is temp for debug/tests
func (e *Executable) Run(inputs map[string]registry.Value) (registry.Value, error) {
	if e.stage != StageInitialized {
		return nil, fmt.Errorf("%w (stage %s)", ErrNotInitialized, e.stage)
	}
	result, err := e.eval.EvalRun(inputs)
	return result, err
}

// TODO: temporary API, see Evaluator.Emitted
func (e *Executable) Emitted() []registry.NamedValue {
	return e.eval.Emitted()
}
