package checker

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/upamune/tla-go/config"
)

func BenchmarkJavaTLCSubsetEq(b *testing.B) {
	benchmarkJavaTLCModel(b, "SubsetEq", "SubsetEq", Success)
}

func BenchmarkJavaTLCAssignmentInitExpensive(b *testing.B) {
	benchmarkJavaTLCModel(b, "AssignmentInitExpensive", "AssignmentInitExpensive", Success)
}

func BenchmarkJavaTLCBidirectionalFairness(b *testing.B) {
	benchmarkJavaTLCModel(b, "BidirectionalTransitions", "BidirectionalTransitions1Bx", LivenessViolation)
}

func benchmarkJavaTLCModel(b *testing.B, name, cfgName string, want Status) {
	b.Helper()
	source, err := os.ReadFile("testdata/java-tlc/" + name + ".tla")
	if err != nil {
		b.Fatal(err)
	}
	text, err := os.ReadFile("testdata/java-tlc/" + cfgName + ".cfg")
	if err != nil {
		b.Fatal(err)
	}
	cfg, err := config.Parse(string(text))
	if err != nil {
		b.Fatal(err)
	}
	cfg.CheckDeadlock = false
	for b.Loop() {
		result, err := CheckConfig(context.Background(), string(source), cfg)
		if err != nil || result.Status != want {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}

func BenchmarkJavaTLCOftenCounter(b *testing.B) {
	const source = "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 1000\nP == []<>(x = 999)\n====\n"
	const cfg = "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE\n"
	for b.Loop() {
		result, err := Check(source, cfg)
		if err != nil || result.Status != LivenessViolation || result.DistinctStates != 1000 {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}

func BenchmarkJavaTLCOneBitMutex(b *testing.B) {
	source, err := os.ReadFile("testdata/java-tlc/one-bit/OneBitMutexNoSymmetryMC.tla")
	if err != nil {
		b.Fatal(err)
	}
	dependency, err := os.ReadFile("testdata/java-tlc/one-bit/OneBitMutex.tla")
	if err != nil {
		b.Fatal(err)
	}
	text, err := os.ReadFile("testdata/java-tlc/one-bit/OneBitMutexNoSymmetryMC.cfg")
	if err != nil {
		b.Fatal(err)
	}
	cfg, err := config.Parse(string(text))
	if err != nil {
		b.Fatal(err)
	}
	cfg.CheckDeadlock = false
	loader := func(_ context.Context, name string) (string, error) {
		if name != "OneBitMutex" {
			return "", fmt.Errorf("unexpected module %s", name)
		}
		return string(dependency), nil
	}
	for b.Loop() {
		result, err := CheckConfigWithLoader(context.Background(), string(source), cfg, loader)
		if err != nil || result.Status != LivenessViolation || result.DistinctStates != 127 {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}

func BenchmarkJavaTLCRandomSubsetNext(b *testing.B) {
	source, err := os.ReadFile("testdata/java-tlc/random-subset/RandomSubsetNext.tla")
	if err != nil {
		b.Fatal(err)
	}
	cfg, err := os.ReadFile("testdata/java-tlc/random-subset/RandomSubsetNext.cfg")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		result, err := CheckWithOptions(context.Background(), string(source), string(cfg), Options{Seed: 15041980})
		if err != nil || result.Status != InvariantViolation || result.GeneratedStates != 67321 || result.DistinctStates != 7732 {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}
