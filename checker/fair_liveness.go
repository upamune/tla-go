package checker

import (
	"slices"

	"github.com/upamune/tla-go/syntax/parser"
)

type fairSearch struct {
	search              *search
	property            temporalProperty
	bad                 []bool
	trigger             []bool
	parent              []int
	via                 []string
	viaSource           []*parser.Expression
	enabled             [][]int8
	guarded             *guardedMonitor
	required            []bool
	requiredAction      *temporalProperty
	recurrenceWitnesses []cycleWitness
	recurrenceKnown     []bool
	acceptance          [][]bool
}

func (s *search) checkFairProperty(p temporalProperty) (bool, error) {
	if p.mode == actionOften {
		filtered, err := s.withoutPropertyEdges(p)
		if err != nil {
			return false, err
		}
		found, err := filtered.checkFairGraph(p)
		s.result = filtered.result
		return found, err
	}
	return s.checkFairGraph(p)
}

func (s *search) checkFairGraph(p temporalProperty) (bool, error) {
	if p.mode == leadsTo && s.model.cfg.Symmetry != "" {
		general := s.reducedLeadsTo(p)
		if s.supportsReducedTableau(general) {
			return s.checkReducedTableau(general)
		}
	}
	if s.model.hasReduction() && p.mode == eventually && len(s.model.fairness) > 0 {
		graph := s.viewTableauGraph()
		found, err := graph.checkFairGraphRequired(p, nil, false, false)
		s.result = graph.result
		return found, err
	}
	return s.checkFairGraphRequired(p, nil, false, false)
}

func (s *search) checkFairGraphRequired(p temporalProperty, required *parser.Expression, action, inverted bool) (bool, error) {
	f := fairSearch{search: s, property: p, bad: make([]bool, len(s.nodes)), trigger: make([]bool, len(s.nodes)), enabled: make([][]int8, len(s.model.fairness))}
	for i := range f.enabled {
		if !s.model.fairness[i].recurrence {
			f.enabled[i] = make([]int8, len(s.nodes))
		}
	}
	if err := f.evaluateStates(); err != nil {
		return false, err
	}
	if err := f.evaluateRequired(required, action, inverted); err != nil {
		return false, err
	}
	allowed := f.reachableStates()
	f.debugStates(allowed)
	if f.canUseJavaLeadsTo() {
		found, handled, err := f.checkJavaLeadsTo()
		if handled || err != nil {
			return found, err
		}
	}
	return f.findFairCycle(allowed)
}

func (f *fairSearch) evaluateStates() error {
	if f.property.mode == actionOften || f.property.mode == guardedAlways {
		return nil
	}
	for i, node := range f.search.nodes {
		if err := f.search.model.ctx.Err(); err != nil {
			return err
		}
		if err := f.evaluateState(i, node); err != nil {
			return err
		}
	}
	return nil
}

func (f *fairSearch) evaluateState(index int, node stateNode) error {
	ok, err := f.holdsCachedTemporalState(index, node)
	if err != nil {
		return err
	}
	f.bad[index] = !ok
	if f.property.mode == leadsToStaying || (f.property.mode == leadsTo && f.bad[index]) {
		f.trigger[index], err = f.search.model.holdsProperty(f.property.trigger, node.state, f.property.locals)
	}
	return err
}

func (f *fairSearch) holdsCachedTemporalState(index int, node stateNode) (bool, error) {
	if f.property.cacheAtom != nil {
		key := ltlValueKey{formula: f.property.cacheAtom, base: index, choice: -1}
		if value, ok := f.search.ltlValues[key]; ok {
			return value, nil
		}
	}
	return f.search.model.holdsTemporalState(f.property, node.state)
}

func (f *fairSearch) reachableStates() []bool {
	s := f.search
	allowed := make([]bool, len(s.nodes))
	f.parent = make([]int, len(s.nodes))
	f.via = make([]string, len(s.nodes))
	f.viaSource = make([]*parser.Expression, len(s.nodes))
	queue := make([]int, 0, len(s.nodes))
	f.addReachabilitySeeds(allowed, &queue)
	for head := 0; head < len(queue); head++ {
		f.discoverNeighbors(queue[head], allowed, &queue)
	}
	if f.property.mode == often {
		for i := range allowed {
			allowed[i] = allowed[i] && f.bad[i]
		}
	}
	return allowed
}

func (f *fairSearch) addReachabilitySeeds(allowed []bool, queue *[]int) {
	if f.property.mode == actionOften {
		for node := range f.search.nodes {
			allowed[node] = true
			f.parent[node] = -1
			*queue = append(*queue, node)
		}
		return
	}
	if seededProperty(f.property.mode) {
		f.addTriggerSeeds(allowed, queue)
		return
	}
	for _, initial := range f.search.initial {
		if f.property.mode != eventually || f.bad[initial] {
			allowed[initial] = true
			f.parent[initial] = -1
			*queue = append(*queue, initial)
		}
	}
}

func seededProperty(mode temporalMode) bool {
	return mode == leadsTo || mode == leadsToStaying || mode == guardedAlways
}

