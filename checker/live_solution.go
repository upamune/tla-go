package checker

import (
	"fmt"

	"github.com/upamune/tla-go/evaluator"
)

// liveSolution is TLC's OrderOfSolution: one tableau formula and the possible
// error models that share it.
type liveSolution struct {
	tableau     *liveTableau
	promises    []*liveNode
	checkState  []*liveNode
	checkAction []*liveNode
	pems        []livePEM
}

// livePEM is a PossibleErrorModel. Its fields index checkState and checkAction.
type livePEM struct{ aeAction, aeState, eaAction []int }

type livePEMFormulas struct{ eaAction, aeState, aeAction, temporal []*liveNode }

// liveSolutions follows Liveness.processLiveness: it negates the implied
// temporal properties, conjoins the fairness assumptions and groups the
// disjunctive normal form by tableau formula.
func (m *model) liveSolutions() ([]liveSolution, error) {
	formula, err := newLiveConverter(m).liveness()
	if formula == nil || err != nil {
		return nil, err
	}
	formula = formula.simplify().toDNF()
	if formula.kind == liveBool && !formula.value {
		return nil, nil
	}
	disjuncts := []*liveNode{formula}
	if formula.kind == liveDisj {
		disjuncts = formula.items
	}
	return groupLiveSolutions(disjuncts)
}

func groupLiveSolutions(disjuncts []*liveNode) ([]liveSolution, error) {
	var temporal []*liveNode
	var groups [][]livePEMFormulas
	for _, disjunct := range disjuncts {
		pem, err := classifyLiveDisjunct(disjunct.flattenSingleJunctions())
		if err != nil {
			return nil, err
		}
		formula := pem.temporalFormula()
		index := liveTemporalIndex(temporal, formula)
		if index == len(temporal) {
			temporal = append(temporal, formula)
			groups = append(groups, nil)
		}
		groups[index] = append(groups[index], pem)
	}
	solutions := make([]liveSolution, len(temporal))
	for i, formula := range temporal {
		solutions[i] = newLiveSolution(formula, groups[i])
	}
	return solutions, nil
}

func liveTemporalIndex(formulas []*liveNode, formula *liveNode) int {
	for i, existing := range formulas {
		if existing == nil && formula == nil || existing != nil && formula != nil && liveEqual(existing, formula) {
			return i
		}
	}
	return len(formulas)
}

func classifyLiveDisjunct(disjunct *liveNode) (livePEMFormulas, error) {
	var pem livePEMFormulas
	items := []*liveNode{disjunct}
	if disjunct.kind == liveConj {
		items = disjunct.items
	}
	for _, item := range items {
		if err := pem.classify(item); err != nil {
			return pem, err
		}
	}
	return pem, nil
}

// classify sorts <>[]A, []<>S, []<>A and remaining action-free formulas.
func (p *livePEMFormulas) classify(item *liveNode) error {
	if body := item.eaBody(); body != nil && body.level() < 3 {
		p.eaAction = append(p.eaAction, body)
		return nil
	}
	if body := item.aeBody(); body != nil && body.level() <= 2 {
		if body.level() <= 1 {
			p.aeState = append(p.aeState, body)
		} else {
			p.aeAction = append(p.aeAction, body)
		}
		return nil
	}
	if item.containsAction() {
		return liveActionFormatError()
	}
	p.temporal = append(p.temporal, item)
	return nil
}

func liveActionFormatError() error {
	return &ModelValidationError{Message: "Temporal formulas containing actions must be of forms <>[]A or []<>A.", Code: 2214, ExitCode: 255}
}

func (p livePEMFormulas) temporalFormula() *liveNode {
	switch len(p.temporal) {
	case 0:
		return nil
	case 1:
		return p.temporal[0]
	}
	result := &liveNode{kind: liveConj}
	for _, item := range p.temporal {
		result.add(item)
	}
	return result
}

func newLiveSolution(formula *liveNode, pems []livePEMFormulas) liveSolution {
	var solution liveSolution
	if formula != nil {
		binary := formula.makeBinary()
		promises := &livePar{}
		binary.extractPromises(promises)
		solution.tableau, solution.promises = newLiveTableau(binary), promises.items
	}
	for _, pem := range pems {
		solution.pems = append(solution.pems, livePEM{
			aeAction: liveBin(pem.aeAction, &solution.checkAction),
			aeState:  liveBin(pem.aeState, &solution.checkState),
			eaAction: liveBin(pem.eaAction, &solution.checkAction),
		})
	}
	return solution
}

func liveBin(checks []*liveNode, bin *[]*liveNode) []int {
	indices := make([]int, len(checks))
	for i, check := range checks {
		indices[i] = liveTemporalIndex(*bin, check)
		if indices[i] == len(*bin) {
			*bin = append(*bin, check)
		}
	}
	return indices
}

// liveConverter builds TLC's LiveExprNode tree from compiled properties and
// fairness conditions. Every leaf receives a distinct tag.
type liveConverter struct {
	model  *model
	leaves map[int]*liveLeaf
	tags   int
}

func newLiveConverter(m *model) *liveConverter { return &liveConverter{model: m} }

func (c *liveConverter) liveness() (*liveNode, error) {
	if len(c.model.livenessOrder) == 0 {
		return nil, nil
	}
	result := &liveNode{kind: liveConj}
	for _, fair := range c.model.fairness {
		node, err := c.fairness(fair)
		if err != nil {
			return nil, err
		}
		result.add(node)
	}
	negated, err := c.negatedProperties()
	if err != nil || len(result.items) == 0 {
		return negated, err
	}
	result.add(negated)
	return result, nil
}

