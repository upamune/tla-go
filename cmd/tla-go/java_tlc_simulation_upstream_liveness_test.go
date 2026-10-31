package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type upstreamSimulationLiveness struct {
	name, dir, spec, seed string
	args                  []string
	code                  int
}

type recordedSimulation struct {
	Code     int      `json:"code"`
	Messages []string `json:"messages"`
}

// The models of TLC's liveness/simulation JUnit tests and NQSpecTest, run with
// fixed seeds. SuccessfulSimulationTestCase stops Test2 after a while and
// NQSpecTest sets 100 traces; num bounds both here. expected.json records Java
// TLC 1.7.4; set UPDATE_SIMULATION_LIVENESS=1 with TLC_JAR to record it again.
var upstreamSimulationLivenessCases = []upstreamSimulationLiveness{
	{"Example1Test", "simulation", "Example1", "1", []string{"-depth", "11"}, 13},
	{"Example2Test", "simulation", "Example2", "1", []string{"-depth", "11"}, 13},
	// LiveCheck1 fails on the empty trace of a zero-depth simulation.
	{"Example1ZeroDepth", "simulation", "Example1", "1", []string{"num=2", "-depth", "0"}, 255},
	{"SimulationTest2", "test-model", "Test2", "1", []string{"num=200", "-depth", "6"}, 0},
	{"SimulationTest2a", "test-model", "Test2a", "1", []string{"-depth", "6"}, 13},
	{"LiveCheckSimulationTest2", "test-model", "Test2", "2", []string{"num=200", "-depth", "10"}, 0},
	{"LiveCheckSimulationTest2a", "test-model", "Test2a", "2", []string{"-depth", "10"}, 13},
	{"StutteringTest", "CodePlexBug08", "MC", "1", nil, 13},
	{"StutteringTestSeed2", "CodePlexBug08", "MC", "2", nil, 13},
	{"StutteringTestSeed3", "CodePlexBug08", "MC", "3", nil, 13},
	{"NQSpecTest", "NQSpec", "MC", "1", []string{"num=10"}, 0},
	// Models of model-checking tests, run in simulation: symmetry merges
	// states, and TLC fails to recover a state from its fingerprint when the
	// symmetry is unsound or initial states depend on TLCGet("level").
	{"UnsymmetricMCA", "symmetry", "UnsymmetricMCA", "1", []string{"num=10", "-depth", "30"}, 13},
	{"UnsymmetricMCB", "symmetry", "UnsymmetricMCB", "1", []string{"num=10", "-depth", "30"}, 255},
	{"TLCGetLevel", "test-model", "TLCGetLevel", "1", []string{"num=10", "-depth", "30"}, 255},
}

func TestJavaTLCSimulationUpstreamLiveness(t *testing.T) {
	root := filepath.Join("testdata", "java-tlc", "simulation-liveness")
	expectedPath := filepath.Join(root, "expected.json")
	expected := map[string]recordedSimulation{}
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	jar := os.Getenv("TLC_JAR")
	update := jar != "" && os.Getenv("UPDATE_SIMULATION_LIVENESS") != ""
	for _, tc := range upstreamSimulationLivenessCases {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"-deadlock", "-workers", "1", "-fp", "0", "-seed", tc.seed, "-simulate"}, tc.args...)
			got := runUpstreamSimulation(t, filepath.Join(root, tc.dir), tc.spec, args)
			if jar != "" {
				java := runJavaUpstreamSimulation(t, jar, filepath.Join(root, tc.dir), tc.spec, args)
				if update {
					expected[tc.name] = java
				}
				gotMessages, javaMessages := comparableProgress(got.Messages, java.Messages)
				compareRecordedSimulation(t, "Java TLC", recordedSimulation{got.Code, gotMessages}, recordedSimulation{java.Code, javaMessages})
			}
			compareRecordedSimulation(t, "recorded Java TLC", got, expected[tc.name])
			if got.Code != tc.code {
				t.Fatalf("exit %d; want %d", got.Code, tc.code)
			}
		})
	}
	if update {
		data, err := json.MarshalIndent(expected, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(expectedPath, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func compareRecordedSimulation(t *testing.T, source string, got, want recordedSimulation) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tla-go exit %d, messages %q\n%s exit %d, messages %q", got.Code, got.Messages, source, want.Code, want.Messages)
	}
}

// Java TLC's progress thread races with the end of a fast run, so it can omit
// a final progress report. Compare the other messages and, when both runs
// report progress, the last count of generated states.
func comparableProgress(got, want []string) ([]string, []string) {
	got, gotCount := splitProgress(got)
	want, wantCount := splitProgress(want)
	if gotCount != "" && wantCount != "" {
		got, want = append(got, gotCount), append(want, wantCount)
	}
	return got, want
}

func splitProgress(messages []string) ([]string, string) {
	var rest []string
	last := ""
	for _, message := range messages {
		if strings.HasPrefix(message, "2209:") {
			last = message
		} else {
			rest = append(rest, message)
		}
	}
	return rest, last
}

func runUpstreamSimulation(t *testing.T, dir, spec string, args []string) recordedSimulation {
	t.Helper()
	var out, errout bytes.Buffer
	code := run(context.Background(), append(append(args, "-tool"), filepath.Join(dir, spec+".tla")), &out, &errout)
	if errout.Len() != 0 {
		t.Fatalf("stderr: %s", errout.String())
	}
	return recordedSimulation{Code: code, Messages: livenessMessages(readToolMessages(out.String()))}
}

func runJavaUpstreamSimulation(t *testing.T, jar, source, spec string, args []string) recordedSimulation {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	javaArgs := append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx512m", "-XX:+UseParallelGC", "-cp", jar, "tlc2.TLC"}, args...)
	cmd := exec.Command("java", append(javaArgs, "-tool", spec)...)
	cmd.Dir = dir
	output, _ := cmd.CombinedOutput()
	if cmd.ProcessState == nil {
		t.Fatalf("Java did not start: %s", output)
	}
	return recordedSimulation{Code: cmd.ProcessState.ExitCode(), Messages: livenessMessages(readToolMessages(string(output)))}
}
