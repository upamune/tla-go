package checker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/value"
)

func TestRankedIntervalComparison(t *testing.T) {
	for _, tc := range []struct {
		a, b value.Value
		want int
	}{
		{value.Interval(1, 10000), value.Interval(0, 0), 1},
		{value.Interval(-2, 1), value.Interval(3, 6), -1},
		{value.Interval(-2147483648, -2147483648), value.Interval(2147483647, 2147483647), -1},
		{value.Interval(3, 2), value.Interval(5, 4), 0},
		{value.Interval(1, 2), value.Interval(1, 2), 0},
	} {
		m := model{rankedSets: make(map[value.Value][]value.Value)}
		got, err := m.compareOrderedValues(tc.a, tc.b)
		if err != nil || got != tc.want {
			t.Fatalf("compare %v with %v = %d, %v; want %d", tc.a, tc.b, got, err, tc.want)
		}
	}
	if !rankedIntervalsFit(value.Interval(-2147483648, -2), value.Interval(1, 2147483647)) {
		t.Fatal("maximal representable interval must use the optimized comparison")
	}
}

// IntervalValue.compareTo uses the 32-bit size of each interval, then its
// lower endpoint. Keep the overflow case off the optimized path.
func TestJavaTLCIntervalComparison(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to compare the pinned Java interval implementation")
	}
	data, err := os.ReadFile(jar)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88" {
		t.Fatalf("TLC_JAR must be Java TLC 1.7.4: %v", err)
	}
	jar, err = filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	const source = `import tlc2.value.impl.IntervalValue;
public class IntervalOracle {
  public static void main(String[] args) {
    int[][] values = {{1,2147483647,0,0}, {-2,1,3,6}, {3,2,5,4},
      {1,2,1,2}, {-2147483648,-2,-2147483647,-1},
      {-2147483648,-2147483648,2147483647,2147483647},
      {-2147483648,-1,0,0}};
    for (int[] v : values) {
      try {
        System.out.println(Integer.signum(new IntervalValue(v[0],v[1]).compareTo(new IntervalValue(v[2],v[3]))));
      } catch (RuntimeException e) {
        if (!e.getMessage().contains("Size of interval value exceeds")) throw e;
        System.out.println("overflow");
      }
    }
  }
}`
	file := filepath.Join(dir, "IntervalOracle.java")
	if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "javac", "-J-Xmx128m", "-cp", jar, file).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v\n%s", err, out)
	}
	cp := strings.Join([]string{jar, dir}, string(os.PathListSeparator))
	out, err := exec.CommandContext(ctx, "java", "-Xmx128m", "-cp", cp, "IntervalOracle").CombinedOutput()
	if err != nil || string(out) != "1\n-1\n0\n0\n-1\n-1\noverflow\n" {
		t.Fatalf("Java interval comparison: %v\n%s", err, out)
	}
	if rankedIntervalsFit(value.Interval(-2147483648, -1), value.Interval(0, 0)) {
		t.Fatal("overflowing interval must not use the optimized comparison")
	}
}

func BenchmarkRankedIntervalComparison(b *testing.B) {
	a, c := value.Interval(1, 10000), value.Interval(0, 0)
	b.ReportAllocs()
	for b.Loop() {
		m := model{rankedSets: make(map[value.Value][]value.Value)}
		if _, err := m.compareOrderedValues(a, c); err != nil {
			b.Fatal(err)
		}
	}
}
