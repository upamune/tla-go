package checker

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

type stateNode struct {
	state  evaluator.State
	parent int
	action string
	source *parser.Expression
	depth  int
}
type search struct {
	model               *model
	nodes               []stateNode
	visited             map[string]bool
	indices             map[string]int
	edges               [][]graphEdge
	initial             []int
	result              Result
	outDegrees          [32]int
	guarded             []guardedMonitor
	ltlGroups           []ltlCheckGroup
	ltlValues           map[ltlValueKey]bool
	fairChecks          []fairCachedChecks
	ltlSetMultiplier    int
	ltlFingerprints     map[int]uint64
	ltlVariableOrder    []string
	actionSource        *parser.Expression
	graphOrder          []string
	graphStates         map[string]evaluator.State
	candidateInModel    bool
	failureCandidateKey string
	reducedTableauTrace bool
}

func (m *model) check() (Result, error) {
	m.tlcRuntime.started = true
	m.emitLivenessWarnings()
	m.emitModelLiveness()
	if result, failed, err := m.checkAssumptions(); failed || err != nil {
		return result, err
	}
	m.emitSearchEvent(SearchEvent{Kind: InitialStart})
	if m.noStateSearch() {
		// Java TLC still reports that no initial states were generated.
		m.emitSearchEvent(SearchEvent{Kind: InitialComplete})
		return Result{Status: Success, Depth: 1}, nil
	}
	init, _ := m.definition(m.cfg.Init)
	s := newSearch(m)
	m.tlcRuntime.search = &s
	gen := generator{model: m, initial: true, emit: func(state evaluator.State, action string, source *parser.Expression) (bool, error) {
		return s.visit(state, -1, action, source)
	}}
	more, err := gen.solve(&workList{item: work{expr: init}}, m.partialState(), "")
	if err != nil {
		if incomplete, ok := errors.AsType[*incompleteState](err); ok {
			return s.incompleteInitialFailure(incomplete), nil
		}
		return s.runtimeFailure(err, nil, 75)
	}
	if !more {
		s.finishInitialEvaluationDetail()
		return s.result, nil
	}
	m.emitSearchEvent(SearchEvent{Kind: InitialComplete, GeneratedStates: s.result.GeneratedStates, DistinctStates: s.result.DistinctStates})
	if s.missingNextAction() {
		return s.result, nil
	}
	return s.explore()
}

func (m *model) checkAssumptions() (Result, bool, error) {
	for _, assumption := range m.module.Assumptions {
		if assumption.Axiom {
			continue
		}
		ok, err := m.holds(assumption.Expression, nil, nil)
		if err != nil {
			if failure, valid := errors.AsType[*evaluator.RuntimeError](err); valid {
				return m.assumptionViolation(assumption.Expression, failure.Message, failure.ToolCode), true, nil
			}
			return Result{}, false, err
		}
		if !ok {
			return m.assumptionViolation(assumption.Expression, "", 0), true, nil
		}
	}
	return Result{}, false, nil
}

func (m *model) assumptionViolation(expr *parser.Expression, message string, toolCode int) Result {
	r := expr.Location.Range
	name := fmt.Sprintf("line %d, col %d to line %d, col %d of module %s", r.Start.Line, r.Start.Column, r.End.Line, r.End.Column, r.Module)
	return Result{Status: AssumptionViolation, Violation: &Violation{Name: name, Message: message, ToolCode: toolCode, Trace: []TraceStep{}}}
}

func newSearch(m *model) search {
	s := search{model: m, visited: map[string]bool{}, result: Result{Status: Success}}
	if m.graph != nil {
		s.graphOrder = m.symmetryVariableOrder()
		if m.hasReduction() {
			s.graphStates = map[string]evaluator.State{}
		}
	}
	if len(m.properties) > 0 || len(m.guarded) > 0 {
		s.indices = map[string]int{}
	}
	for _, property := range m.guarded {
		s.guarded = append(s.guarded, guardedMonitor{property: property})
	}
	s.prepareIncrementalLiveness()
	return s
}

