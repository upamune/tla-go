package main

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

// The expected orders are the "Parsing file" lines Java TLC 1.7.4 logs for
// these specifications, without the root module.
func TestSpecTEParseOrder(t *testing.T) {
	for _, tc := range []struct {
		name, root string
		modules    map[string]string
		want       []string
	}{
		{
			"instance after nested extends", "---- MODULE M ----\nEXTENDS H1, TLC, FiniteSets\nI == INSTANCE H3 WITH C <- 2\n====",
			map[string]string{"H1": "EXTENDS Naturals, H2", "H2": "EXTENDS Sequences", "H3": "CONSTANT C"},
			[]string{"H1", "TLC", "FiniteSets", "Naturals", "H2", "Sequences", "H3"},
		},
		{
			"inner modules", "---- MODULE M ----\nEXTENDS H1, Naturals\n---- MODULE Inner ----\nEXTENDS H2\n====\n---- MODULE Inner2 ----\nEXTENDS Inner\n====\nI == INSTANCE Inner2\nJ == INSTANCE H3\n====",
			map[string]string{"H1": "EXTENDS Naturals", "H2": "EXTENDS Naturals", "H3": "EXTENDS H4", "H4": ""},
			[]string{"H1", "Naturals", "H2", "H3", "H4"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, err := parser.ParseModule(tc.root)
			if err != nil {
				t.Fatal(err)
			}
			loader := func(_ context.Context, name string) (string, error) {
				body, ok := tc.modules[name]
				if !ok {
					return "", os.ErrNotExist
				}
				return fmt.Sprintf("---- MODULE %s ----\n%s\n====", name, body), nil
			}
			walk := specTEModuleWalk{loader: loader, parsed: map[string]*parser.Module{root.Name: root}, sources: map[string]string{}}
			if got := walk.parseOrder(root); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
