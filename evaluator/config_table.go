package evaluator

import (
	"fmt"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
)

// configTableError reconstructs TLC's table diagnostic after all guards fail.
func (e *evaluator) configTableError(expr *parser.Expression) error {
	if len(expr.Arms) == 0 {
		return fmt.Errorf("empty configuration table")
	}
	rows := make([]string, 0, len(expr.Arms))
	for _, arm := range expr.Arms {
		row, err := e.configTableRow(arm)
		if err != nil {
			return err
		}
		rows = append(rows, row)
	}
	arguments, err := e.configTableArguments(expr.Arms[0])
	if err != nil {
		return err
	}
	message := fmt.Sprintf("Attempted to apply operator:\n{ %s}\nto arguments (%s), which is undefined.", strings.Join(rows, ", "), strings.Join(arguments, ", "))
	return &RuntimeError{Message: message}
}

func (e *evaluator) configTableRow(arm parser.CaseArm) (string, error) {
	values := make([]string, 0, len(arm.Condition.Args)+1)
	for _, comparison := range arm.Condition.Args {
		v, err := e.eval(comparison.Args[1])
		if err != nil {
			return "", err
		}
		values = append(values, v.String())
	}
	result, err := e.eval(arm.Body)
	if err != nil {
		return "", err
	}
	values = append(values, result.String())
	return "<" + strings.Join(values, ", ") + ">", nil
}

func (e *evaluator) configTableArguments(arm parser.CaseArm) ([]string, error) {
	values := make([]string, 0, len(arm.Condition.Args))
	for _, comparison := range arm.Condition.Args {
		v, err := e.eval(comparison.Args[0])
		if err != nil {
			return nil, err
		}
		values = append(values, v.String())
	}
	return values, nil
}
