package checker

import (
	"fmt"
	"maps"
	"slices"

	"github.com/upamune/tla-go/syntax/parser"
)

// Rename local bindings before expansion so nested LETs and arguments retain
// the lexical scope in which they were written.
func (c *ltlCompiler) compileLet(expr *parser.Expression, negative bool, seen map[string]bool) (*ltlFormula, error) {
	if len(expr.Args) != 1 || len(expr.Instances) != 0 {
		return nil, fmt.Errorf("invalid temporal LET")
	}
	c.isolateDefinitions()
	expr = c.renamer.expression(expr, nil)
	for _, definition := range expr.Definitions {
		if invalidLocalSelfReference(definition) {
			return nil, fmt.Errorf("non-recursive local operator refers to itself: %s", definition.Name)
		}
		c.model.defs[definition.Name] = definition
	}
	c.lexical = append(c.lexical, expr.Definitions)
	previousScope := c.scope
	c.nextScope++
	c.scope = c.nextScope
	defer func() {
		for _, definition := range expr.Definitions {
			delete(c.model.defs, definition.Name)
		}
		c.lexical = c.lexical[:len(c.lexical)-1]
		c.scope = previousScope
	}()
	return c.compile(expr.Args[0], negative, seen)
}

func invalidLocalSelfReference(definition parser.Definition) bool {
	if definition.Recursive || definition.Kind == "Function" {
		return false
	}
	names := map[string]bool{}
	collectNames(definition.Body, names)
	return names[definition.Name]
}

func (c *ltlCompiler) isolateDefinitions() {
	if c.ownsDefs {
		return
	}
	copy := *c.model
	copy.defs = maps.Clone(c.model.defs)
	c.model = &copy
	c.ownsDefs = true
}

// Evaluation is performed by the original program, so carry the local
// definitions with each state or action atom and constant expression.
func (c *ltlCompiler) wrapLexical(expr *parser.Expression) *parser.Expression {
	if !c.referencesLocal(expr) {
		return expr
	}
	for _, definitions := range slices.Backward(c.lexical) {
		expr = &parser.Expression{Kind: "Let", Definitions: definitions, Args: []*parser.Expression{expr}, Location: expr.Location}
	}
	return expr
}

func (c *ltlCompiler) referencesLocal(expr *parser.Expression) bool {
	if len(c.lexical) == 0 {
		return false
	}
	if _, ok := c.localDefinition(expr); ok {
		return true
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if !found {
			found = c.referencesLocal(child)
		}
	})
	return found
}

func (c *ltlCompiler) localDefinition(expr *parser.Expression) (parser.Definition, bool) {
	if expr.Kind != "Identifier" && expr.Kind != "Apply" {
		return parser.Definition{}, false
	}
	for _, definitions := range c.lexical {
		for _, definition := range definitions {
			if definition.Name == expr.Text {
				return definition, true
			}
		}
	}
	return parser.Definition{}, false
}

// Pure state operators can be substituted without changing evaluation order
// or side effects. This avoids rebuilding LET scopes for each explored edge.
func (c *ltlCompiler) inlinePureLocal(expr *parser.Expression) *parser.Expression {
	definition, ok := c.localDefinition(expr)
	if !ok || definition.Kind != "Operator" || definition.Recursive || len(definition.Params) != len(expr.Args) {
		return nil
	}
	if !c.pureLocalBody(definition.Body) || !c.safeLocalArguments(definition, expr.Args) {
		return nil
	}
	expanded, err := substituteActionCall(definition, expr)
	if err != nil {
		return nil
	}
	return expanded
}

func (c *ltlCompiler) safeLocalArguments(definition parser.Definition, args []*parser.Expression) bool {
	names := map[string]bool{}
	collectNames(definition.Body, names)
	for i, arg := range args {
		if names[definition.Params[i].Name] && !c.simpleLocalArgument(arg) {
			return false
		}
	}
	return true
}

func (c *ltlCompiler) simpleLocalArgument(arg *parser.Expression) bool {
	if arg.Kind == "Number" || arg.Kind == "String" {
		return true
	}
	if arg.Kind != "Identifier" {
		return false
	}
	_, local := c.locals[arg.Text]
	_, constant := c.model.constants[arg.Text]
	return local || constant || c.model.variables[arg.Text] || arg.Text == "TRUE" || arg.Text == "FALSE"
}

func (c *ltlCompiler) pureLocalBody(body *parser.Expression) bool {
	return !c.model.hasTemporal(body, map[string]bool{}) && !c.model.hasAction(body, map[string]bool{}) && !c.model.expressionHasEffects(body, map[string]bool{}) && !c.model.readsRandom(body, map[string]bool{})
}
