package evaluator

import (
	"fmt"

	"github.com/upamune/tla-go/value"
)

func (e *evaluator) randomSetsOfSubsets(name string, args []value.Value) (value.Value, error) {
	picks, size, elements, err := e.subsetsArguments(args)
	if err != nil {
		return value.Value{}, err
	}
	random, err := e.randomGenerator()
	if err != nil {
		return value.Value{}, err
	}
	if name == "RandomSetOfSubsets" {
		return e.tossSubsets(picks, size, elements, random)
	}
	counts := make([]value.Value, 5)
	for i := range counts {
		set, err := e.tossSubsets(picks, size, elements, random)
		if err != nil {
			return value.Value{}, err
		}
		count, err := set.Cardinality()
		if err != nil {
			return value.Value{}, err
		}
		counts[i], err = value.Integer(count)
		if err != nil {
			return value.Value{}, err
		}
	}
	return value.Tuple(counts)
}

func (e *evaluator) subsetsArguments(args []value.Value) (int, int, []value.Value, error) {
	picks, err := randomNonnegative(args[0], "first")
	if err != nil {
		return 0, 0, nil, err
	}
	size, err := randomNonnegative(args[1], "second")
	if err != nil {
		return 0, 0, nil, err
	}
	if !args[2].IsSet() || !args[2].IsEnumerable() {
		return 0, 0, nil, randomArgumentError("third", "RandomSetOfSubsets", "finite set", args[2])
	}
	elements, err := e.randomElements(args[2])
	if err != nil {
		return 0, 0, nil, err
	}
	if err = validateSubsetSizes(picks, size, len(elements)); err != nil {
		return 0, 0, nil, err
	}
	return picks, size, elements, nil
}

func randomNonnegative(v value.Value, position string) (int, error) {
	n, ok := v.Int()
	if !ok || n < 0 {
		return 0, randomArgumentError(position, "RandomSetOfSubsets", "nonnegative integer", v)
	}
	return int(n), nil
}

func validateSubsetSizes(picks, size, length int) error {
	if length < 31 && uint64(picks) > uint64(1)<<uint(length) {
		actual, _ := value.Integer(int64(picks))
		return randomArgumentError("first", "RandomSetOfSubsets", fmt.Sprintf("nonnegative integer that is smaller than the subset's size of 2^%d", length), actual)
	}
	if size > length {
		actual, _ := value.Integer(int64(size))
		return randomArgumentError("second", "RandomSetOfSubsets", "nonnegative integer in range 0..Cardinality(S)", actual)
	}
	return nil
}

func (e *evaluator) tossSubsets(picks, size int, elements []value.Value, random *JavaRandom) (value.Value, error) {
	probability := float64(0)
	if len(elements) != 0 {
		probability = float64(size) / float64(len(elements))
	}
	sets := make([]value.Value, 0, picks)
	for range picks {
		items := make([]value.Value, 0)
		for _, v := range elements {
			if random.Float64() < probability {
				items = append(items, v)
			}
		}
		set, err := e.orderedSet(items)
		if err != nil {
			return value.Value{}, err
		}
		sets = append(sets, set)
	}
	return e.orderedSet(sets)
}
