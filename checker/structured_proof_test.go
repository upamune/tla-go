package checker

import (
	"context"
	"testing"
)

func TestStructuredProofScopes(t *testing.T) {
	for _, tc := range []struct {
		name, proof string
		valid       bool
	}{
		{"prior step", "<1>1. TRUE\n<1>2. TRUE BY <1>1\n<1>3. QED", true},
		{"forward step", "<1>1. TRUE BY <1>2\n<1>2. TRUE\n<1>3. QED", false},
		{"self reference", "<1>1. TRUE BY <1>1\n<1>2. QED", true},
		{"step declaration local to subproof", "<1>1. ASSUME NEW K PROVE TRUE\n <2>1. K=K\n <2>2. QED\n<1>2. QED", true},
		{"step declaration absent after", "<1>1. ASSUME NEW K PROVE TRUE\n<1>2. K=K\n<1>3. QED", false},
		{"suffices declaration visible after", "<1>1. SUFFICES ASSUME NEW K PROVE TRUE\n<1>2. K=K\n<1>3. QED", true},
		{"suffices declaration absent in proof", "<1>1. SUFFICES ASSUME NEW K PROVE TRUE\n <2>1. K=K\n <2>2. QED\n<1>2. QED", false},
		{"define local", "<1>1. DEFINE F == TRUE\n<1>2. F BY DEF F\n<1>3. QED", true},
		{"multiple definitions", "<1> DEFINE F == TRUE G == F\n<1>2. G BY DEF F, G\n<1> QED", true},
		{"forward local definition", "<1> DEFINE F == G G == TRUE\n<1> QED", false},
		{"define cannot have proof", "<1>1. DEFINE F == TRUE BY TRUE\n<1>2. QED", false},
		{"define is not a fact", "<1>1. DEFINE F == TRUE\n<1>2. TRUE BY <1>1\n<1>3. QED", false},
		{"define does not escape", "<1>1. TRUE\n <2>1. DEFINE F == TRUE\n <2>2. QED\n<1>2. F\n<1>3. QED", false},
		{"new is not later definition", "<1>1. ASSUME NEW F PROVE F=F\n <2>1. TRUE BY DEF F\n <2>2. QED\n<1>2. QED\nF == TRUE", false},
		{"forward selector", "<1>1. F!lab\n<1>2. QED\nF == lab :: TRUE", false},
		{"shadowed selector", "<1>1. ASSUME NEW F PROVE TRUE\n <2>1. F!lab\n <2>2. QED\n<1>2. QED\nF == lab :: TRUE", false},
		{"proof terminator required", "<1>1. TRUE", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CheckContext(context.Background(), "---- MODULE M ----\nTHEOREM T == TRUE\n"+tc.proof+"\n====\n", "")
			if (err == nil) != tc.valid {
				t.Fatalf("valid %t: %v", tc.valid, err)
			}
		})
	}
}

func TestAnonymousTheoremSubexpression(t *testing.T) {
	for _, statement := range []string{"T!lab", "T!:"} {
		source := "---- MODULE M ----\nTHEOREM T == lab :: TRUE\nTHEOREM " + statement + "\n====\n"
		if _, err := CheckContext(context.Background(), source, ""); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
}
