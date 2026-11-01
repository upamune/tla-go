package checker

import (
	"maps"
	"slices"

	"github.com/upamune/tla-go/syntax/parser"
)

// Property preparation runs after binding constants and loading source units.
// Those inputs do not change during property expansion, so all properties can
// share the model-wide purity result. Preparation is sequential.
func (m *model) cachedJava17PureEnvironment() bool {
	if !m.java17PureChecked {
		m.java17Pure = m.java17PureEnvironment()
		m.java17PureChecked = true
	}
	return m.java17Pure
}

// Limit the alternate lasso search to a closed, side-effect-free language.
// Rechecking fairness and successor states is observable for TLC runtime,
// random, file I/O and higher-order operators, including through aliases.
func (m *model) java17PureEnvironment() bool {
	root := m.sourceUnits[m.module.Name]
	if root == nil || len(m.sourceUnits) != 1 || !java17PureRoot(root) {
		return false
	}
	if !java17PureConfig(m) || !java17PureModules(root.Extends) {
		return false
	}
	names := java17EnvironmentNames(m, root)
	for _, definition := range root.Definitions {
		if !java17PureModelDefinition(definition, names) {
			return false
		}
	}
	return true
}

func java17PureRoot(root *parser.Module) bool {
	return len(root.Instances)+len(root.Modules)+len(root.Assumptions)+len(root.Theorems) == 0
}

func java17PureModelDefinition(definition parser.Definition, names map[string]bool) bool {
	return definition.Kind == "Operator" && !definition.Recursive && len(definition.Params)+len(definition.Bounds) == 0 && java17PureModelExpr(definition.Body, names)
}

func java17PureConfig(m *model) bool {
	if m.sourceConfig.Periodic != "" || m.sourceConfig.RLReward != "" || len(m.sourceConfig.Substitutions) != 0 {
		return false
	}
	return java17PureAssignments(m)
}

func java17PureAssignments(m *model) bool {
	for _, assignment := range m.sourceConfig.Constants {
		if assignment.Module != "" || len(assignment.Args) != 0 {
			return false
		}
	}
	for _, v := range m.constants {
		if !scalarConstant(v) {
			return false
		}
	}
	return true
}

func java17PureModules(modules []string) bool {
	for _, name := range modules {
		if !slices.Contains([]string{"Naturals", "Integers", "Sequences", "FiniteSets", "TLC"}, name) {
			return false
		}
	}
	return true
}

func java17EnvironmentNames(m *model, root *parser.Module) map[string]bool {
	names := map[string]bool{"TRUE": true, "FALSE": true}
	for name := range m.variables {
		names[name] = true
	}
	for name := range m.constants {
		names[name] = true
	}
	for _, definition := range root.Definitions {
		names[definition.Name] = true
	}
	return names
}

func java17PureModelExpr(expr *parser.Expression, names map[string]bool) bool {
	if expr == nil || java17ExtraModelChildren(expr) {
		return false
	}
	switch expr.Kind {
	case "Apply":
		return java17PureModelOperator(expr.Text) && java17PureModelArgs(expr.Args, names)
	case "Forall", "Exists":
		return java17PureModelQuantifier(expr, names)
	default:
		return java17PureModelLeafOrComposite(expr, names)
	}
}

func java17PureModelLeafOrComposite(expr *parser.Expression, names map[string]bool) bool {
	switch expr.Kind {
	case "Number", "String":
		return len(expr.Args) == 0
	case "Identifier":
		return len(expr.Args) == 0 && expr.Text != "JavaTime" && names[expr.Text]
	case "And", "Or", "If", "Prime", "Unchanged", "SetEnumeration", "Tuple", "Always", "Eventually", "BoxAction", "AngleAction", "WeakFairness", "StrongFairness":
		return java17PureModelArgs(expr.Args, names)
	default:
		return false
	}
}

func java17ExtraModelChildren(expr *parser.Expression) bool {
	return expr.BuiltinTLC || expr.BuiltinNatural || expr.BuiltinStandard != "" || len(expr.Names)+len(expr.ArgumentCounts)+len(expr.RecursiveDeclarations)+len(expr.Fields)+len(expr.Arms)+len(expr.Updates)+len(expr.Definitions)+len(expr.Instances)+len(expr.Selectors)+len(expr.EnabledBindings)+len(expr.EnabledScopes)+len(expr.InstanceScopes) != 0 || len(expr.Bounds) != 0 && expr.Kind != "Forall" && expr.Kind != "Exists"
}

func java17PureModelOperator(name string) bool {
	return slices.Contains([]string{"=", "#", "/=", "<", "<=", ">", ">=", "+", "-", "*", "%", "\\div", "\\in", "\\notin", "=>", "<=>", "~"}, name)
}

func java17PureModelArgs(args []*parser.Expression, names map[string]bool) bool {
	for _, arg := range args {
		if !java17PureModelExpr(arg, names) {
			return false
		}
	}
	return true
}

func java17PureModelQuantifier(expr *parser.Expression, names map[string]bool) bool {
	scope := maps.Clone(names)
	for _, bound := range expr.Bounds {
		if !java17PureModelExpr(bound.Set, scope) {
			return false
		}
		for _, name := range bound.Names {
			scope[name] = true
		}
	}
	return java17PureModelArgs(expr.Args, scope)
}