func (s *search) explore() (Result, error) {
	next, _ := s.model.definition(s.model.cfg.Next)
	for i := 0; i < len(s.nodes) && !s.model.tlcRuntime.stopped; i++ {
		s.model.tlcRuntime.level = s.nodes[i].depth
		if err := s.model.ctx.Err(); err != nil {
			return Result{}, err
		}
		s.result.QueuedStates--
		more, err := s.expandNode(i, next)
		if err != nil {
			return s.nextRuntimeFailure(err, i)
		}
		if !more {
			return s.finishFailedExpansion(i, next)
		}
	}
	s.debugLivenessGraph()
	result, err := s.checkLiveness()
	if err == nil && result.Status == Success {
		s.finishSuccessfulResult(&result)
	}
	return result, err
}

// missingNextAction reports TLC's error for initial states without a
// next-state action to expand them.
func (s *search) missingNextAction() bool {
	if len(s.nodes) == 0 || s.model.cfg.Next != "" && !s.hasNoNextActions() {
		return false
	}
	s.result.Status = EvaluationFailure
	s.result.Failure = &Failure{Message: "No next state actions defined to generate successor states from.", ToolCode: 2115, ExitCode: 75, SuppressStatistics: true}
	return true
}

func (s *search) hasNoNextActions() bool {
	next, err := s.model.definition(s.model.cfg.Next)
	return err == nil && s.model.noNextActions(next)
}

func (s *search) finishSuccessfulResult(result *Result) {
	if !s.model.tlcRuntime.stopped {
		result.Depth = max(result.Depth, 1)
	}
	result.OutDegrees = s.outDegrees
	result.HasOutDegrees = true
}

func (s *search) expandNode(index int, next *parser.Expression) (bool, error) {
	more, err := s.expandNodeGraph(index, next)
	if err != nil || !more {
		return more, err
	}
	return s.checkIncrementalLiveness(index)
}

func (s *search) expandNodeGraph(index int, next *parser.Expression) (bool, error) {
	if s.model.hasReduction() {
		return s.expandViewNode(index, next)
	}
	return s.expandRawNode(index, next)
}

func (s *search) expandViewNode(index int, next *parser.Expression) (bool, error) {
	more, err := s.expandRawNode(index, next)
	s.moveViewStutterLast(index)
	if err != nil || !more || len(s.model.livenessOrder) == 0 {
		return more, err
	}
	return more, s.fingerprintViewSuccessors(index)
}

func (s *search) expandRawNode(index int, next *parser.Expression) (bool, error) {
	runtime := s.model.tlcRuntime
	previous := runtime.traceCurrent
	runtime.traceCurrent = index
	defer func() { runtime.traceCurrent = previous }()
	count := 0
	before := s.result.DistinctStates
	gen := generator{model: s.model, current: s.nodes[index].state, emit: func(state evaluator.State, action string, source *parser.Expression) (bool, error) {
		count++
		if action == "" {
			action = s.model.cfg.Next
		}
		return s.visit(state, index, action, source)
	}}
	partial := s.model.takePartial()
	more, err := gen.solveNext(next, partial)
	s.model.returnPartial(partial)
	if err != nil || !more {
		addEnclosingActionSource(err, gen.actionRoot(next))
		if s.result.HasOutDegrees {
			// Java TLC samples a state's outdegree only after its expansion finishes.
			s.result.OutDegrees = s.outDegrees
		}
		return more, err
	}
	return s.finishExpansion(index, count, before)
}

func (s *search) finishExpansion(index, count, before int) (bool, error) {
	if err := s.graphStuttering(index); err != nil {
		return false, err
	}
	if count == 0 && s.model.cfg.CheckDeadlock {
		s.result.Status = Deadlock
		s.result.Violation = &Violation{Name: "deadlock", Trace: s.trace(index)}
		return false, nil
	}
	if more, err := s.checkGuardedProperties(index); err != nil || !more {
		return more, err
	}
	s.outDegrees[min(s.result.DistinctStates-before, len(s.outDegrees)-1)]++
	return true, nil
}

