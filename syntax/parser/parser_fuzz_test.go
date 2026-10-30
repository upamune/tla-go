package parser

import "testing"

func FuzzParseModule(f *testing.F) {
	for _, source := range []string{
		"---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1\n====\n",
		"---- MODULE M ----\nA == LET F(x) == x IN F(1)\n====\n",
		"---- MODULE M ----\nA == \\A n \\in {1, 2} : n > 0\n====\n",
	} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 65536 {
			t.Skip()
		}
		_, _ = ParseModule(source)
	})
}
