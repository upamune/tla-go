package evaluator

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (e *evaluator) sequenceOperatorArguments(args []*parser.Expression, arity int) ([]value.Value, operatorRef, error) {
	sequence, err := e.eval(args[0])
	if err != nil {
		return nil, operatorRef{}, err
	}
	items, err := sequence.TupleElements()
	if err != nil {
		return nil, operatorRef{}, err
	}
	op, err := e.operatorArgument(args[1], arity)
	return items, op, err
}

func (e *evaluator) selectSequence(args []*parser.Expression) (value.Value, error) {
	sequence, err := e.eval(args[0])
	if err != nil {
		return value.Value{}, err
	}
	items, err := selectSequenceElements(sequence)
	if err != nil {
		return value.Value{}, err
	}
	op, err := e.operatorArgument(args[1], 1)
	if err != nil {
		return value.Value{}, err
	}
	selected := make([]value.Value, 0, len(items))
	for _, item := range items {
		keep, err := e.selects(op, item)
		if err != nil {
			return value.Value{}, err
		}
		if !keep {
			continue
		}
		selected = append(selected, item)
		if err := e.program.checkMethodSetSize(selectSeqSignature, len(selected)); err != nil {
			return value.Value{}, err
		}
	}
	return value.Tuple(selected)
}

func selectSequenceElements(sequence value.Value) ([]value.Value, error) {
	items, err := sequenceTupleElements(sequence)
	if err != nil && sequence.Kind() != value.StringKind {
		return nil, sequenceArgumentError("first", "SelectSeq", "sequence", sequence)
	}
	return items, err
}

func (e *evaluator) selects(op operatorRef, item value.Value) (bool, error) {
	result, err := e.invokeOperator(op, []value.Value{item})
	if err != nil {
		return false, err
	}
	if b, ok := result.Bool(); ok {
		return b, nil
	}
	name := op.name
	if op.lambda != nil {
		name = "LAMBDA"
	}
	return false, sequenceError(2169, fmt.Sprintf("The second argument of SelectSeq should be a boolean-valued operator, but instead it is:\n<Operator %s>", name))
}

func (e *evaluator) sortSequence(args []*parser.Expression) (value.Value, error) {
	items, op, err := e.sequenceOperatorArguments(args, 2)
	if err != nil {
		return value.Value{}, err
	}
	sorted := make([]value.Value, 0, len(items))
	for _, item := range items {
		index, err := e.sortPosition(op, sorted, item)
		if err != nil {
			return value.Value{}, err
		}
		sorted = append(sorted, value.Value{})
		copy(sorted[index+1:], sorted[index:])
		sorted[index] = item
	}
	return value.Tuple(sorted)
}

func (e *evaluator) sortPosition(op operatorRef, sorted []value.Value, item value.Value) (int, error) {
	for i, other := range sorted {
		result, err := e.invokeOperator(op, []value.Value{item, other})
		if err != nil {
			return 0, err
		}
		if b, ok := result.Bool(); ok && b {
			return i, nil
		}
	}
	return len(sorted), nil
}