func (s *search) visit(state evaluator.State, parent int, action string, source *parser.Expression) (bool, error) {
	previousSource := s.actionSource
	s.actionSource = source
	defer func() { s.actionSource = previousSource }()
	runtime := s.model.tlcRuntime
	previousCurrent, previousCandidate := runtime.traceCurrent, runtime.traceCandidate
	runtime.traceCurrent, runtime.traceCandidate = parent, state
	if s.model.hasReduction() {
		runtime.viewLastCandidate, runtime.viewCandidateLevel = state, s.candidateLevel(parent)
	}
	defer func() { runtime.traceCurrent, runtime.traceCandidate = previousCurrent, previousCandidate }()
	oldLevel := s.model.tlcRuntime.level
	s.model.tlcRuntime.level = s.candidateLevel(parent)
	defer func() { s.model.tlcRuntime.level = oldLevel }()
	s.result.GeneratedStates++
	if parent < 0 {
		s.countInitialState()
	}
	s.result.Depth = max(s.result.Depth, 1)
	key, inModel, err := s.candidateKeyAndConstraints(state, parent)
	if err != nil {
		return false, candidateFailure(err, state, parent)
	}
	s.candidateInModel = inModel
	if s.alreadySeen(inModel, key) {
		return s.visitExisting(state, parent, action, key)
	}
	if inModel {
		s.countState(parent)
	}
	if more, err := s.checkGraphCandidate(state, parent, action, inModel); err != nil || !more {
		s.recordFailureCandidate(key, parent, inModel)
		return more, err
	}
	s.addIfIncluded(state, parent, action, key, inModel)
	return true, nil
}

func (s *search) countInitialState() {
	if n := s.model.tlcRuntime.initial; n > 1 && n&(n-1) == 0 {
		s.model.emitSearchEvent(SearchEvent{Kind: InitialProgress, GeneratedStates: n})
	}
	s.model.tlcRuntime.initial++
}

func (s *search) candidateLevel(parent int) int {
	if parent < 0 {
		return 1
	}
	return s.nodes[parent].depth + 1
}

func (s *search) checkNewCandidate(state evaluator.State, parent int, action string) (bool, error) {
	if s.continueInitialInvariants(parent) {
		if more, err := s.checkInitialInvariantsContinuing(state, action); err != nil || !more {
			return more, err
		}
		return s.checkSafetyProperties(state, parent, action)
	}
	if more, err := s.checkViolation(state, parent, action); err != nil || !more {
		if err != nil || !s.continueInvariantViolation() {
			return more, err
		}
	}
	return s.checkSafetyContinuing(state, parent, action)
}

func (s *search) visitExisting(state evaluator.State, parent int, action, key string) (bool, error) {
	if err := s.graphTransition(s.graphRepresentative(state, key), parent, action, false); err != nil {
		return false, err
	}
	if more, err := s.checkSafetyContinuing(state, parent, action); err != nil || !more {
		s.recordFailureCandidate(key, parent, s.candidateInModel)
		return more, err
	}
	s.linkExisting(parent, key, action, state)
	return true, nil
}

func (s *search) linkExisting(parent int, key, action string, state evaluator.State) {
	if s.indices != nil && parent >= 0 {
		edge := graphEdge{to: s.indices[key], action: action, source: s.actionSource}
		if s.model.hasReduction() {
			edge.next = state
		}
		s.edges[parent] = append(s.edges[parent], edge)
	}
}

func (s *search) addIfIncluded(state evaluator.State, parent int, action, key string, inModel bool) {
	if !inModel {
		return
	}
	s.add(state, parent, action, key)
	if s.graphStates != nil {
		s.graphStates[key] = state
	}
	if s.indices != nil {
		s.linkState(parent, key)
	}
}

func (s *search) alreadySeen(inModel bool, key string) bool { return inModel && s.visited[key] }
func (s *search) countState(parent int) {
	depth := 1
	if parent >= 0 {
		depth = s.nodes[parent].depth + 1
	} else {
		s.result.QueuedStates++
	}
	s.result.DistinctStates++
	s.result.Depth = max(s.result.Depth, depth)
}

func (s *search) checkViolation(state evaluator.State, parent int, action string) (bool, error) {
	name, err := s.model.invariantViolation(state)
	if err != nil {
		return s.invariantRuntimeFailure(err, name, state, parent, action)
	}
	if name == "" {
		return true, nil
	}
	trace := s.trace(parent)
	trace = append(trace, TraceStep{Action: action, State: state, ActionRange: actionSourceRange(s.actionSource)})
	s.result.Status = InvariantViolation
	s.result.Violation = &Violation{Name: name, Trace: trace}
	return false, nil
}

