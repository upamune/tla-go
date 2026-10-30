package checker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

func TestUpstreamSubexpressionFixtures(t *testing.T) {
	dir := "testdata/subexpressions"
	source, err := os.ReadFile(filepath.Join(dir, "Subexpressions.tla"))
	if err != nil {
		t.Fatal(err)
	}
	loader := func(_ context.Context, name string) (string, error) {
		data, err := os.ReadFile(filepath.Join(dir, name+".tla"))
		return string(data), err
	}
	for _, tc := range []struct {
		config string
		status Status
		change bool
	}{
		{"Subexpressions.cfg", Success, false},
		{"Violation.cfg", InvariantViolation, false},
		{"Subexpressions.cfg", AssumptionViolation, true},
	} {
		t.Run(tc.config+"/"+string(tc.status), func(t *testing.T) {
			config, err := os.ReadFile(filepath.Join(dir, tc.config))
			if err != nil {
				t.Fatal(err)
			}
			model := string(source)
			if tc.change {
				model = strings.Replace(model, "ASSUME A!Double(4)!1!<< = 4", "ASSUME A!Double(4)!1!<< = 8", 1)
			}
			got, err := CheckWithLoader(context.Background(), model, string(config), loader)
			if err != nil || got.Status != tc.status {
				t.Fatalf("got %+v, %v; want %s", got, err, tc.status)
			}
			if tc.status == Success && (got.GeneratedStates != 5 || got.DistinctStates != 4 || got.Depth != 4) {
				t.Fatalf("unexpected counts: %+v", got)
			}
			if tc.status == InvariantViolation && (got.Violation == nil || got.Violation.Name != "Broken" || len(got.Violation.Trace) != 4) {
				t.Fatalf("unexpected violation: %+v", got.Violation)
			}
		})
	}
}

func TestUpstreamSubexpressionMatrix(t *testing.T) {
	file, err := os.Open("testdata/upstream-subexpression-matrix.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		var tc struct{ Name, Source, Config, Status, Error string }
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(tc.Name, func(t *testing.T) {
			got, err := Check(tc.Source, tc.Config)
			if tc.Error != "" {
				message, _, _ := strings.Cut(tc.Error, " at line")
				if err == nil || !strings.Contains(err.Error(), message) {
					t.Fatalf("got %+v, %v; want error %q", got, err, tc.Error)
				}
				return
			}
			if err != nil || string(got.Status) != tc.Status {
				t.Fatalf("got %+v, %v; want %s", got, err, tc.Status)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 34 {
		t.Fatalf("got %d cases; want 34", count)
	}
}

func TestSubexpressionsInImportedModules(t *testing.T) {
	module := "---- MODULE I ----\nEXTENDS Naturals\nF(n) == n+2\nG(n) == F(n)!1\n===="
	loader := func(_ context.Context, name string) (string, error) {
		if name == "I" {
			return module, nil
		}
		return "", os.ErrNotExist
	}
	for _, tc := range []struct{ header, assumption string }{
		{"EXTENDS Naturals\nA == INSTANCE I", "A!G(3)=3"},
		{"EXTENDS Naturals, I", "G(3)=3"},
	} {
		source := "---- MODULE M ----\n" + tc.header + "\nVARIABLE x\nASSUME " + tc.assumption + "\nInit == x=0\nNext == UNCHANGED x\n===="
		got, err := CheckWithLoader(context.Background(), source, "INIT Init NEXT Next CHECK_DEADLOCK FALSE", loader)
		if err != nil || got.Status != Success {
			t.Fatalf("%s: got %+v, %v", tc.header, got, err)
		}
	}
}

func TestSubexpressionsDoNotMutateParsedModule(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nD(a)==a+1\nASSUME D(2)!1=2\nInit==x=0\nNext==UNCHANGED x\n===="
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse("INIT Init NEXT Next CHECK_DEADLOCK FALSE")
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(module)
	if err != nil {
		t.Fatal(err)
	}
	got, err := CheckModuleWithOptions(context.Background(), module, cfg, Options{})
	if err != nil || got.Status != Success {
		t.Fatalf("got %+v, %v", got, err)
	}
	after, err := json.Marshal(module)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("checking changed the caller's parsed module")
	}
}