func (f *fairSearch) addTriggerSeeds(allowed []bool, queue *[]int) {
	for node, trigger := range f.trigger {
		if trigger {
			allowed[node] = true
			f.parent[node] = -1
			*queue = append(*queue, node)
		}
	}
}

func (f *fairSearch) discoverNeighbors(node int, allowed []bool, queue *[]int) {
	for _, edge := range f.search.edges[node] {
		if allowed[edge.to] || ((f.property.mode == eventually || f.property.mode == leadsTo) && !f.bad[edge.to]) {
			continue
		}
		allowed[edge.to] = true
		f.parent[edge.to] = node
		f.via[edge.to] = edge.action
		f.viaSource[edge.to] = edge.source
		*queue = append(*queue, edge.to)
	}
}

func (f *fairSearch) findFairCycle(allowed []bool) (bool, error) {
	queue := [][]bool{allowed}
	for len(queue) > 0 {
		if err := f.search.model.ctx.Err(); err != nil {
			return false, err
		}
		allowed, queue = queue[0], queue[1:]
		found, pruned, err := f.checkComponents(f.search.components(allowed))
		if err != nil || found {
			return found, err
		}
		queue = append(queue, pruned...)
	}
	return false, nil
}

func (f *fairSearch) checkComponents(components [][]int) (bool, [][]bool, error) {
	var pending [][]bool
	for _, component := range slices.Backward(components) {

		valid, pruned, err := f.checkComponent(component)
		f.debugComponent(component, valid, pruned != nil)
		if err != nil {
			return false, nil, err
		}
		if valid {
			return true, nil, f.recordPreferredCycle(component)
		}
		if pruned != nil {
			pending = append(pending, pruned)
		}
	}
	return false, pending, nil
}

func (f *fairSearch) recordPreferredCycle(component []int) error {
	if f.property.mode == generalTemporal && f.search.lassoRuntimeMayChangeVerdict() && f.safePreferredCycle() {
		single, err := f.acceptingSelfLoop(component)
		if err != nil {
			return err
		}
		if single != nil {
			component = single
		}
	}
	if f.shouldShortenReplayCycle() {
		if found := f.recordShortestTableauCycle(component); found {
			return nil
		}
	}
	return f.recordCycle(component)
}

func (f *fairSearch) safePreferredCycle() bool {
	for _, fair := range f.search.model.fairness {
		if f.search.model.expressionHasEffects(fair.action, map[string]bool{}) || f.search.model.readsRandom(fair.action, map[string]bool{}) {
			return false
		}
	}
	return true
}

func (f *fairSearch) shouldShortenReplayCycle() bool {
	return f.property.mode == generalTemporal && len(f.search.model.fairness) == 0 && f.search.model.lassoReplayHasEffects()
}

func (f *fairSearch) acceptingSelfLoop(component []int) ([]int, error) {
	for _, node := range component {
		if !f.search.stutters(node) {
			continue
		}
		single := []int{node}
		candidate := *f
		candidate.recurrenceWitnesses = slices.Clone(f.recurrenceWitnesses)
		candidate.recurrenceKnown = slices.Clone(f.recurrenceKnown)
		valid, _, err := candidate.checkComponent(single)
		if err != nil {
			return nil, err
		}
		if valid {
			f.recurrenceWitnesses, f.recurrenceKnown = candidate.recurrenceWitnesses, candidate.recurrenceKnown
			return single, nil
		}
	}
	return nil, nil
}

func (f *fairSearch) checkComponent(component []int) (bool, []bool, error) {
	if !f.acceptsComponent(component) {
		return false, nil, nil
	}
	members := make([]bool, len(f.search.nodes))
	for _, node := range component {
		members[node] = true
	}
	if ok, err := f.acceptsRequiredAction(component, members); err != nil || !ok {
		return false, nil, err
	}
	for index, fair := range f.search.model.fairness {
		valid, prune, err := f.checkFairness(index, fair, component, members)
		if err != nil || !valid {
			return false, prune, err
		}
	}
	return true, nil, nil
}

func (f *fairSearch) acceptsComponent(component []int) bool {
	if len(component) == 1 && !f.search.stutters(component[0]) {
		return false
	}
	if (f.property.mode == eventuallyAlways || f.property.mode == leadsToStaying) && !componentContains(component, f.bad) {
		return false
	}
	if !f.acceptsTableau(component) {
		return false
	}
	return len(f.required) == 0 || componentContains(component, f.required)
}

func (f *fairSearch) acceptsTableau(component []int) bool {
	for _, acceptance := range f.acceptance {
		if !componentContains(component, acceptance) {
			return false
		}
	}
	return true
}

func componentContains(component []int, marked []bool) bool {
	for _, node := range component {
		if marked[node] {
			return true
		}
	}
	return false
}

func fairAngle(fair fairnessCondition) *parser.Expression {
	return &parser.Expression{Kind: "AngleAction", Args: []*parser.Expression{fair.action, fair.subscript}}
}
