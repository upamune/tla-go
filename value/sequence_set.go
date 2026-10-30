package value

// Sequences represents the set of all finite sequences over base.
func Sequences(base Value) Value {
	return Value{kind: SequenceSetKind, data: &payload{domain: base}}
}

func (v Value) containsSequence(element Value) (bool, error) {
	if element.kind == ModelKind {
		return false, nil
	}
	elements, err := element.TupleElements()
	if err != nil {
		return false, err
	}
	for _, item := range elements {
		found, err := v.data.domain.Contains(item)
		if err != nil || !found {
			return false, err
		}
	}
	return true, nil
}

func (v Value) sequenceSetIsFinite() (bool, error) {
	finite, err := v.data.domain.IsFinite()
	if err != nil || !finite {
		return finite, err
	}
	return v.data.domain.isEmpty()
}
