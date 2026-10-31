package main

import (
	"fmt"
	"os"
	"testing"
)

func TestJavaTLCSimulationMultipleLeadsTo(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, next := range []string{"x'=0", "IF x=0 THEN x'=1 ELSE x'=0", "IF x=0 THEN x'=1 ELSE x'=2", "x' \\in {0,1,2}"} {
		for _, pair := range [][2]string{
			{"(x=0) ~> (x=1)", "(x=1) ~> (x=2)"},
			{"(x=1) ~> (x=2)", "(x=0) ~> (x=1)"},
			{"(x=0) ~> (x=1)", "<>(x=2)"},
			{"<>(x=2)", "(x=0) ~> (x=1)"},
			{"(x=0) ~> (x=1)", "[]<>(x=1)"},
			{"[]<>(x=1)", "(x=0) ~> (x=1)"},
			{"(x=1) ~> (x=0)", "<>[](x=1)"},
			{"<>[](x=1)", "(x=1) ~> (x=0)"},
		} {
			for _, seed := range []string{"1", "2"} {
				t.Run(fmt.Sprintf("%s/%s/%s/%s", next, pair[0], pair[1], seed), func(t *testing.T) {
					body := fmt.Sprintf("VARIABLE x\nInit == x=0\nNext == %s\nP == %s\nQ == %s", next, pair[0], pair[1])
					compareSimulationGraphConfig(t, jar, body, "INIT Init NEXT Next PROPERTY P Q", seed)
				})
			}
		}
	}
}

func TestJavaTLCSimulationMultipleLeadsConstants(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, next := range []string{"x'=0", "IF x=0 THEN x'=1 ELSE x'=2", "x' \\in {0,1,2}"} {
		for _, pair := range [][2]string{
			{"TRUE ~> (x=1)", "[]<>(x=2)"},
			{"[]<>(x=2)", "TRUE ~> (x=1)"},
			{"FALSE ~> (x=1)", "<>(x=2)"},
			{"<>(x=2)", "FALSE ~> (x=1)"},
			{"(x=0) ~> TRUE", "(x=1) ~> (x=2)"},
			{"(x=1) ~> (x=2)", "(x=0) ~> TRUE"},
			{"<>[](x=1)", "TRUE ~> (x=2)"},
			{"TRUE ~> (x=2)", "<>[](x=1)"},
		} {
			for _, seed := range []string{"1", "2"} {
				t.Run(fmt.Sprintf("%s/%s/%s/%s", next, pair[0], pair[1], seed), func(t *testing.T) {
					body := fmt.Sprintf("VARIABLE x\nInit == x=0\nNext == %s\nP == %s\nQ == %s", next, pair[0], pair[1])
					compareSimulationGraphConfig(t, jar, body, "INIT Init NEXT Next PROPERTY P Q", seed)
				})
			}
		}
	}
}

func TestJavaTLCSimulationMultipleLeadsQuantified(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, next := range []string{"x'=0", "IF x=0 THEN x'=1 ELSE x'=2", "x' \\in {0,1,2}"} {
		for _, property := range []string{
			"\\A i \\in {0,1} : (x=i) ~> (x=i+1)",
			"\\A i \\in {0,1,2} : (x=i) ~> (x=0)",
			"\\A i \\in {0,1} : (x=i) ~> (x=i)",
		} {
			for _, seed := range []string{"1", "2", "3"} {
				t.Run(fmt.Sprintf("%s/%s/%s", next, property, seed), func(t *testing.T) {
					body := fmt.Sprintf("VARIABLE x\nInit == x=0\nNext == %s\nP == %s", next, property)
					compareSimulationGraph(t, jar, body, seed)
				})
			}
		}
	}
}

func TestJavaTLCSimulationThreeTemporalProperties(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, next := range []string{"x'=0", "IF x=0 THEN x'=1 ELSE x'=2", "x' \\in {0,1,2}"} {
		for _, properties := range [][3]string{
			{"(x=0) ~> (x=1)", "[]<>(x=2)", "(x=1) ~> (x=2)"},
			{"TRUE ~> (x=1)", "<>(x=2)", "[]<>(x=2)"},
			{"<>[](x=2)", "(x=1) ~> (x=0)", "FALSE ~> (x=2)"},
		} {
			for _, seed := range []string{"1", "2"} {
				t.Run(fmt.Sprintf("%s/%v/%s", next, properties, seed), func(t *testing.T) {
					body := fmt.Sprintf("VARIABLE x\nInit == x=0\nNext == %s\nP == %s\nQ == %s\nR == %s", next, properties[0], properties[1], properties[2])
					compareSimulationGraphConfig(t, jar, body, "INIT Init NEXT Next PROPERTY P Q R", seed)
				})
			}
		}
	}
}

func TestJavaTLCSimulationMixedRecurrenceOrder(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, next := range []string{"IF x=0 THEN x'=1 ELSE x'=2", "x' \\in {0,1,2}"} {
		for target := range 3 {
			for other := range 3 {
				for _, swap := range []bool{false, true} {
					for _, seed := range []string{"1", "2"} {
						t.Run(fmt.Sprintf("%s/%d/%d/%t/%s", next, target, other, swap, seed), func(t *testing.T) {
							p := fmt.Sprintf("TRUE ~> (x=%d)", target)
							q := fmt.Sprintf("[]<>(x=%d)", other)
							if swap {
								p, q = q, p
							}
							body := fmt.Sprintf("VARIABLE x\nInit == x=0\nNext == %s\nP == %s\nQ == %s", next, p, q)
							compareSimulationGraphConfig(t, jar, body, "INIT Init NEXT Next PROPERTY P Q", seed)
						})
					}
				}
			}
		}
	}
}

func TestJavaTLCSimulationMixedMultipleTraces(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	body := "VARIABLE x\nInit == x=0\nNext == x' \\in {0,1,2}\nP == (x=0) ~> (x=0)\nQ == TRUE ~> (x \\in {0,1,2})"
	compareSimulationGraphParameters(t, jar, body, "INIT Init NEXT Next PROPERTY P Q", "1", "5", "3")
}
