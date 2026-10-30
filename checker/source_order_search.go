package checker

import "github.com/upamune/tla-go/syntax/parser"

func (o *sourceOrder) rankDependencies(root *sourceReferences, ranks map[string]int) {
	parsed := map[string]bool{root.unit: true}
	for {
		scope, name := o.findReference(root, true)
		if scope == nil {
			scope, name = o.findReference(root, false)
		}
		if scope == nil {
			return
		}
		o.relations[sourceRelation{scope, name}] = true
		o.dependencies[scope.unit] = append(o.dependencies[scope.unit], name)
		if !parsed[name] {
			parsed[name] = true
			if unit := sourceUnit(name, o.units); unit != nil {
				rankParsedTokens(unit.SourceTokens, ranks, false)
			}
		}
	}
}

func (o *sourceOrder) findReference(root *sourceReferences, extensions bool) (*sourceReferences, string) {
	return o.searchReference(root, extensions, map[*sourceReferences]bool{})
}

func (o *sourceOrder) searchReference(scope *sourceReferences, extensions bool, visited map[*sourceReferences]bool) (*sourceReferences, string) {
	if visited[scope] {
		return nil, ""
	}
	visited[scope] = true
	names := scope.instances
	if extensions {
		names = scope.extends
	}
	for _, name := range names {
		if !o.resolved(scope, name) {
			return scope, name
		}
	}
	return o.searchChildren(scope, extensions, visited)
}

func (o *sourceOrder) resolved(scope *sourceReferences, name string) bool {
	return o.inner[name] != nil || o.relations[sourceRelation{scope, name}]
}

func (o *sourceOrder) searchChildren(scope *sourceReferences, extensions bool, visited map[*sourceReferences]bool) (*sourceReferences, string) {
	for _, child := range o.children(scope) {
		if found, name := o.searchReference(child, extensions, visited); found != nil {
			return found, name
		}
	}
	return nil, ""
}

func (o *sourceOrder) children(scope *sourceReferences) []*sourceReferences {
	children := make([]*sourceReferences, 0, len(scope.extends)+len(scope.instances)+len(scope.inner))
	for _, name := range scope.extends {
		children = append(children, o.referenced(name))
	}
	for _, name := range scope.instances {
		if o.resolved(scope, name) {
			children = append(children, o.referenced(name))
		}
	}
	return append(children, scope.inner...)
}

func (o *sourceOrder) rankSemanticUnit(name string, unit *parser.Module, analyzed map[string]bool, ranks map[string]int) {
	if analyzed[name] {
		return
	}
	analyzed[name] = true
	refs := o.unitReferences(name)
	for _, extension := range []bool{true, false} {
		o.rankSemanticDependencies(name, refs, extension, analyzed, ranks)
	}
	rankSemanticTokens(unit.SourceTokens, ranks)
}

func (o *sourceOrder) rankSemanticDependencies(name string, refs *sourceReferences, extension bool, analyzed map[string]bool, ranks map[string]int) {
	for _, dependency := range o.dependencies[name] {
		if refs.allExtends[dependency] != extension {
			continue
		}
		if child := sourceUnit(dependency, o.units); child != nil {
			o.rankSemanticUnit(dependency, child, analyzed, ranks)
		}
	}
}
