package upstreamjunit

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestSuiteFixtures(t *testing.T) {
	root := filepath.Join("testdata", "upstream")
	classes, err := Inventory(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		values    []string
		args      []string
		deadlock  bool
		inherited bool
		reason    string
	}{
		{"Test10", []string{"2", "1", "0"}, nil, false, true, "coverage setup"},
		{"Test22", []string{"1344", "64", "0"}, nil, false, true, "coverage setup"},
		{"Test17", []string{"7", "2", "0"}, nil, false, true, "coverage expectation"},
		{"Test57", nil, nil, false, false, "constructor side effects"},
		{"ETest1", nil, nil, false, false, "output interception"},
		{"ETest3", nil, nil, true, false, "output interception"},
		{"ETest8", nil, []string{"-simulate"}, false, false, "output interception"},
		{"Test220", nil, nil, true, false, "output interception"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			idx := slices.IndexFunc(classes, func(c Class) bool { return c.Name == "tlc2.tool.suite."+tc.name })
			if idx < 0 {
				t.Fatal("missing fixture")
			}
			r := Resolve(root, classes, classes[idx])
			if r.Case.Model != strings.ToLower(tc.name) || r.Path != "suite" || r.Deadlock != tc.deadlock || !reflect.DeepEqual(r.Case.Args, tc.args) || (len(r.Inherited) > 0) != tc.inherited || !strings.Contains(strings.Join(r.Reasons, ";"), tc.reason) {
				t.Fatalf("resolution: %+v", r)
			}
			if tc.inherited && (!reflect.DeepEqual(r.Inherited[1].Values, tc.values) || r.Inherited[3].Present) {
				t.Fatalf("inherited: %+v", r.Inherited)
			}
		})
	}
}

func TestSuiteCommentsAndHooks(t *testing.T) {
	dir := t.TempDir()
	src := `package example;
class TestComment extends SuiteTestCase {
 // public TestComment() { super(dynamic()); }
 /* void testSpec() {} */
 protected boolean checkDeadLock() { return true; }
 void beforeSetUp() {}
}`
	if err := os.WriteFile(filepath.Join(dir, "TestComment.java"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	classes, err := Inventory(dir)
	if err != nil {
		t.Fatal(err)
	}
	r := Resolve(dir, classes, classes[0])
	if len(r.Inherited) != 4 || !r.Deadlock || !strings.Contains(strings.Join(r.Reasons, ";"), "custom harness hook: beforeSetUp") {
		t.Fatalf("resolution: %+v", r)
	}
}
