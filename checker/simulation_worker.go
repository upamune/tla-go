package checker

import (
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// simulationWorker uses TLC's worker-local random stream to choose an action
// before choosing a successor. Its callers validate every generated successor.
type simulationWorker struct {
	model    *model
	actions  []simulationAction
	random   *evaluator.JavaRandom
	selected simulationAction
}

func newSimulationWorker(m *model, actions []simulationAction, master *evaluator.JavaRandom) simulationWorker {
	return simulationWorker{model: m, actions: actions, random: evaluator.NewJavaRandom(master.Long())}
}

func (w *simulationWorker) randomState(states []evaluator.State) evaluator.State {
	if len(states) == 0 {
		return nil
	}
	return states[int(w.random.Float64()*float64(len(states)))]
}

func (w *simulationWorker) nextStates(current evaluator.State) ([]evaluator.State, error) {
	count := len(w.actions)
	index := int(w.random.Float64() * float64(count))
	stride := int(w.random.Prime())
	for range count {
		if err := w.model.ctx.Err(); err != nil {
			return nil, err
		}
		states, err := w.actionStates(w.actions[index], current)
		if err != nil || len(states) != 0 {
			w.selected = w.actions[index]
			return states, err
		}
		index = (index + stride) % count
	}
	return nil, nil
}

func (w *simulationWorker) actionStates(action simulationAction, current evaluator.State) ([]evaluator.State, error) {
	var states []evaluator.State
	gen := generator{model: w.model, current: current, lexical: action.scope, allowIncomplete: true, emit: func(state evaluator.State, _ string, _ *parser.Expression) (bool, error) {
		states = append(states, state)
		return true, nil
	}}
	_, err := gen.solve([]work{{expr: action.expr, locals: action.locals}}, evaluator.State{}, action.name)
	return states, err
}
