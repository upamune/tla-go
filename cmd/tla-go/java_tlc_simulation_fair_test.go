package main

import (
	"fmt"
	"os"
	"testing"
)

func TestJavaTLCSimulationFairness(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, next := range []string{"x'=0", "IF x=0 THEN x'=1 ELSE x'=0", "IF x=0 THEN x'=1 ELSE x'=2", "x' \\in {0,1,2}"} {
		for _, fair := range []string{"WF_x(Next)", "SF_x(Next)"} {
			for _, property := range []string{"<>(x=1)", "<>(x=2)", "[]<>(x=1)", "<>[](x=1)"} {
				for _, seed := range []string{"1", "2"} {
					t.Run(fmt.Sprintf("%s/%s/%s/%s", next, fair, property, seed), func(t *testing.T) {
						body := fmt.Sprintf("VARIABLE x\nInit == x=0\nNext == %s\nSpec == Init /\\ [][Next]_x /\\ %s\nP == %s", next, fair, property)
						compareSimulationGraphConfig(t, jar, body, "SPECIFICATION Spec PROPERTY P", seed)
					})
				}
			}
		}
	}
}

func TestJavaTLCSimulationFairnessActions(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, actions := range []string{
		"A == x=0 /\\ x'=1\nB == x=1 /\\ x'=0\nNext == A \\/ B",
		"A == x=0 /\\ x'=1\nB == x=0 /\\ x'=2\nC == x \\in {1,2} /\\ UNCHANGED x\nNext == A \\/ B \\/ C",
		"A == x=1 /\\ x'=2\nNext == x' \\in {0,1,2}",
	} {
		for _, fair := range []string{"WF_x(A)", "SF_x(A)"} {
			for _, property := range []string{"<>(x=1)", "<>(x=2)", "[]<>(x=1)", "<>[](x=1)"} {
				for _, seed := range []string{"1", "2", "3"} {
					t.Run(fmt.Sprintf("%s/%s/%s/%s", actions, fair, property, seed), func(t *testing.T) {
						body := fmt.Sprintf("VARIABLE x\nInit == x=0\n%s\nSpec == Init /\\ [][Next]_x /\\ %s\nP == %s", actions, fair, property)
						compareSimulationGraphConfig(t, jar, body, "SPECIFICATION Spec PROPERTY P", seed)
					})
				}
			}
		}
	}
}
