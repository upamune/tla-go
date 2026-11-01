package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"

	"github.com/upamune/tla-go/internal/javatmp"
)

func TestJavaTLCSimulationLevelAcrossTraces(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLE x
Init == x = 0
Next == /\ PrintT(<<"next", TLCGet("level")>>) /\ x' = x+1
Inv == PrintT(<<"inv", TLCGet("level")>>)
Bound == PrintT(<<"bound", TLCGet("level")>>)
====
`
	const cfg = "INIT Init\nNEXT Next\nINVARIANT Inv\nCONSTRAINT Bound\nCHECK_DEADLOCK FALSE\n"
	dir := writeSimulationFixture(t, source, cfg)
	var out, errout bytes.Buffer
	args := []string{"-tool", "-workers", "1", "-fp", "0", "-simulate", "num=2", "-seed", "1", "-depth", "2", filepath.Join(dir, "M.tla")}
	code := run(context.Background(), args, &out, &errout)
	if code != 0 {
		t.Fatalf("Go exit %d; stdout %s; stderr %s", code, out.String(), errout.String())
	}
	pattern := regexp.MustCompile(`(?m)^<<"(?:inv|bound|next)", [0-9]+>>$`)
	got := pattern.FindAllString(out.String(), -1)
	want := []string{`<<"inv", 0>>`, `<<"bound", 1>>`, `<<"next", 1>>`, `<<"inv", 1>>`, `<<"bound", 1>>`, `<<"next", 2>>`, `<<"inv", 2>>`, `<<"bound", 2>>`, `<<"bound", 1>>`, `<<"next", 1>>`, `<<"inv", 1>>`, `<<"bound", 1>>`, `<<"next", 2>>`, `<<"inv", 2>>`, `<<"bound", 2>>`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Go levels %q; want %q", got, want)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "-simulate", "num=2", "-seed", "1", "-depth", "2", "M.tla")
		cmd.Dir = dir
		output, err := javatmp.Apply(cmd).CombinedOutput()
		if err != nil {
			t.Fatalf("Java TLC: %v\n%s", err, output)
		}
		if java := pattern.FindAllString(string(output), -1); !reflect.DeepEqual(got, java) {
			t.Fatalf("Go levels %q; Java levels %q", got, java)
		}
	}
}
