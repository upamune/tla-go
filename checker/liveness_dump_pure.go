package checker

import (
	"slices"

	"github.com/upamune/tla-go/syntax/parser"
)

// This is a deliberately closed expression language: unrecognized syntax,
// operators, modules and operator-valued arguments disable reconstruction.
func (m *model) pureLivenessDumpModel() bool {
	return m.pureLivenessDumpRoot() && m.pureLivenessDumpDefinitions()
}

func (m *model) pureLivenessDumpRoot() bool {
	root := m.sourceUnits[m.module.Name]
	if root == nil || len(root.Instances) != 0 || len(root.Modules) != 0 || m.parallelInstances() {
		return false
	}
	for _, module := range root.Extends {
		if !slices.Contains([]string{"Naturals", "Integers", "Sequences", "FiniteSets", "TLC"}, module) {
			return false
		}
	}
	return true
}

func (m *model) pureLivenessDumpDefinitions() bool {
	for _, def := range m.module.Definitions {
		if def.Recursive || len(def.Params) != 0 || len(def.Bounds) != 0 || !m.pureLivenessDumpExpr(def.Body) {
			return false
		}
	}
	return true
}

func (m *model) pureLivenessDumpExpr(expr *parser.Expression) bool {
	if expr == nil || !plainLivenessDumpSyntax(expr) || !m.pureLivenessDumpKind(expr) {
		return false
	}
	valid := true
	walkChildren(expr, func(child *parser.Expression) {
		if valid {
			valid = m.pureLivenessDumpExpr(child)
		}
	})
	return valid
}

func plainLivenessDumpSyntax(expr *parser.Expression) bool {
	return len(expr.Bounds)+len(expr.Updates)+len(expr.Definitions)+len(expr.Instances)+len(expr.Selectors)+len(expr.EnabledBindings)+len(expr.EnabledScopes)+len(expr.InstanceScopes)+len(expr.RecursiveDeclarations) == 0
}

func (m *model) pureLivenessDumpKind(expr *parser.Expression) bool {
	switch expr.Kind {
	case "Number", "String":
		return len(expr.Args) == 0
	case "Identifier":
		return len(expr.Args) == 0 && m.pureLivenessDumpIdentifier(expr.Text)
	case "Apply":
		return pureLivenessDumpOperator(expr)
	case "Prime", "Unchanged", "Always", "Eventually", "Enabled":
		return len(expr.Args) == 1
	case "If":
		return len(expr.Args) == 3
	case "And", "Or", "Product", "Tuple", "SetEnumeration", "Record", "RecordSet", "Case", "FieldAccess", "FunctionSet", "BoxAction", "AngleAction", "WeakFairness", "StrongFairness":
		return true
	default:
		return false
	}
}

func (m *model) pureLivenessDumpIdentifier(name string) bool {
	if slices.Contains([]string{"TRUE", "FALSE", "BOOLEAN", "STRING", "Nat", "Int"}, name) || m.variables[name] {
		return true
	}
	if _, ok := m.constants[name]; ok {
		return true
	}
	def, ok := m.defs[name]
	return ok && len(def.Params) == 0 && len(def.Bounds) == 0
}

func pureLivenessDumpOperator(expr *parser.Expression) bool {
	name := tlcOperatorName(expr)
	if len(expr.Args) == 1 {
		return slices.Contains([]string{"~", "-", "-.", "DOMAIN", "SUBSET", "UNION"}, name)
	}
	return len(expr.Args) == 2 && slices.Contains([]string{"=", "#", "/=", "<", ">", "<=", ">=", "+", "-", "*", "%", "\\div", "..", "\\in", "\\notin", "\\subseteq", "\\cup", "\\cap", "\\", "=>", "<=>"}, name)
}
