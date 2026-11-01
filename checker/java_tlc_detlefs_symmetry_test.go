package checker

import (
	"os"
	"path/filepath"
	"testing"
)

// The reduced Detlefs model exercises permutations of nested functions.
// Its fixture is derived from the Java TLC upstream regression model.
func TestJavaTLCDetlefsSymmetry(t *testing.T) {
	dir := filepath.Join("testdata", "detlefs")
	source, err := os.ReadFile(filepath.Join(dir, "Detlefs.tla"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := os.ReadFile(filepath.Join(dir, "Detlefs.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Check(string(source), string(cfg))
	if err != nil || got.Status != Success || got.GeneratedStates != 23821 || got.DistinctStates != 10149 || got.Depth != 42 {
		t.Fatalf("got %+v, %v; want 23821 generated, 10149 distinct, depth 42", got, err)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaTLCModel(t, jar, "Detlefs", source, cfg, 23821, 10149, 42)
	}
}
