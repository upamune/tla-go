package value

import (
	"fmt"
	"slices"
	"sync"
	"unicode/utf16"
)

// TLCFingerprintPolynomialCount is the number of FP64 polynomials supported by TLC.
const TLCFingerprintPolynomialCount = len(tlcPolynomials)

type fingerprintTable struct {
	once  sync.Once
	bytes [256]uint64
}

var fingerprintTables [TLCFingerprintPolynomialCount]fingerprintTable

// TLCFingerprintAlgorithm holds one immutable FP64 algorithm. Separate checks
// may use different polynomials concurrently.
type TLCFingerprintAlgorithm struct {
	polynomial uint64
	table      *[256]uint64
}

// NewTLCFingerprintAlgorithm selects a polynomial by its Java TLC index.
func NewTLCFingerprintAlgorithm(index int) (*TLCFingerprintAlgorithm, error) {
	if index < 0 || index >= len(tlcPolynomials) {
		return nil, fmt.Errorf("TLC fingerprint polynomial index %d is out of range", index)
	}
	entry := &fingerprintTables[index]
	entry.once.Do(func() { entry.bytes = makeTLCByteTable(tlcPolynomials[index]) })
	return &TLCFingerprintAlgorithm{polynomial: tlcPolynomials[index], table: &entry.bytes}, nil
}

// Polynomial returns the signed long reported by Java TLC for this algorithm.
func (a *TLCFingerprintAlgorithm) Polynomial() int64 { return int64(a.polynomial) }

func makeTLCByteTable(polynomial uint64) [256]uint64 {
	var powers [72]uint64
	power := uint64(1) << 63
	for i := range powers {
		powers[i] = power
		low := power & 1
		power >>= 1
		if low != 0 {
			power ^= polynomial
		}
	}
	var table [256]uint64
	for value := range table {
		for bit := range 8 {
			if value&(1<<bit) != 0 {
				table[value] ^= powers[71-bit]
			}
		}
	}
	return table
}

type tlcFingerprinter struct {
	fingerprint uint64
	table       *[256]uint64
	rank        func(string) (int, bool)
	compare     func(Value, Value) (int, error)
}

func (f *tlcFingerprinter) byte(b byte) {
	f.fingerprint = f.fingerprint>>8 ^ f.table[byte(f.fingerprint)^b]
}

func (f *tlcFingerprinter) integer(n uint32) {
	for shift := 0; shift < 32; shift += 8 {
		f.byte(byte(n >> shift))
	}
}

// TLCFingerprint implements TLC FP64 for states without model values. Model
// value fingerprints depend on SANY's global string-intern order and are not
// yet supported by this function.
func TLCFingerprint(state []Value) (uint64, error) {
	return TLCFingerprintWithOrdering(state, nil, nil)
}

// TLCFingerprintWithOrdering uses the check's intern ranks and value ordering.
func TLCFingerprintWithOrdering(state []Value, rank func(string) (int, bool), compare func(Value, Value) (int, error)) (uint64, error) {
	algorithm, _ := NewTLCFingerprintAlgorithm(0)
	return algorithm.Fingerprint(state, rank, compare)
}

// Fingerprint computes the Java TLC FP64 fingerprint using the selected polynomial.
func (a *TLCFingerprintAlgorithm) Fingerprint(state []Value, rank func(string) (int, bool), compare func(Value, Value) (int, error)) (uint64, error) {
	f := tlcFingerprinter{fingerprint: a.polynomial, table: a.table, rank: rank, compare: compare}
	for _, v := range state {
		if err := f.value(v); err != nil {
			return 0, err
		}
	}
	return f.fingerprint, nil
}

func (f *tlcFingerprinter) value(v Value) error {
	switch v.kind {
	case BooleanKind:
		f.byte(0)
		if v.number == 0 {
			f.byte('f')
		} else {
			f.byte('t')
		}
	case IntegerKind:
		f.byte(1)
		f.integer(uint32(v.number))
	case StringKind:
		f.text(v.data.text)
	case FunctionKind, LazyFunctionKind:
		return f.function(v)
	case ModelKind:
		return f.model(v)
	default:
		return f.set(v)
	}
	return nil
}

func (f *tlcFingerprinter) model(v Value) error {
	if f.rank != nil {
		if rank, ok := f.rank(v.data.text); ok {
			f.byte(21)
			f.integer(uint32(rank + 1))
			return nil
		}
	}
	return fmt.Errorf("TLC fingerprint requires the intern rank of model value %s", v.data.text)
}

func (f *tlcFingerprinter) text(text string) {
	units := utf16.Encode([]rune(text))
	f.byte(3)
	f.integer(uint32(len(units)))
	for _, unit := range units {
		f.byte(byte(unit))
	}
}

func (f *tlcFingerprinter) function(v Value) error {
	pairs, err := v.Pairs()
	if err != nil {
		return err
	}
	if err := f.orderPairs(pairs); err != nil {
		return err
	}
	f.byte(9)
	f.integer(uint32(len(pairs)))
	for _, pair := range pairs {
		if err := f.value(pair.Key); err != nil {
			return err
		}
		if err := f.value(pair.Value); err != nil {
			return err
		}
	}
	return nil
}

func (f *tlcFingerprinter) set(v Value) error {
	items, err := v.Elements()
	if err != nil {
		return err
	}
	if err := f.orderValues(items); err != nil {
		return err
	}
	f.byte(5)
	f.integer(uint32(len(items)))
	for _, item := range items {
		if err := f.value(item); err != nil {
			return err
		}
	}
	return nil
}

func (f *tlcFingerprinter) orderValues(items []Value) error {
	if f.compare == nil {
		return nil
	}
	var failure error
	slices.SortStableFunc(items, func(a, b Value) int {
		if failure != nil {
			return 0
		}
		var cmp int
		cmp, failure = f.compare(a, b)
		return cmp
	})
	return failure
}

func (f *tlcFingerprinter) orderPairs(pairs []Pair) error {
	if f.compare == nil {
		return nil
	}
	var failure error
	slices.SortStableFunc(pairs, func(a, b Pair) int {
		if failure != nil {
			return 0
		}
		var cmp int
		cmp, failure = f.compare(a.Key, b.Key)
		return cmp
	})
	return failure
}
