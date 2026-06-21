package tascript

import (
	"github.com/MoroZvlg/tascript/evaluator"
	"github.com/MoroZvlg/tascript/registry"
)

type Executable struct {
	eval *evaluator.Evaluator
}

func (e *Executable) Init() (registry.Value, error) {
	return e.eval.EvalInit()
}

func (e *Executable) Run() (registry.Value, error) {
	return e.eval.EvalRun()
}
