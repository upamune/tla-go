package value

import "testing"

func TestSequenceSetMembershipFinitenessAndIdentity(t *testing.T) {
	empty, _ := Set(nil)
	one, _ := Integer(1)
	set, _ := Set([]Value{one})
	seqs := Sequences(set)
	tuple, _ := Tuple([]Value{one, one})
	found, err := seqs.Contains(tuple)
	if err != nil || !found {
		t.Fatalf("membership: %v, %v", found, err)
	}
	finite, err := seqs.IsFinite()
	if err != nil || finite {
		t.Fatalf("nonempty base finiteness: %v, %v", finite, err)
	}
	finite, err = Sequences(empty).IsFinite()
	if err != nil || !finite {
		t.Fatalf("empty base finiteness: %v, %v", finite, err)
	}
	eq, err := Equal(seqs, Sequences(Interval(1, 1)))
	if err != nil || !eq || key(t, seqs) != key(t, Sequences(Interval(1, 1))) {
		t.Fatalf("identity: %v, %v", eq, err)
	}
	if _, err := Equal(Sequences(empty), empty); err == nil {
		t.Fatal("overridden sequence set compared with ordinary set")
	}
	if _, err := Sequences(empty).Cardinality(); err == nil {
		t.Fatal("cardinality of overridden sequence set should fail")
	}
}
