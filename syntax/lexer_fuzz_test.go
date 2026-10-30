package syntax

import "testing"

func FuzzTokenize(f *testing.F) {
	for _, source := range []string{
		"---- MODULE M ----\nVARIABLE x\nInit == x = 0\n====\n",
		"---- MODULE M ----\nA == {x \\in 1..3 : x > 1}\n====\n",
		"---- MODULE M ----\n(* nested (* comment *) *)\nA == \"λ\"\n====\n",
	} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 65536 {
			t.Skip()
		}
		_, _ = Tokenize(source)
	})
}
