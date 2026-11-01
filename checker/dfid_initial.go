package checker

import (
	"errors"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// initialize is DFIDModelChecker.doInit. TLC computes every initial state,
// including incomplete ones, before it checks them in order, and it stops
// without statistics after an initial violation or an incomplete state.
func (s *dfidSearch) initialize() (Result, bool, error) {
	states, err := s.initialStates()
	if err != nil {
		return s.initialFailure(err, nil)
	}
	s.generated = len(states)
	for _, state := range states {
		if !s.model.assignsAll(state) {
			return s.incompleteInitial(state)
		}
		if result, failed, err := s.addInitial(state); failed || err != nil {
			return result, failed, err
		}
	}
	return Result{}, false, nil
}

// incompleteInitial reports TLC_STATE_NOT_COMPLETELY_SPECIFIED_INITIAL.
// doInit returns this error rather than throwing it, so TLC reports it during
// a replay as well, and the replay ends without a nested-expression report.
func (s *dfidSearch) incompleteInitial(state evaluator.State) (Result, bool, error) {
	s.model.addUnassigned(state)
	failure := &Failure{Message: "State is not completely specified by the initial predicate:", ToolCode: 2106, InitialState: state, ExitCode: 255, SuppressStatistics: true}
	result := s.publish(Result{Status: EvaluationFailure, Failure: failure})
	if s.replaying {
		return Result{}, true, nil
	}
	return s.counts(result), true, nil
}

func (s *dfidSearch) initialStates() ([]evaluator.State, error) {
	var states []evaluator.State
	init, _ := s.model.definition(s.model.cfg.Init)
	gen := generator{model: s.model, initial: true, allowIncomplete: true, emit: func(state evaluator.State, _ string, _ *parser.Expression) (bool, error) {
		states = append(states, state)
		return true, nil
	}}
	_, err := gen.solve(&workList{item: work{expr: init}}, s.model.partialState(), "")
	return states, err
}

func (s *dfidSearch) addInitial(state evaluator.State) (Result, bool, error) {
	node := dfidNode{state: state}
	inModel, err := s.model.constraintsHold(s.model.cfg.Constraints, state, nil)
	status := dfidNew
	if err == nil && inModel {
		status, err = s.insert(&node)
	}
	if err != nil {
		return s.initialFailure(err, state)
	}
	if status != dfidNew {
		return Result{}, false, nil
	}
	if inModel {
		s.inits = append(s.inits, node)
	}
	if result, failed, err := s.checkInitialInvariants(node); failed || err != nil {
		return result, failed, err
	}
	return s.checkImpliedInits(node)
}

// checkInitialInvariants reports every violated invariant with -continue.
func (s *dfidSearch) checkInitialInvariants(node dfidNode) (Result, bool, error) {
	for _, invariant := range s.invariants {
		ok, err := invariant.holds(node.state)
		if err != nil {
			return s.initialFailure(err, node.state)
		}
		if ok {
			continue
		}
		result := Result{Status: InvariantViolation, Violation: &Violation{Name: invariant.name, Trace: []TraceStep{node.step()}}}
		if !s.model.continueChecking {
			return s.initialViolation(result)
		}
		s.continueViolation(result)
	}
	return Result{}, false, nil
}

func (s *dfidSearch) checkImpliedInits(node dfidNode) (Result, bool, error) {
	for _, p := range s.model.safety {
		if p.mode != initialOnly {
			continue
		}
		ok, err := s.model.holdsProperty(p.predicate, node.state, p.locals)
		if err != nil {
			return s.initialFailure(err, node.state)
		}
		if !ok {
			return s.initialViolation(Result{Status: LivenessViolation, Violation: &Violation{Name: p.name, Trace: []TraceStep{node.step()}}})
		}
	}
	return Result{}, false, nil
}

func (s *dfidSearch) initialViolation(result Result) (Result, bool, error) {
	if s.replaying {
		s.replayEnded(nil)
		return Result{}, true, nil
	}
	return s.counts(s.publish(result)), true, nil
}

// initialFailure follows modelCheckImpl after an exception in doInit: TLC
// reports it, replays doInit for a call stack, and prints statistics. The
// exit status is 0 unless the replay fails as well.
func (s *dfidSearch) initialFailure(err error, state evaluator.State) (Result, bool, error) {
	if failure := s.initialUnionRenderFailure(err, state); failure != nil {
		return s.publish(Result{Status: EvaluationFailure, Failure: failure}), true, nil
	}
	failure, ok := asRuntimeError(err)
	if !ok {
		return Result{}, true, err
	}
	if s.replaying {
		s.replayEnded(err)
		return Result{}, true, nil
	}
	result := &Failure{Message: failure.Message, ToolCode: 2102, InitialState: state}
	nestJavaMessage(result, failure)
	if state == nil {
		result = generalFailure("computing initial states", failure)
	}
	s.publish(Result{Status: EvaluationFailure, Failure: result})
	if err := s.replayInitial(); err != nil {
		return Result{}, true, err
	}
	result.EvaluationDetail = s.replayDetail()
	if s.replayCause != nil {
		result.ExitCode = 75
	}
	return s.counts(Result{Status: EvaluationFailure, Failure: result}), true, nil
}

func (s *dfidSearch) initialUnionRenderFailure(err error, state evaluator.State) *Failure {
	if _, ok := errors.AsType[*value.UnionFirstElementError](err); ok && state != nil && !s.replaying {
		return s.model.initialStateRenderFailure(state)
	}
	return nil
}

func (s *dfidSearch) replayInitial() error {
	s.replaying, s.generated = true, 0
	defer func() { s.replaying = false }()
	_, _, err := s.initialize()
	return err
}
