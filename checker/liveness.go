package checker

import (
	"fmt"
	"maps"
	"slices"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

type graphEdge struct {
	to     int
	action string
	source *parser.Expression
	// next preserves the concrete successor hidden by VIEW, when present.
	next evaluator.State
}

type temporalProperty struct {
	name string
	// source is the property conjunct before classification. TLC simulation
	// converts it to its own liveness normal form.
	source        *parser.Expression
	predicate     *parser.Expression
	trigger       *parser.Expression
	sequence      []*parser.Expression
	bounds        []parser.Bound
	locals        map[string]value.Value
	mode          temporalMode
	triggerAction bool
	general       *ltlFormula
	eventualities []*ltlFormula
	cacheAtom     *ltlFormula
}

type temporalMode int

const (
	eventually temporalMode = iota
	often
	eventuallyAlways
	leadsTo
	actionOften
	stateAlways
	actionAlways
	initialOnly
	guardedAlways
	orderedEventually
	oftenImpliesState
	oftenImpliesAction
	leadsToStaying
	staysOrOftenState
	staysOrOftenAction
	generalTemporal
)

// The supported forms have state predicates. More general temporal formulas
// need a tableau.
func (m *model) prepareProperties() error {
	for _, name := range m.cfg.Properties {
		if m.trueConfigured[name] {
			continue
		}
		expr, err := m.definition(name)
		if err != nil {
			return err
		}
		if err := m.preparePropertyExpression(name, expr, nil, map[string]bool{}); err != nil {
			return err
		}
	}
	if m.emptyTemporalProperty && len(m.livenessOrder) == 0 {
		return m.temporalTautology()
	}
	return nil
}

func (m *model) preparePropertyExpression(name string, expr *parser.Expression, bounds []parser.Bound, seen map[string]bool) error {
	if expr.Kind == "Forall" && len(expr.Args) == 1 {
		all := append(append([]parser.Bound(nil), bounds...), expr.Bounds...)
		return m.preparePropertyExpression(name, expr.Args[0], all, seen)
	}
	if expr.Kind == "And" && m.hasTemporal(expr, map[string]bool{}) {
		if hasFalsePropertyConjunct(expr) {
			return fmt.Errorf("The spec is trivially false because FALSE is false.")
		}
		return m.preparePropertyConjuncts(name, expr.Args, bounds, seen)
	}
	return m.prepareNonConjunctProperty(name, expr, bounds, seen)
}

func hasFalsePropertyConjunct(expr *parser.Expression) bool {
	if isBooleanLiteral(expr, "FALSE") {
		return true
	}
	if expr.Kind == "And" {
		if slices.ContainsFunc(expr.Args, hasFalsePropertyConjunct) {
			return true
		}
	}
	return false
}

func (m *model) prepareNonConjunctProperty(name string, expr *parser.Expression, bounds []parser.Bound, seen map[string]bool) error {
	if handled, err := m.prepareOrderedSequence(name, expr, bounds); handled || err != nil {
		return err
	}
	if handled, err := m.preparePropertyCall(name, expr, bounds, seen); handled {
		return err
	}
	property, err := m.classifyPropertyConjunct(temporalProperty{name: name, bounds: bounds, source: expr})
	if err != nil {
		return err
	}
	return m.expandProperty(property)
}

func (m *model) classifyPropertyConjunct(p temporalProperty) (temporalProperty, error) {
	property, ok := m.classifyProperty(p, p.source)
	if ok && m.compoundTemporalSafety(property) {
		return m.classifyCompoundSafety(p, property)
	}
	if ok {
		return property, nil
	}
	return m.classifyGeneralConjunct(p)
}

func (m *model) classifyCompoundSafety(original, property temporalProperty) (temporalProperty, error) {
	if property.mode == actionAlways {
		if len(property.bounds) == 0 {
			return property, liveActionFormatError()
		}
		return property, nil
	}
	return m.classifyGeneralConjunct(original)
}

func (m *model) classifyGeneralConjunct(p temporalProperty) (temporalProperty, error) {
	if trivialTemporalImplication(p.source) {
		if err := m.temporalTautology(); err != nil {
			return p, err
		}
	}
	p.predicate, p.mode = p.source, generalTemporal
	return p, nil
}

func trivialTemporalImplication(expr *parser.Expression) bool {
	if expr.Kind != "Apply" || expr.Text != "=>" || len(expr.Args) != 2 {
		return false
	}
	return isBooleanLiteral(expr.Args[0], "FALSE") || isBooleanLiteral(expr.Args[1], "TRUE")
}

func isBooleanLiteral(expr *parser.Expression, text string) bool {
	return expr.Kind == "Identifier" && expr.Text == text
}

func (m *model) preparePropertyConjuncts(name string, args []*parser.Expression, bounds []parser.Bound, seen map[string]bool) error {
	for _, arg := range args {
		if err := m.preparePropertyExpression(name, arg, bounds, seen); err != nil {
			return err
		}
	}
	return nil
}

func (m *model) preparePropertyCall(name string, expr *parser.Expression, bounds []parser.Bound, seen map[string]bool) (bool, error) {
	if expr.Kind != "Identifier" && expr.Kind != "Apply" {
		return false, nil
	}
	definition, ok := m.defs[expr.Text]
	if !ok || !m.hasTemporal(expr, map[string]bool{}) {
		return false, nil
	}
	return true, m.expandPropertyCall(name, expr, bounds, seen, definition)
}

func (m *model) expandPropertyCall(name string, call *parser.Expression, bounds []parser.Bound, seen map[string]bool, definition parser.Definition) error {
	if seen[call.Text] || definition.Kind != "Operator" || len(definition.Params) != len(call.Args) {
		return fmt.Errorf("recursive or invalid temporal operator %s", call.Text)
	}
	expanded, err := substituteActionCall(definition, call)
	if err != nil {
		return err
	}
	seen[call.Text] = true
	defer delete(seen, call.Text)
	if len(call.Args) > 0 {
		m.propertyCalls++
		defer func() { m.propertyCalls-- }()
	}
	return m.preparePropertyExpression(name, expanded, bounds, seen)
}

// TLC splits a property only at conjunctions and zero-arity definitions. A
// safety formula under a quantifier or a parameterized operator thus remains
// a temporal formula in both model checking and simulation.
func (m *model) compoundTemporalSafety(property temporalProperty) bool {
	if property.mode != stateAlways && property.mode != actionAlways {
		return false
	}
	return len(property.bounds) > 0 || m.propertyCalls > 0
}

func (m *model) classifyProperty(p temporalProperty, expr *parser.Expression) (temporalProperty, bool) {
	if property, ok := m.classifySafetyProperty(p, expr); ok {
		return property, true
	}
	return m.classifyLivenessProperty(p, expr)
}

func (m *model) classifySafetyProperty(p temporalProperty, expr *parser.Expression) (temporalProperty, bool) {
	if property, ok := m.classifyGuardedAlways(p, expr); ok {
		return property, true
	}
	if isAlwaysBox(expr) {
		p.predicate, p.mode = expr.Args[0], actionAlways
		return p, !m.hasTemporal(p.predicate, map[string]bool{}) && !m.hasAction(p.predicate.Args[1], map[string]bool{})
	}
	if expr.Kind == "Always" && len(expr.Args) == 1 && !m.hasAction(expr.Args[0], map[string]bool{}) {
		p.predicate, p.mode = expr.Args[0], stateAlways
		return p, true
	}
	if !m.hasAction(expr, map[string]bool{}) {
		p.predicate, p.mode = expr, initialOnly
		return p, true
	}
	return p, false
}

func (m *model) classifyLivenessProperty(p temporalProperty, expr *parser.Expression) (temporalProperty, bool) {
	if property, ok := m.classifyStaysOrOften(p, expr); ok {
		return property, true
	}
	if property, ok := m.classifyOftenImplication(p, expr); ok {
		return property, true
	}
	return m.classifyOtherLivenessProperty(p, expr)
}

func (m *model) classifyOtherLivenessProperty(p temporalProperty, expr *parser.Expression) (temporalProperty, bool) {
	if property, ok := m.classifyOrderedEventually(p, expr); ok {
		return property, true
	}
	return m.classifyRemainingLivenessProperty(p, expr)
}

func (m *model) classifyRemainingLivenessProperty(p temporalProperty, expr *parser.Expression) (temporalProperty, bool) {
	if property, ok := m.classifyLeadsToStaying(p, expr); ok {
		return property, true
	}
	if isLeadsTo(expr) {
		p.trigger, p.predicate, p.mode = expr.Args[0], expr.Args[1], leadsTo
		return p, !m.hasAction(p.trigger, map[string]bool{}) && !m.hasAction(p.predicate, map[string]bool{})
	}
	if isOftenAction(expr) {
		p.predicate, p.mode = expr.Args[0].Args[0], actionOften
		return p, !m.hasTemporal(p.predicate, map[string]bool{}) && !m.hasAction(p.predicate.Args[1], map[string]bool{})
	}
	expr, p.mode = temporalPredicate(expr)
	if expr == nil {
		return p, false
	}
	if m.hasAction(expr, map[string]bool{}) {
		return p, false
	}
	p.predicate = expr
	return p, true
}

func isAlwaysBox(expr *parser.Expression) bool {
	return expr.Kind == "Always" && len(expr.Args) == 1 && expr.Args[0].Kind == "BoxAction" && len(expr.Args[0].Args) == 2
}

func isLeadsTo(expr *parser.Expression) bool {
	return expr.Kind == "Apply" && expr.Text == "~>" && len(expr.Args) == 2
}

func isOftenAction(expr *parser.Expression) bool {
	if expr.Kind != "Always" || len(expr.Args) != 1 {
		return false
	}
	inner := expr.Args[0]
	return inner.Kind == "Eventually" && len(inner.Args) == 1 && inner.Args[0].Kind == "AngleAction" && len(inner.Args[0].Args) == 2
}

func (m *model) expandProperty(p temporalProperty) error {
	if err := m.validatePropertyBounds(p); err != nil {
		return err
	}
	expanded := false
	err := m.expandBindings(p.bounds, nil, func(locals map[string]value.Value) error {
		expanded = true
		return m.addPropertyInstance(p, locals)
	})
	if err == nil && !expanded && len(p.bounds) > 0 && m.hasTemporal(p.source, map[string]bool{}) {
		m.emptyTemporalProperty = true
	}
	return err
}

func (m *model) validatePropertyBounds(p temporalProperty) error {
	for _, bound := range p.bounds {
		if bound.Set == nil || m.hasAction(bound.Set, map[string]bool{}) || m.readsState(bound.Set, map[string]bool{}) {
			return fmt.Errorf("temporal quantifier domain must be a constant set")
		}
	}
	return nil
}

func (m *model) addPropertyInstance(p temporalProperty, locals map[string]value.Value) error {
	if p.mode == actionAlways && m.compoundTemporalSafety(p) {
		return liveActionFormatError()
	}
	instance := p
	instance.locals = maps.Clone(locals)
	instance.bounds = nil
	if m.applyJava17ConstantTableau(&instance) {
		m.storePropertyInstance(instance)
		return nil
	}
	m.prepareRecurrenceCache(&instance)
	if instance.mode == generalTemporal {
		if err := m.compileGeneralProperty(&instance); err != nil {
			return err
		}
	}
	m.storePropertyInstance(instance)
	return nil
}

func (m *model) storePropertyInstance(instance temporalProperty) {
	if instance.mode == guardedAlways {
		m.guarded = append(m.guarded, instance)
		m.livenessOrder = append(m.livenessOrder, instance)
	} else if instance.mode == stateAlways || instance.mode == actionAlways || instance.mode == initialOnly {
		m.safety = append(m.safety, instance)
	} else {
		m.properties = append(m.properties, instance)
		m.livenessOrder = append(m.livenessOrder, instance)
	}
}

func (m *model) compileGeneralProperty(instance *temporalProperty) error {
	compiler := newLTLCompiler(m, instance.locals)
	formula, err := compiler.compile(instance.predicate, true, map[string]bool{})
	if err != nil {
		return fmt.Errorf("property %s uses an unsupported temporal expression: %w", instance.name, err)
	}
	if m.propertyUsesTLC(*instance) {
		formula = compiler.markRecurrenceLevels(formula)
	}
	if err := validateFairnessActions(formula); err != nil {
		return err
	}
	if formula.kind == ltlFalse {
		if err := m.temporalTautology(); err != nil {
			return err
		}
	}
	compiler.retainEventualities(formula)
	instance.general, instance.eventualities = formula, compiler.eventuality
	return nil
}

// Model checking rejects a tautology. TLC simulation instead checks the
// remaining solutions, of which a tautology contributes none.
func (m *model) temporalTautology() error {
	if m.tlcRuntime.simulation {
		return nil
	}
	return temporalTautologyError()
}

func temporalTautologyError() error {
	return &ModelValidationError{
		Message:  "Temporal formula is a tautology (its negation is unsatisfiable).",
		Code:     2253,
		ExitCode: 77,
	}
}

func (m *model) holdsProperty(expr *parser.Expression, current evaluator.State, locals map[string]value.Value) (bool, error) {
	result, err := m.program.Evaluate(expr, evaluator.Context{Current: current, Constants: m.constants, Locals: locals, Enabled: m.enabledCallback})
	if err != nil {
		return false, err
	}
	ok, valid := result.Bool()
	if !valid {
		return false, m.nonBooleanPredicateError(expr, result)
	}
	return ok, nil
}

func (m *model) holdsTemporalState(p temporalProperty, state evaluator.State) (bool, error) {
	old := m.tlcRuntime.useBaseLevel
	if p.mode == eventuallyAlways || p.mode == often {
		m.tlcRuntime.useBaseLevel = true
	}
	defer func() { m.tlcRuntime.useBaseLevel = old }()
	return m.holdsProperty(p.predicate, state, p.locals)
}

func (m *model) holdsBaseProperty(expr *parser.Expression, state evaluator.State, locals map[string]value.Value) (bool, error) {
	old := m.tlcRuntime.useBaseLevel
	m.tlcRuntime.useBaseLevel = true
	defer func() { m.tlcRuntime.useBaseLevel = old }()
	return m.holdsProperty(expr, state, locals)
}

func temporalPredicate(expr *parser.Expression) (*parser.Expression, temporalMode) {
	if len(expr.Args) != 1 {
		return nil, eventually
	}
	inner := expr.Args[0]
	if expr.Kind == "Eventually" {
		if inner.Kind == "Always" && len(inner.Args) == 1 {
			return inner.Args[0], eventuallyAlways
		}
		return inner, eventually
	}
	if expr.Kind == "Always" && inner.Kind == "Eventually" && len(inner.Args) == 1 {
		return inner.Args[0], often
	}
	return nil, eventually
}

func (s *search) linkState(parent int, key string) {
	index := len(s.nodes) - 1
	s.indices[key] = index
	// Guarded state properties do not need explicit self-edges: repeating a
	// state cannot change whether their trigger or requirement holds.
	if s.needsLivenessStuttering() {
		s.edges = append(s.edges, []graphEdge{{to: index, action: "Stuttering"}})
	} else {
		s.edges = append(s.edges, nil)
	}
	if parent < 0 {
		s.initial = append(s.initial, index)
	} else {
		s.edges[parent] = append(s.edges[parent], graphEdge{to: index, action: s.nodes[index].action, source: s.nodes[index].source})
	}
}

func (s *search) needsLivenessStuttering() bool {
	return len(s.model.properties) > 0 || len(s.model.fairness) > 0 || s.model.continueChecking && len(s.model.guarded) > 0
}

func (s *search) checkLiveness() (Result, error) {
	if s.model.tlcRuntime.needsLevelGraph {
		return s.checkLivenessWithLevels()
	}
	return s.checkLivenessGraph()
}

func (s *search) checkLivenessGraph() (Result, error) {
	for _, p := range s.model.livenessOrder {
		if p.mode == guardedAlways && !s.model.continueChecking && len(s.model.fairness) == 0 && s.model.tlcRuntime.levelGraph == nil {
			continue
		}
		found, err := s.checkLivenessProperty(p)
		if err != nil {
			return s.result, err
		}
		if found {
			return s.reportLassoProperties()
		}
	}
	return s.result, nil
}

func (s *search) checkLivenessProperty(p temporalProperty) (bool, error) {
	if p.mode == generalTemporal {
		return s.checkGeneralProperty(p)
	}
	if p.mode == guardedAlways {
		return s.checkFairGuardedProperty(p)
	}
	return s.checkProperty(p)
}

func (s *search) checkProperty(p temporalProperty) (bool, error) {
	if p.mode == staysOrOftenState || p.mode == staysOrOftenAction {
		return s.checkStaysOrOften(p)
	}
	return s.checkOtherProperty(p)
}

func (s *search) checkOtherProperty(p temporalProperty) (bool, error) {
	if p.mode == oftenImpliesState || p.mode == oftenImpliesAction {
		return s.checkOftenImplication(p)
	}
	if p.mode == orderedEventually {
		return s.checkOrderedEventually(p)
	}
	if s.canUseUnfairEventually(p) {
		return s.checkUnfairEventually(p)
	}
	if s.needsFairSearch(p) {
		return s.checkFairProperty(p)
	}
	return s.checkSimpleProperty(p)
}

func (s *search) checkSimpleProperty(p temporalProperty) (bool, error) {
	if s.canUseJavaStability(p) {
		if found, handled, err := s.checkJavaStability(p); handled || err != nil {
			return found, err
		}
	}
	for _, i := range s.propertyIndices(p) {
		found, err := s.checkSimplePropertyState(p, i)
		if err != nil || found {
			return found, err
		}
	}
	return false, nil
}

func (s *search) canUseJavaStability(p temporalProperty) bool {
	return p.mode == eventuallyAlways && !s.model.propertyUsesTLC(p) && !s.model.unsafeIncrementalExpression(p.predicate)
}

func (s *search) needsFairSearch(p temporalProperty) bool {
	return s.model.tlcRuntime.levelGraph != nil || len(s.model.fairness) > 0 || p.mode == leadsTo || p.mode == leadsToStaying || p.mode == actionOften || p.mode == often
}

func (s *search) checkSimplePropertyState(p temporalProperty, i int) (bool, error) {
	if err := s.model.ctx.Err(); err != nil {
		return false, err
	}
	holds, err := s.model.holdsProperty(p.predicate, s.nodes[i].state, p.locals)
	if err != nil {
		return false, err
	}
	if !holds && s.stutters(i) {
		s.recordLivenessViolation(p.name, i)
		return true, nil
	}
	return false, nil
}

func (s *search) propertyIndices(p temporalProperty) []int {
	if p.mode == eventually {
		return s.initial
	}
	indices := make([]int, len(s.nodes))
	for i := range indices {
		indices[i] = i
	}
	return indices
}

func (s *search) recordLivenessViolation(name string, index int) {
	trace := s.trace(index)
	loop := len(trace) - 1
	s.result.Status = LivenessViolation
	s.result.Violation = &Violation{Name: name, Trace: trace, LoopStart: &loop, LoopAction: "Stuttering"}
}

func (s *search) stutters(index int) bool {
	for _, edge := range s.edges[index] {
		if edge.to == index {
			return true
		}
	}
	return false
}

func (s *search) canUseUnfairEventually(p temporalProperty) bool {
	return p.mode == eventually && len(s.model.fairness) == 0 && s.model.tlcRuntime.levelGraph == nil
}
