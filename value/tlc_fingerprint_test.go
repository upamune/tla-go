package value

import "testing"

func TestTLCFingerprintInternOrder(t *testing.T) {
	ranks := map[string]int{"zz": 0, "aa": 1, "p1": 2, "p2": 3}
	rank := func(text string) (int, bool) { n, ok := ranks[text]; return n, ok }
	compare := func(a, b Value) (int, error) {
		x, xok := a.Text()
		y, yok := b.Text()
		if xok && yok && a.Kind() == b.Kind() {
			return ranks[x] - ranks[y], nil
		}
		return Compare(a, b)
	}
	set, err := Set([]Value{String("aa"), String("zz")})
	if err != nil {
		t.Fatal(err)
	}
	fn, err := Function([]Pair{{String("aa"), Model("p1")}, {String("zz"), Model("p2")}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		state []Value
		want  uint64
	}{
		{[]Value{Model("p1"), Model("p2")}, 0xbfd7e56ead45774b},
		{[]Value{set, Model("p2")}, 0xf1ffd27f6389d263},
		{[]Value{fn}, 0x2906c39960a1f769},
	} {
		got, err := TLCFingerprintWithOrdering(tc.state, rank, compare)
		if err != nil || got != tc.want {
			t.Errorf("got %x, %v; want %x", got, err, tc.want)
		}
	}
}

func TestTLCFingerprintMatchesReference(t *testing.T) {
	one, _ := Integer(1)
	tuple, err := Tuple([]Value{one, String("m1"), Bool(true)})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		state []Value
		want  uint64
	}{
		{[]Value{one}, 0xf67893cfa654c4b3},
		{[]Value{String("m1")}, 0x93a0e87a38ca264e},
		{[]Value{tuple}, 0x87058dc4b1210ead},
		{[]Value{String("é")}, 0x091aa76ea745a412},
		{[]Value{String("ǩ")}, 0x091aa76ea745a412},
	} {
		got, err := TLCFingerprint(tc.state)
		if err != nil || got != tc.want {
			t.Fatalf("got %016x, %v; want %016x", got, err, tc.want)
		}
	}
	if _, err := TLCFingerprint([]Value{Model("M")}); err == nil {
		t.Fatal("model value fingerprint should report unsupported intern order")
	}
}

// Java 1.7.4 FP64.Init(0), RecordValue.fingerPrint and
// FcnRcdValue.fingerPrint, with "op" interned before "adr".
func TestTLCFingerprintJavaRecordAndFunction(t *testing.T) {
	one, _ := Integer(1)
	pairs := []Pair{{String("adr"), one}, {String("op"), String("Rd")}}
	record, err := Record(pairs)
	if err != nil {
		t.Fatal(err)
	}
	function, err := Function(pairs)
	if err != nil {
		t.Fatal(err)
	}
	ranks := map[string]int{"op": 0, "adr": 1, "Rd": 2}
	compare := func(a, b Value) (int, error) {
		x, xok := a.Text()
		y, yok := b.Text()
		if xok && yok && a.Kind() == b.Kind() {
			return ranks[x] - ranks[y], nil
		}
		return Compare(a, b)
	}
	for _, v := range []Value{record, function} {
		got, err := TLCFingerprintWithOrdering([]Value{v}, nil, compare)
		if err != nil || got != 0x56131a998eb0afd4 {
			t.Fatalf("%v: %x, %v", v, got, err)
		}
	}
}
