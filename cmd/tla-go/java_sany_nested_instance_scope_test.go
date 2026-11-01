package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Compare the root declaration projection with the pinned SANY XML exporter.
func TestJavaSANYNestedInstanceScopeMetadata(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR to compare SANY metadata")
	}
	for _, tc := range []struct{ name, body string }{
		{"enclosing", "---- MODULE N ---- A == 1 ==== J == INSTANCE N ---- MODULE R ---- V == J!A ==== I == INSTANCE R"},
		{"local enclosing", "---- MODULE N ---- A == 1 ==== LOCAL J == INSTANCE N ---- MODULE R ---- V == J!A ==== I == INSTANCE R"},
		{"inner exported", "---- MODULE N ---- A == 1 ==== ---- MODULE R ---- J == INSTANCE N V == J!A ==== I == INSTANCE R"},
		{"inner different name", "---- MODULE N ---- A == 1 ==== J == INSTANCE N ---- MODULE R ---- K == INSTANCE N V == K!A ==== I == INSTANCE R"},
		{"shadowed instance", "---- MODULE N ---- A == 1 ==== ---- MODULE O ---- A == 2 ==== J == INSTANCE N ---- MODULE R ---- J == INSTANCE O V == J!A ==== I == INSTANCE R"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "M.tla")
			source := "---- MODULE M ----\n" + tc.body + "\nVARIABLE x\nInit == x = I!V\nNext == UNCHANGED x\n====\n"
			if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			compareSANYMetadata(t, jar, path)
		})
	}
}

func TestJavaSANYInheritedNestedInstanceScopeMetadata(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR to compare SANY metadata")
	}
	dir := t.TempDir()
	for name, source := range map[string]string{
		"A.tla": "---- MODULE A ----\n---- MODULE N ---- A == 1 ====\nJ == INSTANCE N\n---- MODULE R ---- V == J!A ====\n====\n",
		"M.tla": "---- MODULE M ----\nEXTENDS A\nI == INSTANCE R\nVARIABLE x\nInit == x = I!V\nNext == UNCHANGED x\n====\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	compareSANYMetadata(t, jar, filepath.Join(dir, "M.tla"))
}

func TestJavaSANYNestedInstanceScopeStateMetadata(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR to compare SANY metadata")
	}
	path := filepath.Join(t.TempDir(), "M.tla")
	source := "---- MODULE M ----\nVARIABLE x\n---- MODULE N ---- VARIABLE y A == y ====\nJ == INSTANCE N WITH y <- x\n---- MODULE R ---- V == J!A ====\nI == INSTANCE R\nInit == x = 1 /\\ I!V = 1\nNext == UNCHANGED x\n====\n"
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	compareSANYMetadata(t, jar, path)
}
