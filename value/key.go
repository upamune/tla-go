package value

import (
	"fmt"
	"strconv"
	"strings"
)

// Key encodes a value canonically for exact state identity. It is not TLC's
// 64-bit fingerprint; callers can use Go's map without fingerprint collisions.
func (v Value) Key() (string, error) {
	var b strings.Builder
	if err := v.appendKey(&b); err != nil {
		return "", err
	}
	return b.String(), nil
}

func (v Value) appendKey(b *strings.Builder) error {
	switch v.kind {
	case IntegerKind:
		b.WriteByte('i')
		writeInt(b, int64(v.number))
	case BooleanKind:
		b.WriteByte('b')
		writeInt(b, int64(v.number))
	case StringKind:
		b.WriteByte('s')
		writeString(b, v.data.text)
	case ModelKind:
		b.WriteByte('m')
		writeString(b, v.data.text)
	case SetKind, IntervalKind:
		return v.setKey(b)
	case FunctionKind:
		return v.functionKey(b)
	default:
		return v.appendSpecialKey(b)
	}
	return nil
}

func writeInt(b *strings.Builder, n int64)     { b.WriteString(strconv.FormatInt(n, 10)); b.WriteByte(':') }
func writeString(b *strings.Builder, s string) { writeInt(b, int64(len(s))); b.WriteString(s) }
func (v Value) setKey(b *strings.Builder) error {
	low, high, ok := v.contiguous()
	if ok {
		b.WriteByte('r')
		writeInt(b, int64(low))
		writeInt(b, int64(high))
		return nil
	}
	b.WriteByte('S')
	writeInt(b, int64(len(v.data.elements)))
	for _, element := range v.data.elements {
		if err := element.appendKey(b); err != nil {
			return err
		}
	}
	return nil
}

func (v Value) contiguous() (int32, int32, bool) {
	if v.kind == IntervalKind {
		if v.high < v.number {
			return 0, -1, true
		}
		return v.number, v.high, true
	}
	if len(v.data.elements) == 0 {
		return 0, -1, true
	}
	low, ok := v.data.elements[0].Int()
	if !ok {
		return 0, 0, false
	}
	for i, element := range v.data.elements {
		n, ok := element.Int()
		if !ok || int64(n) != int64(low)+int64(i) {
			return 0, 0, false
		}
	}
	return low, v.data.elements[len(v.data.elements)-1].number, true
}

func (v Value) functionKey(b *strings.Builder) error {
	b.WriteByte('f')
	writeInt(b, int64(len(v.data.pairs)))
	for _, pair := range v.data.pairs {
		if err := pair.Key.appendKey(b); err != nil {
			return err
		}
		if err := pair.Value.appendKey(b); err != nil {
			return err
		}
	}
	return nil
}

func (v Value) appendSpecialKey(b *strings.Builder) error {
	if v.kind == LazyFunctionKind {
		f, err := v.materializedFunction()
		if err != nil {
			return err
		}
		return f.functionKey(b)
	}
	if v.kind == InfiniteSetKind {
		b.WriteByte('I')
		writeInt(b, int64(v.number))
		return nil
	}
	if v.kind == SequenceSetKind {
		b.WriteByte('q')
		return v.data.domain.appendKey(b)
	}
	if !isLazySet(v) {
		return fmt.Errorf("cannot form a key for unsupported value kind %d", v.kind)
	}
	set, err := v.finiteSet()
	if err != nil {
		return err
	}
	return set.setKey(b)
}
