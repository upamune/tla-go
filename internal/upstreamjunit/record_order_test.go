package upstreamjunit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A separate JVM per run is essential: UniqueString allocation is global.
func TestPinnedRecordOrderProbe(t *testing.T) {
	root, jar := os.Getenv("TLA_GO_JUNIT_ROOT"), os.Getenv("TLA_GO_JUNIT_JAR")
	if root == "" || jar == "" {
		t.Skip("set TLA_GO_JUNIT_ROOT and TLA_GO_JUNIT_JAR")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	jar, err = filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyInputs(root, jar); err != nil {
		t.Fatal(err)
	}
	classes := t.TempDir()
	cmd := exec.Command("javac", "-cp", jar, "-d", classes, "testdata/record_order/RecordOrderProbe.java")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac: %v\n%s", err, out)
	}
	for _, tc := range []struct{ first, second, token, records, before, after string }{
		{"op", "adr", "tokens=op:1,adr:2", "[op |-> 1, adr |-> 1];[op |-> 1, adr |-> 2]", "[adr |-> 1, op |-> 2]", "[op |-> 2, adr |-> 1]"},
		{"adr", "op", "tokens=adr:1,op:2", "[adr |-> 1, op |-> 1];[adr |-> 2, op |-> 1]", "[adr |-> 1, op |-> 2]", "[adr |-> 1, op |-> 2]"},
	} {
		t.Run(tc.first, func(t *testing.T) {
			want := tc.token + "\nuncached=" + tc.records + "\nsharedNames=true\nliteralBefore=" + tc.before + "\nliteralAfter=" + tc.after + "\ncached=" + tc.records + "\noriginalAfter=" + strings.Split(tc.records, ";")[0] + "\n"
			for range 3 {
				out, err := exec.Command("java", "-cp", classes+string(os.PathListSeparator)+jar, "RecordOrderProbe", tc.first, tc.second).CombinedOutput()
				if err != nil || string(out) != want {
					t.Fatalf("java: %v\ngot:\n%s\nwant:\n%s", err, out, want)
				}
			}
		})
	}
}
