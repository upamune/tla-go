package upstreamjunit

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestResolveUpstreamClasses(t *testing.T) {
	root := filepath.Join("testdata", "upstream")
	classes, err := Inventory(root)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, model, path, workers string
		deadlock, dump, pcal       bool
		args                       []string
	}{
		{"tlc2.tool.MinimumDiameterTest", "MinimumDiameter", "", "1", false, false, false, nil},
		{"tlc2.tool.DiameterTest", "DieHardTLA", "", "4", false, true, false, nil},
		{"tlc2.tool.BugzillaBug279Test", "InitStateBug", "Bug279", "1", true, false, false, nil},
		{"tlc2.tool.DepthFirstDieHardTest", "DieHard", "", "1", false, true, false, []string{"-dfid", "7"}},
		{"tlc2.tool.coverage.CCoverageTest", "C", "coverage", "1", false, true, false, []string{"-coverage", "9999"}},
		{"pcal.NoLoop2Test", "NoLoop2", "pcal", "1", false, true, true, []string{"-wf", "-termination"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			i := slices.IndexFunc(classes, func(c Class) bool { return c.Name == tc.name })
			if i < 0 {
				t.Fatal("class missing")
			}
			r := Resolve(root, classes, classes[i])
			if r.Case.Model != tc.model || r.Path != tc.path || r.Case.Workers != tc.workers || r.Deadlock != tc.deadlock || r.Dump != tc.dump || r.Translate != tc.pcal || !reflect.DeepEqual(append(r.Case.Args, r.Case.TranslationArgs...), tc.args) {
				t.Fatalf("resolution: %+v", r)
			}
			if len(r.Reasons) == 0 {
				t.Fatal("missing unresolved reasons")
			}
		})
	}
}

func TestUpstreamAssertedDepth(t *testing.T) {
	src, err := os.ReadFile("testdata/upstream/tlc2/tool/MinimumDiameterTest.java")
	if err != nil {
		t.Fatal(err)
	}
	ec, err := os.ReadFile("testdata/EC.java")
	if err != nil {
		t.Fatal(err)
	}
	bodies, presence, omissions := AssertedIDs(string(src), string(ec))
	if !slices.Contains(bodies, "2194") || !slices.Contains(bodies, "2199") || !slices.Contains(presence, "2186") || !slices.Contains(omissions, "coverage assertions") {
		t.Fatalf("%v %v %v", bodies, presence, omissions)
	}
}

func TestConstructorDelegation(t *testing.T) {
	dir := t.TempDir()
	src := `package example;
class Child extends ModelCheckerTestCase {
 public Child() { this("Example"); }
 public Child(String model) { super(model, "pcal", new String[]{"-wf"}); }
 protected boolean doDump() { return false; }
}`
	if err := os.WriteFile(filepath.Join(dir, "Child.java"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	classes, err := Inventory(dir)
	if err != nil {
		t.Fatal(err)
	}
	r := Resolve(dir, classes, classes[0])
	if r.Case.Model != "Example" || r.Path != "pcal" || r.Dump || !reflect.DeepEqual(r.Case.Args, []string{"-wf"}) {
		t.Fatalf("resolution: %+v", r)
	}
}

func TestLiteralPathJoin(t *testing.T) {
	r := Resolution{}
	r.resolveArgument(`"symmetry" + File.separator + "NQ"`)
	if r.Path != filepath.Join("symmetry", "NQ") || len(r.Reasons) != 0 {
		t.Fatalf("resolution: %+v", r)
	}
}
