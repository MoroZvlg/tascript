package tascript

import (
	"errors"
	"fmt"

	"github.com/MoroZvlg/tascript/evaluator"
	"github.com/MoroZvlg/tascript/registry"
	"github.com/MoroZvlg/tascript/resolved"
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
	ErrSetTooLate     = errors.New("a failed program can not be filled")

	ErrMidActivation = evaluator.ErrMidActivation
	ErrSlotEmpty     = evaluator.ErrSlotEmpty
)

type Executable struct {
	eval  *evaluator.Evaluator
	stage Stage
}

// Slot is a handle on one interior declared cell. Resolve it once at wire time:
// lookups are by name, reads and writes are not.
type Slot struct {
	ex   *Executable
	idx  int
	decl *resolved.SlotDecl
}

func (e *Executable) Slots() []Slot {
	decls := e.eval.SlotDecls()
	slots := make([]Slot, 0, len(decls))
	for i, decl := range decls {
		slots = append(slots, Slot{ex: e, idx: i, decl: decl})
	}
	return slots
}

func (e *Executable) Slot(kind, name string) (Slot, bool) {
	for i, decl := range e.eval.SlotDecls() {
		if decl.Kind == kind && decl.Name == name {
			return Slot{ex: e, idx: i, decl: decl}, true
		}
	}
	return Slot{}, false
}

func (s Slot) Kind() string          { return s.decl.Kind }
func (s Slot) Name() string          { return s.decl.Name }
func (s Slot) Type() registry.TypeID { return s.decl.T }

// Get reads at any stage; a cell the host never filled and Init never reached is empty.
func (s Slot) Get() (registry.Value, error) {
	return s.ex.eval.SlotGet(s.idx)
}

// Set fills the cell during the host's turn: before Init, or between ticks.
func (s Slot) Set(value registry.Value) error {
	if s.ex.stage == StageFailed {
		return fmt.Errorf("%w (stage %s)", ErrSetTooLate, s.ex.stage)
	}
	return s.ex.eval.SlotSet(s.idx, value)
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
// An error is not a rollback: slot writes before it persist, and Run may be called again.
func (e *Executable) Run() error {
	if e.stage != StageInitialized {
		return fmt.Errorf("%w (stage %s)", ErrNotInitialized, e.stage)
	}
	_, err := e.eval.EvalRun()
	return err
}
