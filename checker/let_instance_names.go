package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

func validateLetInstanceNames(expr *parser.Expression) error {
	names := make(map[string]bool, len(expr.Definitions)+len(expr.Instances))
	for _, definition := range expr.Definitions {
		names[definition.Name] = true
	}
	for _, instance := range expr.Instances {
		if instance.Name == "" {
			continue
		}
		if names[instance.Name] {
			return fmt.Errorf("duplicate LET instance name %s", instance.Name)
		}
		names[instance.Name] = true
	}
	return nil
}
