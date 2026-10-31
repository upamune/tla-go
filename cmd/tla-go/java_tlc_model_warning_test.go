package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Java TLC warns (2300) when a symmetry set has fewer than two elements. The
// cases follow the upstream Github432Test, which substitutes the elements of
// Humans and Others into Github432.cfg.
func TestJavaTLCSymmetrySetWarning(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "Github432.tla"))
	if err != nil {
		t.Fatal(err)
	}
	template, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "Github432.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, humans, others string
		extra                []string
		want                 []string
	}{
		{"one human", "Alice", "Cat, Dog", nil, []string{"The set Humans has been defined to be a symmetry set but contains less than two elements."}},
		{"no humans", "", "Emu", nil, []string{"The sets Humans, and Others have been defined to be a symmetry set but contain less than two elements."}},
		{"no others", "Frank, Glenda", "", nil, []string{"The set Others has been defined to be a symmetry set but contains less than two elements."}},
		{"no warning", "Hauser, Ignatio", "Jackal, Kangaroo", nil, nil},
		{"simulation", "Alice", "Cat, Dog", []string{"-simulate", "num=1", "-depth", "2", "-seed", "1"}, []string{"The set Humans has been defined to be a symmetry set but contains less than two elements."}},
		{"nowarning", "Alice", "Cat, Dog", []string{"-nowarning"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := strings.Replace(strings.Replace(string(template), "%1%", tc.humans, 1), "%2%", tc.others, 1)
			if err := os.WriteFile(filepath.Join(dir, "Github432.tla"), source, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "Github432.cfg"), []byte(cfg), 0o600); err != nil {
				t.Fatal(err)
			}
			args := append(append([]string{"-tool", "-workers", "1", "-fp", "0"}, tc.extra...), "Github432.tla")
			messages, code, stderr := runGoToolInDir(t, dir, args)
			if got := toolMessageBodies(messages, 2300); stderr != "" || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("exit %d; warnings %q, want %q; stderr %s", code, got, tc.want, stderr)
			}
			// Like the upstream test, compare only the warnings: with no humans,
			// Java TLC finds no next-state actions where tla-go reports deadlock.
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, _ := runJavaTLCArgs(t, jar, dir, args)
				if got := toolMessageBodies(readToolMessages(java), 2300); !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("Java warnings %q\n%s", got, java)
				}
			}
		})
	}
}

// Java TLC warns (2144) and leaves the value unchanged when an EXCEPT path is
// outside a function's domain, as in the upstream PlusCal StackTest model.
func TestJavaTLCExceptUnknownFieldWarning(t *testing.T) {
	const at = "The EXCEPT was applied to non-existing fields of the value at\n"
	for _, tc := range []struct {
		name, init, next string
		extra            []string
		want             []string
	}{
		{"tuple index", "f = <<0, 0>>", "f' = [f EXCEPT ![3] = 1, ![1] = @ + 1]", nil, []string{at + "line 6, col 18 to line 6, col 18 of module M"}},
		{"record field", "f = [a |-> 0]", "f' = [f EXCEPT !.b = 1, !.a = @ + 1]", nil, []string{at + "line 6, col 18 to line 6, col 18 of module M"}},
		{"nested path", "f = <<<<0>>, <<0>>>>", "f' = [f EXCEPT ![1][2] = 1, ![2][1] = @ + 1]", nil, []string{at + "line 6, col 18 to line 6, col 18 of module M"}},
		{"initial state", "f = [<<0>> EXCEPT ![0] = 1]", "f' = [f EXCEPT ![1] = @ + 1]", nil, []string{at + "line 4, col 14 to line 4, col 18 of module M"}},
		{"defined key", "f = <<0>>", "f' = [f EXCEPT ![1] = @ + 1]", nil, nil},
		{"nowarning", "f = <<0, 0>>", "f' = [f EXCEPT ![3] = 1, ![1] = @ + 1]", []string{"-nowarning"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE f\nInit == " + tc.init + "\nNext == /\\ TLCGet(\"level\") < 3\n        /\\ " + tc.next
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\n")
			args := append(append([]string{"-tool", "-workers", "1", "-fp", "0"}, tc.extra...), "M.tla")
			messages, code, stderr := runGoToolInDir(t, dir, args)
			if got := toolMessageBodies(messages, 2144); code != 11 || stderr != "" || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("exit %d; warnings %q, want %q; stderr %s", code, got, tc.want, stderr)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				javaMessages := readToolMessages(java)
				if javaCode != code || !reflect.DeepEqual(toolMessageBodies(javaMessages, 2144), tc.want) || !reflect.DeepEqual(traceStateMessages(javaMessages), traceStateMessages(messages)) {
					t.Fatalf("Java exit %d, Go exit %d\nJava:\n%s\nGo: %+v", javaCode, code, java, messages)
				}
			}
		})
	}
}

func toolMessageBodies(messages []toolMessage, code int) []string {
	var bodies []string
	for _, message := range messages {
		if message.code == code {
			bodies = append(bodies, message.body)
		}
	}
	return bodies
}
