package checker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
)

// Compare observable EXCEPT callbacks and exploration with Java TLC 1.7.4.
func TestJavaTLCNoopExceptEffects(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Naturals, TLC
CONSTANTS A, B
VARIABLES f, x
Init == /\ f = [i \in 1..2 |-> i] /\ x = 0
Next == /\ f' = [f EXCEPT ![1] = IF PrintT(<<x, @>>) THEN @ ELSE @]
        /\ x' = (x + 1) % 2
P == <>(x = 0)
V == <<f, x>>
Sym == Permutations({A, B})
====`
	for _, tc := range []struct {
		name, value string
		printed     []string
	}{
		{"integers", "i", []string{"<<0, 1>>", "<<1, 1>>"}},
		{"strings", `IF i = 1 THEN "first" ELSE "second"`, []string{`<<0, "first">>`, `<<1, "first">>`}},
	} {
		for _, extra := range []string{"", "VIEW V\n", "SYMMETRY Sym\n", "VIEW V\nSYMMETRY Sym\n"} {
			t.Run(fmt.Sprintf("%s/reduction-%q", tc.name, extra), func(t *testing.T) {
				source := strings.Replace(source, `[i \in 1..2 |-> i]`, `[i \in 1..2 |-> `+tc.value+`]`, 1)
				cfg := "CONSTANTS A = A B = B\nINIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n" + extra
				var printed []string
				result, err := CheckWithOptions(context.Background(), source, cfg, Options{OnPrint: func(s string) { printed = append(printed, s) }})
				if err != nil || result.Status != Success || result.GeneratedStates != 3 || result.DistinctStates != 2 {
					t.Fatalf("Go result: %+v, %v", result, err)
				}
				if !reflect.DeepEqual(printed, tc.printed) {
					t.Fatalf("Go prints %q; want %q", printed, tc.printed)
				}
				if jar := os.Getenv("TLC_JAR"); jar != "" {
					compareNoopExceptJava(t, jar, source, cfg, tc.printed)
				}
			})
		}
	}
}

// ENABLED must still evaluate the action and its callback on every invocation.
func TestJavaTLCNoopExceptEnabledEffects(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLES f, x
Init == /\ f = [i \in 1..2 |-> i] /\ x = 0 /\ TLCSet(0, 0)
A == /\ f' = [f EXCEPT ![1] = IF TLCSet(0, TLCGet(0) + 1) THEN @ ELSE @]
     /\ x' = (x + 1) % 2
Next == /\ ENABLED A /\ A /\ PrintT(TLCGet(0))
P == <>(x = 0)
====`
	cfg := "INIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n"
	var printed []string
	result, err := CheckWithOptions(context.Background(), source, cfg, Options{OnPrint: func(s string) { printed = append(printed, s) }})
	want := []string{"2", "4"}
	if err != nil || result.Status != Success || result.GeneratedStates != 3 || result.DistinctStates != 2 || !reflect.DeepEqual(printed, want) {
		t.Fatalf("Go result: %+v, prints=%q, error=%v", result, printed, err)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareNoopExceptJava(t, jar, source, cfg, want)
	}
}

func compareNoopExceptJava(t *testing.T, jar, source, cfg string, want []string) {
	t.Helper()
	dir := t.TempDir()
	for name, content := range map[string]string{"M.tla": source, "M.cfg": cfg} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "-checkpoint", "0", "M.tla")
	cmd.Dir = dir
	output, err := javatmp.Apply(cmd).CombinedOutput()
	if err != nil || ctx.Err() != nil {
		t.Fatalf("Java: %v, %v\n%s", err, ctx.Err(), output)
	}
	var printed []string
	for line := range strings.SplitSeq(string(output), "\n") {
		_, numberErr := strconv.Atoi(line)
		if strings.HasPrefix(line, "<<") || numberErr == nil {
			printed = append(printed, line)
		}
	}
	if !reflect.DeepEqual(printed, want) || !strings.Contains(string(output), "3 states generated, 2 distinct states found") {
		t.Fatalf("Java prints %q; want %q\n%s", printed, want, output)
	}
}
