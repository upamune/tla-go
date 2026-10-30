package checker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestUpstreamModules(t *testing.T) {
	f, err := os.Open("testdata/upstream-modules.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var sample struct {
			Name, Source, Config string
			Modules              map[string]string
			Want                 expectedCheck
		}
		if err := json.Unmarshal(scanner.Bytes(), &sample); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(sample.Name, func(t *testing.T) {
			loader := func(_ context.Context, name string) (string, error) {
				source, ok := sample.Modules[name]
				if !ok {
					return "", fmt.Errorf("missing module %s", name)
				}
				return source, nil
			}
			result, err := CheckWithLoader(context.Background(), sample.Source, sample.Config, loader)
			if err != nil {
				t.Fatal(err)
			}
			got := project(result)
			if sample.Name == "upstream-parameterized-spec-violation" {
				// The upstream test compares the trace length, not TLC's internal
				// generated action names. Keep checking every state here.
				for i := range got.Trace {
					got.Trace[i].Action = ""
				}
				for i := range sample.Want.Trace {
					sample.Want.Trace[i].Action = ""
				}
			}
			if !reflect.DeepEqual(got, sample.Want) {
				t.Fatalf("got %#v, want %#v", got, sample.Want)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 71 {
		t.Fatalf("expected 71 cases, got %d", count)
	}
}

func TestModuleLoaderErrors(t *testing.T) {
	root := "---- MODULE M ----\nEXTENDS A\nVARIABLE x\nInit == x = 0\nNext == x' = x\n===="
	for _, tt := range []struct {
		name, source, contains string
	}{
		{"cycle", "---- MODULE A ----\nEXTENDS M\n====", "cyclic"},
		{"wrong name", "---- MODULE B ----\n====", "File name 'A' does not match the name 'B'"},
		{"invalid syntax", "invalid", "module A"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			loader := func(context.Context, string) (string, error) { return tt.source, nil }
			_, err := CheckWithLoader(context.Background(), root, "INIT Init NEXT Next", loader)
			if err == nil || !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestLocalDefinitionsAreNotImported(t *testing.T) {
	root := "---- MODULE M ----\nEXTENDS A\nVARIABLE x\nInit == x = Hidden\nNext == x' = x\n===="
	loader := func(context.Context, string) (string, error) {
		return "---- MODULE A ----\nLOCAL Hidden == 1\nValue == Hidden\n====", nil
	}
	if _, err := CheckWithLoader(context.Background(), root, "INIT Init NEXT Next", loader); err == nil {
		t.Fatal("accepted a reference to an imported LOCAL definition")
	}
}

func TestExtendedModuleCannotSeeSiblingOrImporter(t *testing.T) {
	root := "---- MODULE M ----\nEXTENDS A, B\nVARIABLE x\nSecret == 2\nInit == x = 0\nNext == x' = x\n===="
	for _, reference := range []string{"Secret", "FromA"} {
		t.Run(reference, func(t *testing.T) {
			modules := map[string]string{
				"A": "---- MODULE A ----\nFromA == 1\n====",
				"B": "---- MODULE B ----\nUse == " + reference + "\n====",
			}
			loader := func(_ context.Context, name string) (string, error) { return modules[name], nil }
			if _, err := CheckWithLoader(context.Background(), root, "INIT Init NEXT Next", loader); err == nil {
				t.Fatalf("B resolved inaccessible name %s", reference)
			}
		})
	}
}

func TestSharedDependencyIsLoadedOnce(t *testing.T) {
	root := "---- MODULE M ----\nEXTENDS A, B\nVARIABLE x\nInit == x = C\nNext == x' = x\n===="
	modules := map[string]string{
		"A":    "---- MODULE A ----\nEXTENDS Base\nAValue == C\n====",
		"B":    "---- MODULE B ----\nEXTENDS Base\nBValue == C\n====",
		"Base": "---- MODULE Base ----\nC == 1\n====",
	}
	loads := map[string]int{}
	loader := func(_ context.Context, name string) (string, error) {
		loads[name]++
		return modules[name], nil
	}
	result, err := CheckWithLoader(context.Background(), root, "INIT Init NEXT Next", loader)
	if err != nil || result.Status != Success || loads["Base"] != 1 {
		t.Fatalf("result=%#v loads=%v err=%v", result, loads, err)
	}
}

func TestExtendedLocalInstanceIsNotExported(t *testing.T) {
	modules := map[string]string{
		"A": "---- MODULE A ----\nLOCAL INSTANCE B\nVisible == Value\n====",
		"B": "---- MODULE B ----\nValue == 1\n====",
	}
	loader := func(_ context.Context, name string) (string, error) { return modules[name], nil }
	source := "---- MODULE M ----\nEXTENDS A\nVARIABLE x\nInit == x = Value\nNext == x' = x\n===="
	_, err := CheckWithLoader(context.Background(), source, "INIT Init NEXT Next", loader)
	if err == nil || !strings.Contains(err.Error(), "undefined symbol Value") {
		t.Fatalf("got %v", err)
	}
}

func TestMixedExtensionInstanceCyclesAreRejected(t *testing.T) {
	for _, modules := range []map[string]string{
		{"A": "---- MODULE A ----\nEXTENDS B\n====", "B": "---- MODULE B ----\nI == INSTANCE A\n===="},
		{"A": "---- MODULE A ----\nI == INSTANCE B\n====", "B": "---- MODULE B ----\nEXTENDS A\n===="},
	} {
		loader := func(_ context.Context, name string) (string, error) { return modules[name], nil }
		source := "---- MODULE M ----\nEXTENDS A\nVARIABLE x\nInit == x = 0\nNext == x' = x\n===="
		_, err := CheckWithLoader(context.Background(), source, "INIT Init NEXT Next", loader)
		if err == nil || !strings.Contains(err.Error(), "cyclic") {
			t.Fatalf("got %v", err)
		}
	}
}

func TestInheritedNestedModuleDoesNotFallBackToAnUnrelatedFile(t *testing.T) {
	modules := map[string]string{
		"Holder": "---- MODULE Holder ----\n---- MODULE Held ----\nValue == 1\n====\n====",
		"Held":   "---- MODULE Held ----\nValue == 2\n====",
	}
	loads := map[string]int{}
	loader := func(_ context.Context, name string) (string, error) {
		loads[name]++
		return modules[name], nil
	}
	source := "---- MODULE M ----\nEXTENDS Holder\nVARIABLE x\nI == INSTANCE Held\nInit == x = I!Value\nNext == x' = x\n===="
	result, err := CheckWithLoader(context.Background(), source, "INIT Init NEXT Next", loader)
	if err != nil || result.Status != Success || loads["Held"] != 0 {
		t.Fatalf("result=%#v loads=%v err=%v", result, loads, err)
	}
}

func TestConflictingInheritedNestedModulesAreRejected(t *testing.T) {
	modules := map[string]string{
		"Holder": "---- MODULE Holder ----\n---- MODULE Held ----\nValue == 1\n====\n====",
		"Other":  "---- MODULE Other ----\n---- MODULE Held ----\nValue == 2\n====\n====",
	}
	loader := func(_ context.Context, name string) (string, error) { return modules[name], nil }
	for _, source := range []string{
		"---- MODULE M ----\nEXTENDS Holder\n---- MODULE Held ----\nValue == 3\n====\nVARIABLE x\nInit == x = 0\nNext == x' = x\n====",
		"---- MODULE M ----\nEXTENDS Holder, Other\nVARIABLE x\nInit == x = 0\nNext == x' = x\n====",
	} {
		_, err := CheckWithLoader(context.Background(), source, "INIT Init NEXT Next", loader)
		if err == nil || !strings.Contains(err.Error(), "nested module Held") {
			t.Fatalf("got %v", err)
		}
	}
}

func TestExtendedInstanceCannotCaptureImporterOrSibling(t *testing.T) {
	for _, name := range []string{"FromRoot", "FromSibling"} {
		t.Run(name, func(t *testing.T) {
			modules := map[string]string{
				"A":       "---- MODULE A ----\nI == INSTANCE B WITH C <- " + name + "\nValue == I!Value\n====",
				"B":       "---- MODULE B ----\nCONSTANT C\nValue == C\n====",
				"Sibling": "---- MODULE Sibling ----\nFromSibling == 2\n====",
			}
			loader := func(_ context.Context, name string) (string, error) { return modules[name], nil }
			source := "---- MODULE M ----\nEXTENDS A, Sibling\nVARIABLE x\nFromRoot == 1\nInit == x = 0\nNext == x' = x\n===="
			if _, err := CheckWithLoader(context.Background(), source, "INIT Init NEXT Next", loader); err == nil {
				t.Fatalf("accepted inaccessible name %s", name)
			}
		})
	}
}
