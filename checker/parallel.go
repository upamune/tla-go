package checker

import (
	"maps"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// parallelEligible deliberately accepts only a small, closed expression language.
// In particular, speculative evaluation must not invoke a user operator, TLC,
// lazy values, external callbacks, or any of the liveness and reduction machinery.
func (m *model) parallelEligible(next *parser.Expression) bool {
	if m.tlcRuntime.options.Workers < 2 || runtime.GOMAXPROCS(0) < 2 || m.tlcRuntime.options.SerialExploration || m.parallelFeatures() || m.parallelModules() {
		return false
	}
	return m.parallelExpression(next) && parallelExpressionSize(next) >= 80
}

func (m *model) parallelFeatures() bool {
	c := m.cfg
	return m.parallelCallbacks() || m.viewStates || m.hasReduction() || c.Alias != "" || len(c.Properties)+len(c.Possible)+len(c.Postconditions) != 0 || m.fromSpecification
}

func (m *model) parallelCallbacks() bool {
	return m.coverage != nil || m.graph != nil || m.debug != nil || m.continueChecking
}

func (m *model) parallelModules() bool {
	if !primitiveState(m.constants) || m.parallelInstances() {
		return true
	}
	for _, name := range m.module.Extends {
		if name != "Naturals" && name != "Integers" && name != "Sequences" && name != "FiniteSets" {
			return true
		}
	}
	return false
}

func (m *model) parallelInstances() bool {
	return m.module.HasTLCInstance || len(m.module.InstanceStandardModules) != 0 || len(m.module.Instances) != 0 || len(m.sourceUnits) > 1
}

func (m *model) parallelExpression(e *parser.Expression) bool {
	if e == nil || parallelExtraChildren(e) || !m.parallelExpressionKind(e) {
		return false
	}
	for _, arg := range e.Args {
		if !m.parallelExpression(arg) {
			return false
		}
	}
	return true
}

// Small expressions cannot amortize speculative generation and ordered merging.
func parallelExpressionSize(e *parser.Expression) int {
	n := 1
	for _, arg := range e.Args {
		n += parallelExpressionSize(arg)
	}
	return n
}

func parallelExtraChildren(e *parser.Expression) bool {
	return len(e.Bounds)+len(e.Fields)+len(e.Arms)+len(e.Updates)+len(e.Definitions)+len(e.Instances)+len(e.Selectors)+len(e.EnabledBindings)+len(e.EnabledScopes)+len(e.InstanceScopes) != 0
}

func (m *model) parallelExpressionKind(e *parser.Expression) bool {
	switch e.Kind {
	case "Number", "And", "Or", "Prime", "Unchanged":
		return true
	case "Identifier":
		return m.parallelIdentifier(e)
	case "Apply":
		if _, ok := m.defs[e.Text]; ok {
			return false
		}
		return slices.Contains([]string{"=", "#", "/=", "<", ">", "<=", ">=", "+", "-", "*", "%", "\\div", "~", "=>", "<=>"}, e.Text)
	default:
		return false
	}
}

func (m *model) parallelIdentifier(e *parser.Expression) bool {
	if e.BuiltinNatural || e.BuiltinTLC || e.BuiltinStandard != "" {
		return false
	}
	if m.variables[e.Text] || e.Text == "TRUE" || e.Text == "FALSE" {
		return true
	}
	if _, defined := m.defs[e.Text]; defined {
		return false
	}
	_, constant := m.constants[e.Text]
	return constant
}

type parallelCandidate struct {
	state  evaluator.State
	action string
	source *parser.Expression
}
type parallelExpansion struct {
	candidates []parallelCandidate
	usable     bool
}

// The budget applies to all retained candidates, including duplicates and
// incomplete expansions. Once exhausted, the whole batch is discarded.
const parallelBatchCandidateLimit = 4096

type parallelBudget struct {
	count atomic.Int64
	stop  atomic.Bool
}

func (b *parallelBudget) reserve() bool {
	for !b.stop.Load() {
		n := b.count.Load()
		if n >= parallelBatchCandidateLimit {
			b.stop.Store(true)
			return false
		}
		if b.count.CompareAndSwap(n, n+1) {
			return true
		}
	}
	return false
}

// forkParallelModel has private generator scratch, evaluator caches, ordering,
// and runtime. AST and definition indexes are read only in the accepted subset.
func (m *model) forkParallelModel() *model {
	fork := &model{ctx: m.ctx, module: m.module, cfg: m.cfg, defs: m.defs, variables: m.variables, constants: maps.Clone(m.constants), names: m.names, actionCache: map[*parser.Expression]*parser.Expression{}, letCache: map[*parser.Expression]*parser.Expression{}, valueRanks: maps.Clone(m.valueRanks), rankedSets: map[value.Value][]value.Value{}, rankedFunctions: map[value.Value][]value.Pair{}}
	fork.tlcRuntime = newTLCRuntime(m.cfg, m.tlcRuntime.options)
	fork.tlcRuntime.model = fork
	fork.program = evaluator.New(m.module)
	fork.program.SetOrdering(modelOrdering{model: fork})
	fork.enabledCallback = fork.enabled
	return fork
}

func primitiveState(state evaluator.State) bool {
	for _, v := range state {
		if v.Kind() != value.IntegerKind && v.Kind() != value.BooleanKind {
			return false
		}
	}
	return true
}

func (fork *model) parallelGenerate(next *parser.Expression, state evaluator.State, budget *parallelBudget) parallelExpansion {
	if !primitiveState(state) {
		return parallelExpansion{}
	}
	result := parallelExpansion{usable: true}
	gen := generator{model: fork, current: state, emit: func(state evaluator.State, action string, source *parser.Expression) (bool, error) {
		if !primitiveState(state) || !budget.reserve() {
			result.usable = false
			return false, nil
		}
		result.candidates = append(result.candidates, parallelCandidate{state, action, source})
		return true, nil
	}}
	more, err := gen.solveNext(next, fork.partialState())
	if err != nil || !more {
		result.usable = false
	}
	return result
}

// generateBatch waits for all speculative work before committing any result.
func (s *search) generateBatch(forks []*model, next *parser.Expression, start, end int) ([]parallelExpansion, bool) {
	result := make([]parallelExpansion, end-start)
	budget := &parallelBudget{}
	var group sync.WaitGroup
	for worker, fork := range forks {
		group.Go(func() {
			for i := start + worker; i < end; i += len(forks) {
				if budget.stop.Load() || fork.ctx.Err() != nil {
					return
				}
				result[i-start] = fork.parallelGenerate(next, s.nodes[i].state, budget)
			}
		})
	}
	group.Wait()
	if budget.stop.Load() {
		return nil, false
	}
	return result, true
}

func (s *search) mergeParallel(index int, next *parser.Expression, expansion parallelExpansion) (bool, error) {
	before := s.result.DistinctStates
	for _, c := range expansion.candidates {
		action := c.action
		if action == "" {
			action = s.model.cfg.Next
		}
		more, err := s.visit(c.state, index, action, c.source)
		if err != nil || !more {
			gen := generator{model: s.model}
			addEnclosingActionSource(err, gen.actionRoot(next))
			if s.result.HasOutDegrees {
				s.result.OutDegrees = s.outDegrees
			}
			return more, err
		}
	}
	return s.finishExpansion(index, len(expansion.candidates), before)
}

func (m *model) parallelForks() []*model {
	count := min(m.tlcRuntime.options.Workers, runtime.GOMAXPROCS(0), 32)
	forks := make([]*model, count)
	for i := range forks {
		forks[i] = m.forkParallelModel()
	}
	return forks
}

type parallelSearch struct {
	enabled bool
	forks   []*model
	batch   []parallelExpansion
	start   int
}

func (p *parallelSearch) expand(s *search, i int, next *parser.Expression) (bool, error) {
	if !p.enabled {
		return s.expandNode(i, next)
	}
	if i >= p.start+len(p.batch) && len(s.nodes)-i >= 32 {
		p.generate(s, i, next)
	}
	if i >= p.start && i < p.start+len(p.batch) {
		expansion := p.batch[i-p.start]
		p.batch[i-p.start] = parallelExpansion{}
		if expansion.usable {
			return s.mergeParallel(i, next, expansion)
		}
	}
	return s.expandNode(i, next)
}

func (p *parallelSearch) generate(s *search, i int, next *parser.Expression) {
	if p.forks == nil {
		p.forks = s.model.parallelForks()
	}
	p.start = i
	// Drop the previous batch before allocating the next one.
	p.batch = nil
	var withinBudget bool
	p.batch, withinBudget = s.generateBatch(p.forks, next, i, min(len(s.nodes), i+len(p.forks)*16))
	if !withinBudget {
		p.enabled = false
		p.forks = nil
	}
}
