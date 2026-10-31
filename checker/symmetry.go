package checker

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

type permutation struct {
	images map[string]value.Value
	cache  map[value.Value]value.Value
}

func (m *model) prepareSymmetry() error {
	if m.cfg.Symmetry == "" {
		return nil
	}
	def, ok := m.defs[m.cfg.Symmetry]
	if !ok || def.Kind != "Operator" || len(def.Params) > 0 {
		return fmt.Errorf("The symmetry function %s must specify a set of permutations", m.cfg.Symmetry)
	}
	generators, err := m.symmetryGenerators(def.Body)
	if err != nil {
		return err
	}
	if def.Body.Kind == "Apply" && len(def.Body.Args) >= len(generators) {
		return nil
	}
	m.symmetry = symmetryClosure(generators)
	m.symmetryOrder = m.symmetryVariableOrder()
	return nil
}

func (m *model) symmetryGenerators(expr *parser.Expression) ([]permutation, error) {
	v, err := m.eval(expr, nil, nil)
	if err != nil {
		return nil, err
	}
	if !v.IsSet() || !v.IsEnumerable() {
		return nil, fmt.Errorf("The symmetry operator must specify a set of functions")
	}
	elements, err := v.Elements()
	if err != nil {
		return nil, err
	}
	result := make([]permutation, 0, len(elements))
	for _, element := range elements {
		p, err := symmetryPermutation(element, m.cfg.Symmetry)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, nil
}

func symmetryPermutation(v value.Value, name string) (permutation, error) {
	if v.Kind() != value.FunctionKind && v.Kind() != value.LazyFunctionKind {
		return permutation{}, fmt.Errorf("The symmetry values of %s must be functions", name)
	}
	pairs, err := v.Pairs()
	if err != nil {
		return permutation{}, err
	}
	result := permutation{images: map[string]value.Value{}}
	for _, pair := range pairs {
		if pair.Key.Kind() != value.ModelKind || pair.Value.Kind() != value.ModelKind {
			return permutation{}, fmt.Errorf("Symmetry function must have model values as domain and range")
		}
		k, _ := pair.Key.Text()
		image, _ := pair.Value.Text()
		if k != image {
			result.images[k] = pair.Value
		}
	}
	return result, nil
}

func symmetryClosure(generators []permutation) []permutation {
	var group []permutation
	seen := map[string]bool{}
	add := func(p permutation) {
		if len(p.images) == 0 {
			return
		}
		key := permutationKey(p)
		if !seen[key] {
			seen[key] = true
			p.cache = make(map[value.Value]value.Value)
			group = append(group, p)
		}
	}
	for _, p := range generators {
		add(p)
	}
	for i := 0; i < len(group); i++ {
		for _, p := range generators {
			add(composePermutations(p, group[i]))
		}
	}
	return group
}

func permutationKey(p permutation) string {
	var b strings.Builder
	for _, name := range slices.Sorted(maps.Keys(p.images)) {
		image, _ := p.images[name].Text()
		fmt.Fprintf(&b, "%d:%s%d:%s", len(name), name, len(image), image)
	}
	return b.String()
}

func composePermutations(first, second permutation) permutation {
	result := permutation{images: map[string]value.Value{}}
	names := map[string]bool{}
	for name := range first.images {
		names[name] = true
	}
	for name := range second.images {
		names[name] = true
	}
	for name := range names {
		image, ok := first.images[name]
		if !ok {
			image = value.Model(name)
		}
		imageName, _ := image.Text()
		if next, ok := second.images[imageName]; ok {
			image = next
			imageName, _ = image.Text()
		}
		if imageName != name {
			result.images[name] = image
		}
	}
	return result
}

// symmetryKey only encodes the representative, so it reuses one map for it.
// A nested key, evaluated for a VIEW or a lazy value, gets its own map.
func (m *model) symmetryKey(state evaluator.State) (string, error) {
	scratch := m.symmetryState
	m.symmetryState = nil
	if scratch == nil {
		scratch = make(evaluator.State, len(m.names))
	}
	representative, err := m.symmetryRepresentative(state, scratch)
	if err != nil {
		return "", err
	}
	var key string
	if m.cfg.View != "" {
		key, err = m.viewKey(representative)
	} else {
		key, err = m.unreducedStateKey(representative)
	}
	m.symmetryState = scratch
	return key, err
}

// symmetryRepresentative returns the smallest permutation of state. A
// smaller permutation is built in scratch when it is not nil.
func (m *model) symmetryRepresentative(state, scratch evaluator.State) (evaluator.State, error) {
	minimum := state
	// Permuting a lazy value can evaluate a nested key, which must not share the prefix.
	prefix := m.symmetryPrefix
	m.symmetryPrefix = nil
	if cap(prefix) < len(m.symmetryOrder) {
		prefix = make([]value.Value, 0, len(m.symmetryOrder))
	}
	for _, p := range m.symmetry {
		candidate, smaller, err := m.smallerPermutation(state, minimum, p, prefix[:0], scratch)
		if err != nil {
			return nil, err
		}
		if smaller {
			minimum = candidate
		}
	}
	m.symmetryPrefix = prefix
	return minimum, nil
}

// smallerPermutation compares the permuted state with minimum variable by
// variable and builds the permuted state only when it is smaller. prefix
// holds the permuted values compared so far. Minimum may be scratch: it is
// no longer needed once the permutation is known to be smaller.
func (m *model) smallerPermutation(state, minimum evaluator.State, p permutation, prefix []value.Value, scratch evaluator.State) (evaluator.State, bool, error) {
	for _, name := range m.symmetryOrder {
		v, err := permuteValue(state[name], p)
		if err != nil {
			return nil, false, err
		}
		prefix = append(prefix, v)
		compare, err := m.compareOrderedValues(v, minimum[name])
		if err != nil {
			return nil, false, err
		}
		if compare > 0 {
			return nil, false, nil
		}
		if compare < 0 {
			return m.permuteRemaining(state, prefix, p, scratch)
		}
	}
	return nil, false, nil
}

func (m *model) permuteRemaining(state evaluator.State, prefix []value.Value, p permutation, scratch evaluator.State) (evaluator.State, bool, error) {
	candidate := scratch
	if candidate == nil {
		candidate = make(evaluator.State, len(state))
	}
	clear(candidate)
	for i, v := range prefix {
		candidate[m.symmetryOrder[i]] = v
	}
	for _, name := range m.names {
		if _, ok := candidate[name]; ok {
			continue
		}
		v, err := permuteValue(state[name], p)
		if err != nil {
			return nil, false, err
		}
		candidate[name] = v
	}
	return candidate, true, nil
}
