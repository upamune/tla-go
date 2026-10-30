package checker

import "github.com/upamune/tla-go/syntax/parser"

func (s *metadataScope) assumeProveLevel(expr *parser.Expression, visiting map[*metadataDefinition]bool) (int, error) {
	local := s.clone()
	level := 0
	for _, clause := range expr.Args {
		if clause.Kind == "NewDeclaration" {
			bound, err := local.newDeclarationLevel(clause, visiting)
			if err != nil {
				return 0, err
			}
			level = max(level, bound)
			continue
		}
		value, err := local.expression(clause, visiting)
		if err != nil {
			return 0, err
		}
		level = max(level, value)
	}
	return level, nil
}

func (s *metadataScope) newDeclarationLevel(expr *parser.Expression, visiting map[*metadataDefinition]bool) (int, error) {
	level := 0
	for _, bound := range expr.Args {
		value, err := s.expression(bound, visiting)
		if err != nil {
			return 0, err
		}
		level = max(level, value)
	}
	var declarationLevel int
	switch expr.Text {
	case "VARIABLE", "STATE":
		declarationLevel = 1
	case "ACTION":
		declarationLevel = 2
	}
	s.bind(expr.Names[0], declarationLevel)
	return level, nil
}
