package tascript

import (
	"github.com/MoroZvlg/tascript/evaluator"
	"github.com/MoroZvlg/tascript/registry"
)

type Executable struct {
	eval *evaluator.Evaluator
}

// Init runs the load phase.
// An error is fatal: the program never reached a valid initial state and can not be run.
func (e *Executable) Init() error {
	_, err := e.eval.EvalInit()
	return err
}

// Run executes one tick.
// Error stops execution but do not prevent calling Run once again. State will be rollback
// TODO: result is temp for debug/tests
func (e *Executable) Run() (registry.Value, error) {
	result, err := e.eval.EvalRun()
	return result, err
}
