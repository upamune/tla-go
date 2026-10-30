package checker

import (
	"slices"

	"github.com/upamune/tla-go/syntax/parser"
)

func addInstanceStandardModules(module *parser.Module, names ...string) {
	module.InstanceStandardModules = appendUniqueModules(module.InstanceStandardModules, names...)
}

func appendUniqueModules(existing []string, names ...string) []string {
	for _, name := range names {
		if !slices.Contains(existing, name) {
			existing = append(existing, name)
		}
	}
	return existing
}
