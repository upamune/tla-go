package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
)

// These Java-only observations use TLC 1.7.4 at 5a47802b5c391f59ecdd44117981f4ff8c0656ba.
// They do not assert that Go already models Java's deferred-value cache.
func TestPinnedJavaDeferredValueIdentity(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR to the pinned 1.7.4 jar")
	}
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	const source = `import tlc2.value.impl.*;
public class IdentityProbe {
  static int count(Enumerable e) {
    ValueEnumeration it = e.elements(); int n = 0;
    while (it.nextElement() != null) n++;
    return n;
  }
  static UnionValue union() {
    return new UnionValue(new SetEnumValue(new Value[] {
        new IntervalValue(1, 6), new IntervalValue(2, 7)}, false));
  }
  static SetEnumValue seven() {
    return (SetEnumValue) new IntervalValue(1, 7).toSetEnum();
  }
  public static void main(String[] args) {
    UnionValue u = union(), alias = u, independent = union();
    System.out.println("before=" + count(u));
    u.toSetEnum().size(); System.out.println("after-toSetEnum=" + count(u));
    seven().equals(u); System.out.println("after-right-equals=" + count(u));
    u.equals(seven());
    System.out.println("after-left-equals=" + count(u));
    System.out.println("alias=" + count(alias) + ",independent=" + count(independent));
    UnionValue n = union(); n.deepNormalize();
    System.out.println("deep-copy-same=" + (n.deepCopy() == n));
    System.out.println("deep-normalize-before-size=" + count(n));
    System.out.println("size=" + n.size() + ",after-size=" + count(n));
    SetCupValue cup = new SetCupValue(new IntervalValue(1, 6), new IntervalValue(2, 7));
    System.out.println("cup-before=" + count(cup));
    seven().equals(cup); System.out.println("cup-after-right-equals=" + count(cup));
    System.out.println("cup-size=" + cup.size() + ",after-size=" + count(cup));
    SetCupValue bad = new SetCupValue(new IntervalValue(1, 1),
        new SetEnumValue(new Value[] {BoolValue.ValTrue}, true));
    ValueEnumeration it = bad.elements();
    System.out.println("bad-before=" + it.nextElement() + "," + it.nextElement());
    try { bad.size(); System.out.println("bad-size=unexpected"); }
    catch (RuntimeException e) { System.out.println("bad-size=" + e.getClass().getSimpleName()); }
    try { bad.elements(); System.out.println("bad-after=unexpected"); }
    catch (RuntimeException e) { System.out.println("bad-after=" + e.getClass().getSimpleName()); }
  }
}`
	const expected = `before=12
after-toSetEnum=12
after-right-equals=12
after-left-equals=7
alias=7,independent=12
deep-copy-same=true
deep-normalize-before-size=12
size=7,after-size=7
cup-before=12
cup-after-right-equals=12
cup-size=7,after-size=7
bad-before=1,TRUE
bad-size=TLCRuntimeException
bad-after=TLCRuntimeException
`
	dir := t.TempDir()
	file := filepath.Join(dir, "IdentityProbe.java")
	if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "javac", "-J-Xmx128m", "-cp", jar, file).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v\n%s", err, out)
	}
	classpath := strings.Join([]string{jar, dir}, string(os.PathListSeparator))
	out, err := javatmp.Apply(exec.CommandContext(ctx, "java", "-Xmx128m", "-cp", classpath, "IdentityProbe")).CombinedOutput()
	if err != nil || strings.ReplaceAll(string(out), "\r\n", "\n") != expected {
		t.Fatalf("Java deferred-value observations: %v\n%s", err, out)
	}
}
