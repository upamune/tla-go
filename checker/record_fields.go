package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

func validateRecordFields(expr *parser.Expression) error {
	if expr.Kind != "Record" && expr.Kind != "RecordSet" {
		return nil
	}
	seen := make(map[string]bool)
	for _, field := range expr.Fields {
		if seen[field.Name] {
			position := expr.Location.Position
			if expr.DuplicateField != nil {
				position = *expr.DuplicateField
			}
			return &SemanticError{Err: fmt.Errorf("Non-unique fields in constructor at line %d, column %d", position.Line, position.Column)}
		}
		seen[field.Name] = true
	}
	return nil
}
