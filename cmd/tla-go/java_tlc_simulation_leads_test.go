package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/upamune/tla-go/internal/javatmp"
)

func TestJavaTLCSimulationLeadsTo(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, next := range []string{
		"x' = 0",
		"IF x=0 THEN x'=1 ELSE x'=0",
		"IF x=0 THEN x'=1 ELSE x'=2",
		"x' \\in {0, 1, 2}",
	} {
		for _, trigger := range []int{0, 1, 2} {
			for _, target := range []int{0, 1, 2} {
				for _, seed := range []string{"1", "2", "3"} {
					t.Run(fmt.Sprintf("%s/%d/%d/%s", next, trigger, target, seed), func(t *testing.T) {
						body := fmt.Sprintf("VARIABLE x\nInit == x=0\nNext == %s\nP == (x=%d) ~> (x=%d)", next, trigger, target)
						compareSimulationGraph(t, jar, body, seed)
					})
				}
			}
		}
	}
}

func TestJavaTLCSimulationLeadsToLazyPredicate(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	body := "VARIABLE x\nInit == x=0\nNext == x'=1\nP == (x=1) ~> (1 \\div x = 1)"
	compareSimulationGraph(t, jar, body, "1")
}

func TestJavaTLCSimulationLeadsToDepthAndConstraints(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, next := range []string{"x' = (x + 1) % 3", "x' \\in {0, 1, 2}", "x < 2 /\\ x' = x + 1"} {
		for _, property := range []string{"(x = 0 \\/ x = 1) ~> (x = 2)", "(x = 2) ~> (x < 2)", "(x < 2) ~> (x = 1)"} {
			for _, depth := range []string{"1", "2", "3", "4", "7"} {
				t.Run(next+"/"+property+"/"+depth, func(t *testing.T) {
					body := fmt.Sprintf("VARIABLE x\nInit == x \\in {0,1}\nNext == %s\nC == x < 2\nP == %s", next, property)
					cfg := "INIT Init NEXT Next CONSTRAINT C CHECK_DEADLOCK FALSE PROPERTY P"
					compareSimulationGraphDepth(t, jar, body, cfg, "2", depth)
				})
			}
		}
	}
}

func TestJavaTLCSimulationLeadsToConstants(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, property := range []string{"TRUE ~> (x=1)", "FALSE ~> (x=1)", "(x=0) ~> TRUE", "(x=0) ~> FALSE", "(x=0) ~> (x=0)", "TRUE ~> TRUE", "TRUE ~> FALSE", "FALSE ~> TRUE", "FALSE ~> FALSE"} {
		t.Run(property, func(t *testing.T) {
			body := "VARIABLE x\nInit == x=0\nNext == IF x=0 THEN x'=1 ELSE x'=0\nP == " + property
			compareSimulationGraph(t, jar, body, "1")
		})
	}
}

func TestJavaTLCSimulationLeadsToQuantified(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, seed := range []string{"1", "2", "3"} {
		t.Run(seed, func(t *testing.T) {
			body := "VARIABLE x\nInit == x=0\nNext == x' \\in {0,1,2}\nP == \\A i \\in {1} : (x=i) ~> (x=2)"
			compareSimulationGraph(t, jar, body, seed)
		})
	}
}

func TestJavaTLCSimulationLeadsToConstantGraph(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, next := range []string{"x'=0", "x' \\in {0,1,2}"} {
		for _, property := range []string{"FALSE ~> (x=1)", "TRUE ~> (x=1)", "(x=0) ~> TRUE"} {
			for _, depth := range []string{"1", "2", "5"} {
				for _, seed := range []string{"1", "2", "3"} {
					t.Run(next+"/"+property+"/"+depth+"/"+seed, func(t *testing.T) {
						body := "VARIABLE x\nInit == x=0\nNext == " + next + "\nP == " + property
						compareSimulationGraphDepth(t, jar, body, "INIT Init NEXT Next PROPERTY P", seed, depth)
					})
				}
			}
		}
	}
}

func TestJavaTLCModernSimulationLeadsToVersionDifference(t *testing.T) {
	jar := os.Getenv("TLC_MODERN_JAR")
	if jar == "" {
		t.Skip("TLC_MODERN_JAR is not set")
	}
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == x'=0\nP == (x=0) ~> (x=1)", "INIT Init NEXT Next PROPERTY P")
	args := []string{"-tool", "-workers", "1", "-fp", "0", "-simulate", "num=1", "-seed", "1", "-depth", "5"}
	cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, append(args, "M.tla")...)...)
	cmd.Dir = dir
	output, _ := javatmp.Apply(cmd).CombinedOutput()
	if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 0 {
		t.Fatalf("Java TLC 1.8.0: %s", output)
	}
	var out, errout bytes.Buffer
	if code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout); code != 13 || errout.Len() != 0 {
		t.Fatalf("tla-go exit %d; stdout %s; stderr %s", code, out.String(), errout.String())
	}
}
