package value

import (
	"fmt"
	"strconv"
	"unsafe"
)

// Key encodes a value canonically for exact state identity. It is not TLC's
// 64-bit fingerprint; callers can use Go's map without fingerprint collisions.
func (v Value) Key() (string, error) {
	var b keyWriter
	if err := v.appendKey(&b); err != nil {
		return "", err
	}
	// Like strings.Builder, the key takes the buffer, which nothing reuses.
	return unsafe.String(unsafe.SliceData(b.buf), len(b.buf)), nil
}

// AppendKey appends the encoding returned by Key to dst.
func (v Value) AppendKey(dst []byte) ([]byte, error) {
	b := keyWriter{buf: dst}
	err := v.appendKey(&b)
	return b.buf, err
}

// keyWriter accumulates a key in a caller-supplied buffer.
type keyWriter struct{ buf []byte }

func (b *keyWriter) appendByte(c byte)   { b.buf = append(b.buf, c) }
func (b *keyWriter) appendText(s string) { b.buf = append(b.buf, s...) }

func (v Value) appendKey(b *keyWriter) error {
	switch v.kind {
	case IntegerKind:
		b.appendByte('i')
		writeInt(b, int64(v.number))
	case BooleanKind:
		b.appendByte('b')
		writeInt(b, int64(v.number))
	case StringKind:
		b.appendByte('s')
		writeString(b, v.data.text)
	case ModelKind:
		b.appendByte('m')
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

func writeInt(b *keyWriter, n int64)     { b.buf = append(strconv.AppendInt(b.buf, n, 10), ':') }
func writeString(b *keyWriter, s string) { writeInt(b, int64(len(s))); b.appendText(s) }
func (v Value) setKey(b *keyWriter) error {
	low, high, ok := v.contiguous()
	if ok {
		b.appendByte('r')
		writeInt(b, int64(low))
		writeInt(b, int64(high))
		return nil
	}
	b.appendByte('S')
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

func (v Value) functionKey(b *keyWriter) error {
	b.appendByte('f')
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

func (v Value) appendSpecialKey(b *keyWriter) error {
	if v.kind == SubsetKind {
		return v.appendSubsetKey(b)
	}
	if v.kind == LazyFunctionKind {
		f, err := v.materializedFunction()
		if err != nil {
			return err
		}
		return f.functionKey(b)
	}
	if v.kind == InfiniteSetKind {
		b.appendByte('I')
		writeInt(b, int64(v.number))
		return nil
	}
	if v.kind == SequenceSetKind {
		b.appendByte('q')
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
