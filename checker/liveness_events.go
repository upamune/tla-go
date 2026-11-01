package checker

import (
	"slices"
	"time"

	"github.com/upamune/tla-go/syntax/parser"
)

// startTemporalProgress counts Java's disk-graph table entries, not product
// nodes. TableauNodePtrTable counts fingerprints, including setDone entries
// without a consistent tableau node. Completed pure, unreduced exploration
// visits every such fingerprint; replay of a done state cannot introduce more.
func (s *search) startTemporalProgress() *time.Time {
	m := s.model
	if m.searchEvent == nil || !s.safeTemporalProgress() || m.tlcRuntime.stopped {
		return nil
	}
	if len(s.nodes) == 0 && s.hasNoNextActions() {
		return nil
	}
	branches, tableau := m.progressBranches()
	if !m.validProgressBranches(branches, tableau) {
		return nil
	}
	started := time.Now()
	m.emitSearchEvent(SearchEvent{Kind: TemporalCheckStart, Branches: branches, Nodes: branches * len(s.nodes), GeneratedStates: s.result.GeneratedStates, DistinctStates: s.result.DistinctStates, Level: max(s.result.Depth, 1)})
	return &started
}

func (s *search) invalidateTemporalProgress(started *time.Time, result Result, err error) {
	if started != nil && (err != nil || result.Failure != nil) {
		s.model.emitSearchEvent(SearchEvent{Kind: TemporalCheckUncertain})
	}
}

// Without a tableau or reduction, each completely expanded state has exactly
// one fingerprint entry in Java's single liveness graph, even if its action
// writes output. Only use that count for the incrementally verified path.
func (s *search) safeTemporalProgress() bool {
	if s.model.safeLivenessDiagnostic() {
		return true
	}
	return s.fairIncrementalOneSolution && !s.model.continueChecking && s.head == len(s.nodes) && s.simpleRecurrenceAlwaysTrue() && s.uniqueIncrementalFingerprints()
}

// TLC's disk graph identifies states by fingerprint. If tla-go reached two
// distinct states with the same fingerprint, its state count is not TLC's.
func (s *search) uniqueIncrementalFingerprints() bool {
	if len(s.ltlFingerprints) != len(s.nodes) {
		return false
	}
	seen := make(map[uint64]struct{}, len(s.ltlFingerprints))
	for _, fingerprint := range s.ltlFingerprints {
		if _, exists := seen[fingerprint]; exists {
			return false
		}
		seen[fingerprint] = struct{}{}
	}
	return true
}

// A state predicate that holds for every explored state makes []<>P true on
// every behavior. Restrict the extra scan to simple variable/integer tests;
// arbitrary predicates may be expensive or have effects during evaluation.
func (s *search) simpleRecurrenceAlwaysTrue() bool {
	if len(s.model.livenessOrder) != 1 {
		return false
	}
	p := s.model.livenessOrder[0]
	if p.mode != often || !s.simpleStateComparison(p.predicate) {
		return false
	}
	for _, node := range s.nodes {
		ok, err := s.model.holdsTemporalState(p, node.state)
		if err != nil || !ok {
			return false
		}
	}
	return true
}

func (s *search) simpleStateComparison(expr *parser.Expression) bool {
	if expr == nil || expr.Kind != "Apply" || len(expr.Args) != 2 {
		return false
	}
	switch expr.Text {
	case "=", "#", "<", "<=", ">", ">=":
	default:
		return false
	}
	return expr.Args[0].Kind == "Identifier" && slices.Contains(s.model.names, expr.Args[0].Text) && expr.Args[1].Kind == "Number"
}

func (m *model) validProgressBranches(branches int, tableau bool) bool {
	return branches > 0 && (!tableau || m.safeTableauProgress())
}

func (m *model) safeTableauProgress() bool {
	if m.hasReduction() {
		return false
	}
	// A late tableau node makes Java regenerate successors and recheck both
	// constraints. Reject unstable definitions, including constant helpers.
	for name, def := range m.defs {
		// Initial-state generation is not part of late tableau expansion.
		// Other definitions are still traversed recursively, so references to
		// the initial predicate from Next or a constraint remain guarded.
		if name == m.cfg.Init && !m.initConfiguredForReplay() {
			continue
		}
		if m.unsafeTableauDefinition(def) {
			return false
		}
	}
	return true
}

func (m *model) unsafeTableauDefinition(def parser.Definition) bool {
	return !m.replayConstantDefinition(def) && (m.expressionUsesTLC(def.Body, map[string]bool{}) || m.replayExpressionHasEffects(def.Body, map[string]bool{})) || m.unsafeIncrementalExpression(def.Body)
}

// Definitions referencing Init are checked above recursively. A configured
// predicate can also refer to Init directly, without another definition to scan.
func (m *model) initConfiguredForReplay() bool {
	init := m.cfg.Init
	return slices.Contains([]string{m.cfg.Next, m.cfg.Alias, m.cfg.Periodic, m.cfg.RLReward}, init) || slices.Contains(m.cfg.Constraints, init) ||
		slices.Contains(m.cfg.ActionConstraints, init) || slices.Contains(m.cfg.Invariants, init) ||
		slices.Contains(m.cfg.Properties, init) || slices.Contains(m.cfg.Possible, init) || slices.Contains(m.cfg.Postconditions, init)
}

func (m *model) progressBranches() (int, bool) {
	disjuncts, err := m.liveDisjunctsBounded(1024)
	if err != nil || len(disjuncts) == 0 {
		return 0, false
	}
	formulas, _, err := groupLiveFormulas(disjuncts)
	if err != nil {
		return 0, false
	}
	for _, formula := range formulas {
		if formula != nil {
			return len(formulas), true
		}
	}
	return len(formulas), false
}

// emitModelLiveness reports the number of normalized liveness solutions before
// initial-state generation, as TLC's LiveCheck does. Do not evaluate an
// effectful formula a second time solely to produce a diagnostic message.
func (m *model) emitModelLiveness() {
	if m.searchEvent == nil || !m.safeLivenessNormalization() {
		return
	}
	count, err := m.liveSolutionCount()
	if err == nil && count > 0 {
		m.emitSearchEvent(SearchEvent{Kind: ModelCheckingLiveness, Branches: count})
	}
}

func (m *model) safeLivenessDiagnostic() bool {
	if !m.safeLivenessNormalization() {
		return false
	}
	for _, fair := range m.fairness {
		if m.unsafeIncrementalExpression(fair.action) || m.unsafeIncrementalExpression(fair.subscript) {
			return false
		}
	}
	return true
}

func (m *model) safeLivenessNormalization() bool {
	if len(m.livenessOrder) == 0 {
		return false
	}
	for _, p := range m.livenessOrder {
		if m.unsafeIncrementalExpression(p.source) {
			return false
		}
	}
	for _, fair := range m.fairness {
		// WF/SF conversion creates enabled and action leaves without evaluating
		// either expression. Recurrence conversion can evaluate a constant leaf.
		if fair.recurrence && (m.unsafeIncrementalExpression(fair.action) || m.unsafeIncrementalExpression(fair.subscript)) {
			return false
		}
	}
	return true
}
