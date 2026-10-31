package checker

import (
	"errors"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// dfidExpansion is one call of DFIDModelChecker.doNext. TLC generates and
// checks successors one next-state action at a time, so successors are
// collected per action and checked when the next action starts.
type dfidExpansion struct {
	search     *dfidSearch
	node       dfidNode
	batch      []dfidNode
	successors *[]dfidNode
	trace      func(dfidNode, *dfidNode) []TraceStep
	// deadlocked, allDone, and noLeaf follow doNext's local flags.
	deadlocked, allDone, noLeaf bool
	// aborted marks an exception after which TLC's worker stops.
	aborted bool
}

func newDFIDExpansion(s *dfidSearch, node dfidNode) *dfidExpansion {
	return &dfidExpansion{search: s, node: node, deadlocked: true, allDone: true, noLeaf: true}
}

// run returns false when the worker must stop.
func (e *dfidExpansion) run() (bool, error) {
	m := e.search.model
	gen := generator{model: m, current: e.node.state, emit: e.collect, actionStart: e.flush}
	more, err := gen.solve([]work{{expr: e.search.next, label: true}}, evaluator.State{}, m.cfg.Next)
	if err != nil {
		return e.generationFailure(err)
	}
	if more {
		if more, err = e.flush(); err != nil {
			return false, err
		}
	}
	if more {
		e.finish()
	}
	return !e.aborted, nil
}

func (e *dfidExpansion) collect(state evaluator.State, action string, source *parser.Expression) (bool, error) {
	if action == "" {
		action = e.search.model.cfg.Next
	}
	e.batch = append(e.batch, dfidNode{state: state, action: action, source: source})
	return true, nil
}

// flush checks the successors of one action. It returns false when doNext
// returns early after an error.
func (e *dfidExpansion) flush() (bool, error) {
	batch := e.batch
	e.batch = batch[:0]
	e.search.generated += len(batch)
	e.deadlocked = e.deadlocked && len(batch) == 0
	for i := range batch {
		if more, err := e.successor(&batch[i]); !more || err != nil {
			return more, err
		}
	}
	return true, nil
}

func (e *dfidExpansion) successor(succ *dfidNode) (bool, error) {
	status, err := e.admit(succ)
	if err != nil {
		return e.evaluationFailure(err, succ)
	}
	if status == dfidNew {
		more, skip, err := e.checkInvariants(succ)
		if !more || skip || err != nil {
			return more, err
		}
	}
	return e.checkActionProperties(succ)
}

// admit applies the constraints and records an in-model successor.
func (e *dfidExpansion) admit(succ *dfidNode) (int32, error) {
	inModel, err := e.inModel(succ)
	if err != nil || !inModel {
		return dfidNew, err
	}
	status, err := e.search.insert(succ)
	if err != nil {
		return dfidNew, err
	}
	e.track(succ, status)
	return status, nil
}

// inModel is TLC's isInModel and isInActions.
func (e *dfidExpansion) inModel(succ *dfidNode) (bool, error) {
	m := e.search.model
	inModel, err := m.constraintsHold(m.cfg.Constraints, succ.state, nil)
	if err != nil || !inModel {
		return inModel, err
	}
	return m.constraintsHold(m.cfg.ActionConstraints, e.node.state, succ.state)
}

// track updates doNext's flags and keeps a successor not completed at this
// level for the depth-first search.
func (e *dfidExpansion) track(succ *dfidNode, status int32) {
	states := &e.search.states
	e.allDone = e.allDone && dfidIsDone(status)
	e.noLeaf = e.noLeaf && !states.leaf(status)
	if e.successors != nil && !states.completed(status) {
		*e.successors = append(*e.successors, *succ)
	}
}

// checkInvariants returns skip after a violation reported with -continue,
// which leaves the action properties of that successor unchecked.
func (e *dfidExpansion) checkInvariants(succ *dfidNode) (bool, bool, error) {
	for _, invariant := range e.search.invariants {
		ok, err := invariant.holds(succ.state)
		if err != nil {
			return e.propertyFailure(err, succ, invariantFailure(invariant.name))
		}
		if !ok {
			return e.violation(succ, Result{Status: InvariantViolation, Violation: &Violation{Name: invariant.name}})
		}
	}
	return true, false, nil
}

func (e *dfidExpansion) checkActionProperties(succ *dfidNode) (bool, error) {
	m := e.search.model
	for _, p := range m.safety {
		if p.mode != actionAlways {
			continue
		}
		ok, err := m.holdsActionProperty(p, e.node.state, succ.state)
		if err != nil {
			more, _, err := e.propertyFailure(err, succ, actionPropertyFailure(p.name))
			return more, err
		}
		if !ok {
			more, _, err := e.violation(succ, Result{Status: LivenessViolation, Violation: &Violation{Name: p.name}})
			return more, err
		}
	}
	return true, nil
}

// finish reports a deadlock or marks the state done when all its successors
// are done.
func (e *dfidExpansion) finish() {
	if e.deadlocked && e.search.model.cfg.CheckDeadlock {
		e.report(Result{Status: Deadlock, Violation: &Violation{Name: "deadlock", Trace: e.traceTo(nil)}}, false, nil)
		return
	}
	if e.allDone {
		e.search.states.setDone(e.node.key)
	}
}

// generationFailure handles errors from computing successors. An incomplete
// successor is reported after the successors generated before it.
func (e *dfidExpansion) generationFailure(err error) (bool, error) {
	incomplete, ok := errors.AsType[*incompleteState](err)
	if !ok {
		e.batch = nil
		return e.evaluationFailure(err, nil)
	}
	// TLC counts the incomplete successor with the rest of its action.
	e.search.generated++
	more, err := e.flush()
	if more && err == nil {
		step, message := e.search.model.incompleteSuccessor(incomplete)
		failure := &Failure{Message: message, ToolCode: 2109, ExitCode: 75, Trace: append(e.traceTo(nil), step)}
		e.report(Result{Status: EvaluationFailure, Failure: failure}, false, nil)
	}
	return !e.aborted, err
}

func (e *dfidExpansion) traceTo(succ *dfidNode) []TraceStep {
	if e.trace == nil {
		return nil
	}
	return e.trace(e.node, succ)
}
