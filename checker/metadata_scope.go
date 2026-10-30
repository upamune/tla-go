package checker

import (
	"fmt"
	"maps"
	"slices"

	"github.com/upamune/tla-go/syntax/parser"
)

type metadataDefinition struct {
	value parser.Definition
	scope *metadataScope
}
type metadataScope struct {
	values        map[string]int
	defs          map[string]*metadataDefinition
	tlc           bool
	instanceProbe bool
}

func newMetadataScope(module *parser.Module) *metadataScope {
	s := &metadataScope{values: map[string]int{}, defs: map[string]*metadataDefinition{}, tlc: slices.Contains(module.Extends, "TLC")}
	for _, v := range module.Variables {
		s.values[v.Name] = 1
	}
	for _, c := range module.Constants {
		s.values[c.Name] = 0
	}
	s.addDefinitions(module.Definitions)
	return s
}

func (s *metadataScope) clone() *metadataScope {
	return &metadataScope{values: maps.Clone(s.values), defs: maps.Clone(s.defs), tlc: s.tlc, instanceProbe: s.instanceProbe}
}

func (s *metadataScope) addDefinitions(defs []parser.Definition) {
	for _, d := range defs {
		s.defs[d.Name] = &metadataDefinition{value: d, scope: s}
	}
}

func (s *metadataScope) bind(name string, level int) {
	s.values[name] = level
	delete(s.defs, name)
}

func (s *metadataScope) definition(d *metadataDefinition, args []int, visiting map[*metadataDefinition]bool) (int, error) {
	if d == nil {
		return 0, fmt.Errorf("unknown definition")
	}
	if visiting[d] {
		return 0, fmt.Errorf("recursive operator levels are not supported")
	}
	visiting[d] = true
	defer delete(visiting, d)
	local := d.scope.clone()
	if err := local.bindDefinitionParams(d.value.Params, args); err != nil {
		return 0, err
	}
	level, err := local.bounds(d.value.Bounds, visiting)
	if err != nil {
		return 0, err
	}
	body, err := local.expression(d.value.Body, visiting)
	if err == nil && d.value.Kind == "Function" {
		err = local.checkInstanceLevel(body, 3)
	}
	return max(level, body), err
}

func (s *metadataScope) bindDefinitionParams(params []parser.Declaration, args []int) error {
	for i, param := range params {
		if param.Arity != 0 && args != nil {
			return fmt.Errorf("higher-order operator calls are not supported")
		}
		level := 0
		if i < len(args) {
			level = args[i]
		}
		s.bind(param.Name, level)
	}
	return nil
}

func (s *metadataScope) bounds(bounds []parser.Bound, visiting map[*metadataDefinition]bool) (int, error) {
	level := 0
	for _, bound := range bounds {
		boundLevel, err := s.boundLevel(bound, visiting)
		if err != nil {
			return 0, err
		}
		level = max(level, boundLevel)
		for _, name := range bound.Names {
			s.bind(name, 0)
		}
	}
	return level, nil
}

func (s *metadataScope) boundLevel(bound parser.Bound, visiting map[*metadataDefinition]bool) (int, error) {
	if bound.Set == nil {
		return 0, nil
	}
	return s.expression(bound.Set, visiting)
}
