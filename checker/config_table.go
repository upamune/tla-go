package checker

import (
	"fmt"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

func validateConfigTables(cfg config.Config) error {
	arities := map[[2]string]int{}
	for _, assignment := range cfg.Constants {
		if assignment.Args == nil {
			continue
		}
		key := [2]string{assignment.Module, assignment.Name}
		if arity, ok := arities[key]; ok && arity != len(assignment.Args) {
			return fmt.Errorf("the arity of operator %s is inconsistent in the configuration file", assignment.Name)
		}
		arities[key] = len(assignment.Args)
	}
	return nil
}

func configTableBody(module *parser.Module, cfg *config.Config, params []parser.Declaration, rows []config.Assignment) (*parser.Expression, error) {
	body := &parser.Expression{Kind: "Case", Text: "!config-table!"}
	for _, row := range rows {
		if len(row.Args) != len(params) {
			return nil, fmt.Errorf("table has incorrect or inconsistent arity")
		}
		condition := &parser.Expression{Kind: "And"}
		for i, arg := range row.Args {
			left := &parser.Expression{Kind: "Identifier", Text: params[i].Name}
			right := addConfigValue(module, cfg, arg)
			condition.Args = append(condition.Args, &parser.Expression{Kind: "Apply", Text: "=", Args: []*parser.Expression{left, right}})
		}
		body.Arms = append(body.Arms, parser.CaseArm{Condition: condition, Body: addConfigValue(module, cfg, row.Value)})
	}
	return body, nil
}

func addConfigValue(module *parser.Module, cfg *config.Config, v config.Value) *parser.Expression {
	name := fmt.Sprintf("!config-value!%d", len(cfg.Constants))
	module.Constants = append(module.Constants, parser.Declaration{Name: name})
	cfg.Constants = append(cfg.Constants, config.Assignment{Name: name, Value: v})
	return &parser.Expression{Kind: "Identifier", Text: name}
}