func (c *liveConverter) negatedProperties() (*liveNode, error) {
	result := &liveNode{kind: liveDisj}
	for _, property := range c.model.livenessOrder {
		node, err := c.property(property)
		if err != nil {
			return nil, err
		}
		result.items = append(result.items, liveUnary(liveNeg, node))
	}
	if len(result.items) == 1 {
		return result.items[0], nil
	}
	return result, nil
}

func (c *liveConverter) property(property temporalProperty) (*liveNode, error) {
	if c.model.expressionUsesTLC(property.source, map[string]bool{}) {
		return nil, fmt.Errorf("simulation of temporal property %s using TLCGet is not yet supported", property.name)
	}
	compiler := newLTLCompiler(c.model, property.locals)
	formula, err := compiler.compile(property.source, false, map[string]bool{})
	if err != nil {
		return nil, fmt.Errorf("property %s uses an unsupported temporal expression: %w", property.name, err)
	}
	c.leaves = map[int]*liveLeaf{}
	return c.formula(formula)
}

func (c *liveConverter) formula(formula *ltlFormula) (*liveNode, error) {
	switch formula.kind {
	case ltlTrue, ltlFalse:
		return liveBoolNode(formula.kind == ltlTrue), nil
	case ltlAtom, ltlAction:
		return c.atom(formula)
	case ltlAnd, ltlOr:
		return c.junction(formula)
	}
	body, err := c.formula(formula.left)
	if formula.kind == ltlEventually {
		return liveUnary(liveEven, body), err
	}
	return liveUnary(liveAll, body), err
}

func (c *liveConverter) junction(formula *ltlFormula) (*liveNode, error) {
	result := &liveNode{kind: liveConj}
	if formula.kind == ltlOr {
		result.kind = liveDisj
	}
	for _, part := range []*ltlFormula{formula.left, formula.right} {
		node, err := c.formula(part)
		if err != nil {
			return nil, err
		}
		result.add(node)
	}
	return result, nil
}

// TLC evaluates constant-level leaves while it converts the formula.
func (c *liveConverter) atom(formula *ltlFormula) (*liveNode, error) {
	m := c.model
	if formula.kind == ltlAtom && !m.readsState(formula.atom, map[string]bool{}) && !m.hasAction(formula.atom, map[string]bool{}) {
		holds, err := m.holdsProperty(formula.atom, nil, formula.locals)
		return liveBoolNode(holds != formula.negative), err
	}
	leaf, ok := c.leaves[formula.leaf]
	if !ok {
		leaf = c.newLeaf(c.formulaEvaluator(formula))
		c.leaves[formula.leaf] = leaf
	}
	node := &liveNode{kind: liveState, leaf: leaf}
	if formula.kind == ltlAction {
		node.kind = liveAction
	}
	if formula.negative {
		return liveUnary(liveNeg, node), nil
	}
	return node, nil
}

// formulaEvaluator evaluates the body of an LNStateAST or LNAction leaf.
// TLC's liveness checker reports a non-boolean value with its own error.
func (c *liveConverter) formulaEvaluator(formula *ltlFormula) func(current, next evaluator.State) (bool, error) {
	m, action := c.model, formula.kind == ltlAction
	return func(current, next evaluator.State) (bool, error) {
		context := evaluator.Context{Current: current, Constants: m.constants, Locals: formula.locals, Enabled: m.enabledCallback}
		if action {
			context.Next = next
		}
		v, err := m.program.Evaluate(formula.atom, context)
		if err != nil {
			return false, err
		}
		result, ok := v.Bool()
		if !ok {
			return false, liveNonBoolean(action)
		}
		return result, nil
	}
}

// liveNonBoolean is the error of TLC's liveness checker for a leaf whose
// value is not a Boolean.
func liveNonBoolean(action bool) error {
	if action {
		return &evaluator.RuntimeError{Message: "Encountered an action predicate that's not a boolean.", ToolCode: 2252}
	}
	return &evaluator.RuntimeError{Message: "A state predicate was evaluated to a non-boolean value.", ToolCode: 2250}
}

func (c *liveConverter) newLeaf(eval func(current, next evaluator.State) (bool, error)) *liveLeaf {
	c.tags++
	return &liveLeaf{tag: c.tags, eval: eval}
}

// fairness follows Liveness.astToLive for WF, SF and []<> conjuncts of the
// behavior specification.
func (c *liveConverter) fairness(fair fairnessCondition) (*liveNode, error) {
	if c.model.expressionUsesTLC(fair.action, map[string]bool{}) {
		return nil, fmt.Errorf("simulation of fairness using TLCGet is not yet supported")
	}
	if fair.recurrence {
		return c.recurrence(fair)
	}
	m := c.model
	enabled := &liveNode{kind: liveState, leaf: c.newLeaf(func(current, _ evaluator.State) (bool, error) {
		return m.enabledWithLocals(fairAngle(fair), current, nil, fair.locals)
	})}
	step := &liveNode{kind: liveAction, leaf: c.newLeaf(func(current, next evaluator.State) (bool, error) {
		return m.holdsFair(fair, current, next)
	})}
	disabled := liveUnary(liveNeg, enabled)
	if fair.strong {
		return liveJunction(liveDisj, liveUnary(liveEven, liveUnary(liveAll, disabled)), liveUnary(liveAll, liveUnary(liveEven, step))), nil
	}
	return liveUnary(liveAll, liveUnary(liveEven, liveJunction(liveDisj, disabled, step))), nil
}

func (c *liveConverter) recurrence(fair fairnessCondition) (*liveNode, error) {
	formula := &ltlFormula{kind: ltlAtom, atom: fair.action, locals: fair.locals}
	if fair.recurrenceAction {
		formula.kind = ltlAction
	}
	c.leaves = map[int]*liveLeaf{}
	body, err := c.atom(formula)
	return liveUnary(liveAll, liveUnary(liveEven, body)), err
}
