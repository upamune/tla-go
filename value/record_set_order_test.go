package value

import (
	"reflect"
	"testing"
)

// This documents the current Go enumeration independently of Java's intern table.
func TestRecordSetEnumerationCanonicalIdentity(t *testing.T) {
	one, _ := Integer(1)
	two, _ := Integer(2)
	choices, _ := Set([]Value{one, two})
	singleton, _ := Set([]Value{one})
	set, err := RecordSet([]Pair{{String("adr"), choices}, {String("op"), singleton}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	err = set.Each(func(v Value) (bool, error) {
		got = append(got, v.String())
		literal, err := Record([]Pair{{String("op"), one}, {String("adr"), v.data.pairs[0].Value}})
		if err != nil {
			return false, err
		}
		equal, err := Equal(v, literal)
		if err != nil || !equal {
			t.Errorf("record equality: %v, %v", equal, err)
		}
		a, err := TLCFingerprint([]Value{v})
		if err != nil {
			return false, err
		}
		b, err := TLCFingerprint([]Value{literal})
		if err != nil {
			return false, err
		}
		if a != b {
			t.Errorf("fingerprint differs: %x != %x", a, b)
		}
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"[adr |-> 1, op |-> 1]", "[adr |-> 2, op |-> 1]"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