func (s *search) add(state evaluator.State, parent int, action, key string) {
	depth := 1
	if parent >= 0 {
		depth = s.nodes[parent].depth + 1
		s.result.QueuedStates++
	}
	s.visited[key] = true
	s.nodes = append(s.nodes, stateNode{state: state, parent: parent, action: action, source: s.actionSource, depth: depth})
	if s.model.hasReduction() {
		s.model.tlcRuntime.viewKeys = append(s.model.tlcRuntime.viewKeys, key)
		s.model.tlcRuntime.viewLivenessSeen = append(s.model.tlcRuntime.viewLivenessSeen, false)
	}
	s.model.tlcRuntime.rememberTraceState(state, len(s.nodes)-1)
}

func (s *search) satisfiesConstraints(state evaluator.State, parent int) (bool, error) {
	ok, err := s.model.constraintsHold(s.model.cfg.Constraints, state, nil)
	if err != nil || !ok {
		return ok, err
	}
	if err := s.model.trackPossible(s.model.possibleStates, state, nil); err != nil {
		return false, err
	}
	if parent < 0 {
		return true, nil
	}
	oldLevel := s.model.tlcRuntime.level
	s.model.tlcRuntime.level = s.nodes[parent].depth
	defer func() { s.model.tlcRuntime.level = oldLevel }()
	ok, err = s.model.constraintsHold(s.model.cfg.ActionConstraints, s.nodes[parent].state, state)
	if err != nil || !ok {
		return ok, err
	}
	if err := s.model.trackPossible(s.model.possibleActions, s.nodes[parent].state, state); err != nil {
		return false, err
	}
	return true, nil
}

func (m *model) constraintsHold(names []string, current, next evaluator.State) (bool, error) {
	for _, name := range names {
		if m.trueConfigured[name] {
			continue
		}
		expr, _ := m.definition(name)
		ok, err := m.constraintHolds(expr, current, next)
		if err != nil || !ok {
			return ok, err
		}
	}
	return true, nil
}

// invariantViolation returns the first invariant, including state-level
// []P properties, that state violates or fails to evaluate.
func (m *model) invariantViolation(state evaluator.State) (string, error) {
	for _, name := range m.cfg.Invariants {
		if m.trueConfigured[name] {
			continue
		}
		expr, _ := m.definition(name)
		ok, err := m.holds(expr, state, nil)
		if err != nil {
			return name, err
		}
		if !ok {
			return name, nil
		}
	}
	return m.safetyViolation(state)
}

func (m *model) safetyViolation(state evaluator.State) (string, error) {
	for _, p := range m.safety {
		if p.mode != stateAlways {
			continue
		}
		ok, err := m.holdsProperty(p.predicate, state, p.locals)
		if err != nil || !ok {
			return m.safetyViolationName(p), err
		}
	}
	return "", nil
}

func (s *search) trace(index int) []TraceStep {
	var path []TraceStep
	for index >= 0 {
		node := s.nodes[index]
		path = append(path, TraceStep{Action: node.action, State: node.state, ActionRange: actionSourceRange(node.source)})
		index = node.parent
	}
	for left, right := 0, len(path)-1; left < right; left, right = left+1, right-1 {
		path[left], path[right] = path[right], path[left]
	}
	return path
}

func (m *model) stateKey(state evaluator.State) (string, error) {
	if !m.hasReduction() {
		return m.unreducedStateKey(state)
	}
	if m.cfg.Symmetry != "" {
		return m.symmetryKey(state)
	}
	reduced, err := m.reduceState(state)
	if err != nil {
		return "", err
	}
	return m.reducedKey(reduced)
}

func (m *model) unreducedStateKey(state evaluator.State) (string, error) {
	if m.tlcFingerprint {
		return m.fingerprintKey(state)
	}
	// A lazy value can evaluate a definition while it is encoded, so a nested
	// call must not share the buffers.
	key, scratch := m.stateKeyBuffer[:0], m.valueKeyBuffer
	m.stateKeyBuffer, m.valueKeyBuffer = nil, nil
	for _, name := range m.names {
		var err error
		scratch, err = state[name].AppendKey(scratch[:0])
		if err != nil {
			return "", err
		}
		key = strconv.AppendInt(key, int64(len(scratch)), 10)
		key = append(key, ':')
		key = append(key, scratch...)
	}
	m.stateKeyBuffer, m.valueKeyBuffer = key, scratch
	return string(key), nil
}
