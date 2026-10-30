package value

import "strings"

// TLC orders sets by cardinality and then by their elements. Enumerating
// subsets in that order avoids materializing the entire power set merely to
// produce its canonical state key.
func (v Value) appendSubsetKey(b *strings.Builder) error {
	base, err := v.subsetBase()
	if err != nil {
		return v.withFingerprintSource(err)
	}
	if err := sortValues(base); err != nil {
		return v.withFingerprintSource(err)
	}
	b.WriteByte('S')
	writeInt(b, int64(1)<<len(base))
	for size := 0; size <= len(base); size++ {
		if err := appendSubsetSize(b, base, size); err != nil {
			return err
		}
	}
	return nil
}

func appendSubsetSize(b *strings.Builder, base []Value, size int) error {
	indices := make([]int, size)
	for i := range indices {
		indices[i] = i
	}
	for {
		if err := appendSubsetMembers(b, base, indices); err != nil {
			return err
		}
		if !advanceCombination(indices, len(base)) {
			return nil
		}
	}
}

func appendSubsetMembers(b *strings.Builder, base []Value, indices []int) error {
	if low, high, ok := contiguousSubset(base, indices); ok {
		b.WriteByte('r')
		writeInt(b, int64(low))
		writeInt(b, int64(high))
		return nil
	}
	b.WriteByte('S')
	writeInt(b, int64(len(indices)))
	for _, index := range indices {
		if err := base[index].appendKey(b); err != nil {
			return err
		}
	}
	return nil
}

func contiguousSubset(base []Value, indices []int) (int32, int32, bool) {
	if len(indices) == 0 {
		return 0, -1, true
	}
	low, ok := base[indices[0]].Int()
	if !ok {
		return 0, 0, false
	}
	for i, index := range indices {
		n, integer := base[index].Int()
		if !integer || int64(n) != int64(low)+int64(i) {
			return 0, 0, false
		}
	}
	return low, base[indices[len(indices)-1]].number, true
}
