package checker

import (
	"errors"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// DepthFirstOptions selects TLC's depth-first iterative deepening (-dfid).
// Each iteration restarts at the initial states and searches one level
// deeper, so GeneratedStates counts states generated again in every
// iteration. Java TLC picks initial states and successors in a random order;
// tla-go takes them in generation order, which is one of the orders TLC can
// pick. Liveness properties are rejected, as in TLC.
type DepthFirstOptions struct {
	// MaxDepth is TLC's -dfid argument, the deepest level to search. Below 2,
	// checking stops after the initial states with an EvaluationFailure that
	// has no message, as Java TLC exits with status 255 without one.
	MaxDepth int
	// Workers is TLC's worker count. TLC's depth-first mode rejects more than
	// one worker; tla-go searches with one worker.
	Workers int
	// OnError, when set, receives each violation or evaluation failure when
	// TLC reports it. TLC finishes the search below the current initial state
	// before it stops, so output from later states follows the report. The
	// returned Result repeats the error that stopped checking. Trace states
	// are read-only and must be copied before modification.
	OnError func(Result)
}

const dfidWorkersMessage = "Depth-First Iterative Deepening mode does not support multiple workers (https://github.com/tlaplus/tlaplus/issues/548).  Please run TLC with a single worker."

const dfidLivenessMessage = "Depth-First Iterative Deepening mode does not support checking liveness properties (https://github.com/tlaplus/tlaplus/issues/548).  Please check liveness properties in Breadth-First-Search mode."

// dfidNode is a state on TLC's depth-first stack with its fingerprint key.
type dfidNode struct {
	state  evaluator.State
	key    string
	action string
	source *parser.Expression
}

// dfidSearch ports DFIDModelChecker for a single worker.
type dfidSearch struct {
	model   *model
	options DepthFirstOptions
	next    *parser.Expression
	states  dfidStates
	order   []string
	// fingerprintFailed stops collecting fingerprints TLC cannot reproduce.
	fingerprintFailed bool
	inits             []dfidNode
	generated         int
	level             int
	invariants        []dfidInvariant
	// done and stopped follow TLC's done flag and DFIDWorker stop code 2.
	done, stopped bool
	stop          Result
	continued     int
	// replay is the expansion TLC replays for a call stack after an error.
	replay      *dfidNode
	replaying   bool
	replayCause error
}

func (m *model) checkDepthFirst(options DepthFirstOptions) (Result, error) {
	if err := m.validateDepthFirst(options); err != nil {
		return Result{}, err
	}
	m.tlcRuntime.started = true
	if result, failed, err := m.checkAssumptions(); failed || err != nil {
		return result, err
	}
	s := &dfidSearch{model: m, options: options, states: newDFIDStates(), order: m.symmetryVariableOrder(), invariants: m.dfidInvariants(), level: 1}
	s.next, _ = m.definition(m.cfg.Next)
	m.tlcRuntime.depthFirst = s
	if result, done, err := s.start(); done || err != nil {
		return result, err
	}
	return s.iterate()
}

// start computes the initial states. TLC reports success at once when the
// next-state relation has no action.
func (s *dfidSearch) start() (Result, bool, error) {
	if !s.model.noStateSearch() {
		if result, failed, err := s.initialize(); failed || err != nil {
			return result, true, err
		}
	}
	s.emitInitial()
	if s.model.noStateSearch() || s.next == nil || s.model.noNextActions(s.next) {
		return s.success(), true, nil
	}
	return Result{}, false, nil
}

// validateDepthFirst rejects checks that TLC's depth-first mode lacks, in
// the order of DFIDModelChecker's assertions.
func (m *model) validateDepthFirst(options DepthFirstOptions) error {
	if options.Workers > 1 {
		return &ModelValidationError{Message: dfidWorkersMessage, Code: 1000, ExitCode: 255}
	}
	if len(m.livenessOrder) > 0 {
		return &ModelValidationError{Message: dfidLivenessMessage, Code: 1000, ExitCode: 255}
	}
	if len(m.cfg.Postconditions) > 0 || len(m.possibleStates)+len(m.possibleActions) > 0 {
		return errors.New("POSTCONDITION and _POSSIBLE are not supported in depth-first mode")
	}
	return nil
}

func (s *dfidSearch) emitInitial() {
	s.model.emitSearchEvent(SearchEvent{Kind: InitialComplete, GeneratedStates: s.generated, DistinctStates: len(s.inits)})
}

// iterate is the level loop of DFIDModelChecker.modelCheckImpl. TLC reports
// success only from the iteration after the search ends, so a search that
// ends at MaxDepth reaches the depth limit instead.
func (s *dfidSearch) iterate() (Result, error) {
	terminated := false
	for level := 2; level <= s.options.MaxDepth; level++ {
		if terminated {
			return s.finishTerminated()
		}
		s.level = level
		s.model.emitSearchEvent(SearchEvent{Kind: DepthFirstLevel, Level: level, GeneratedStates: s.generated, DistinctStates: len(s.states.status)})
		s.states.nextLevel()
		worker := &dfidWorker{search: s}
		if err := worker.run(); err != nil {
			return Result{}, err
		}
		terminated = s.stopped || !worker.moreLevel
	}
	if s.options.MaxDepth < 2 {
		return s.counts(Result{Status: EvaluationFailure, Failure: &Failure{ExitCode: 255}}), nil
	}
	return s.finishBounded(), nil
}

func (s *dfidSearch) finishTerminated() (Result, error) {
	if !s.done {
		return s.success(), nil
	}
	if s.replay != nil {
		if err := s.replayExpansion(*s.replay); err != nil {
			return Result{}, err
		}
		if detail := s.replayDetail(); detail != "" && s.stop.Failure != nil {
			s.stop.Failure.EvaluationDetail = detail
		}
	}
	return s.counts(s.stop), nil
}

func (s *dfidSearch) finishBounded() Result {
	if s.done {
		return s.counts(s.stop)
	}
	return s.counts(Result{Status: DepthLimit})
}

func (s *dfidSearch) success() Result {
	result := s.counts(Result{Status: Success})
	if !s.fingerprintFailed {
		result.FingerprintDistance, result.HasFingerprintDistance = dfidFingerprintDistance(s.states.fingerprints), true
	}
	return result
}

func (s *dfidSearch) counts(result Result) Result {
	result.GeneratedStates, result.DistinctStates, result.QueuedStates = s.generated, len(s.states.status), -1
	result.Depth, result.ContinuedViolations = s.level, s.continued
	return result
}

// insert records a state in the fingerprint set and keeps its TLC fingerprint
// for the observed collision probability.
func (s *dfidSearch) insert(node *dfidNode) (int32, error) {
	key, err := s.model.stateKey(node.state)
	if err != nil {
		return dfidNew, err
	}
	normalizeStateMembership(node.state)
	node.key = key
	status, added := s.states.insert(key)
	if added && !s.fingerprintFailed {
		fingerprint, err := s.model.javaFingerprint(node.state, s.order)
		s.fingerprintFailed = err != nil
		s.states.fingerprints = append(s.states.fingerprints, fingerprint)
	}
	return status, nil
}
