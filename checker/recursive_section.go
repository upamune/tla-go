package checker

import (
	"fmt"
	"slices"

	"github.com/upamune/tla-go/syntax/parser"
)

type recursiveSectionUnit struct {
	position parser.Position
	kind     string
}

type recursiveSectionInterval struct {
	start, end parser.Position
	hasEnd     bool
}

// Java SANY's Generator.checkIfInRecursiveSection rejects these module units
// while any declaration in the enclosing module remains unresolved. INSTANCE
// is intentionally allowed. Validate before module linking changes source order.
func validateRecursiveSection(module *parser.Module) error {
	if len(module.RecursiveDeclarations) == 0 {
		return nil
	}
	units := recursiveSectionUnits(module)
	intervals := recursiveSectionIntervals(module)
	slices.SortFunc(units, func(a, b recursiveSectionUnit) int {
		if a.position.Line != b.position.Line {
			return a.position.Line - b.position.Line
		}
		return a.position.Column - b.position.Column
	})
	for _, unit := range units {
		if recursiveSectionOpen(unit.position, intervals) {
			return fmt.Errorf("%s may not appear within a recursive definition section", unit.kind)
		}
	}
	return nil
}

func recursiveSectionUnits(module *parser.Module) []recursiveSectionUnit {
	var units []recursiveSectionUnit
	for _, d := range module.Constants {
		units = append(units, recursiveSectionUnit{d.Location.Position, "A declaration"})
	}
	for _, d := range module.Variables {
		units = append(units, recursiveSectionUnit{d.Location.Position, "A VARIABLE declaration"})
	}
	for _, a := range module.Assumptions {
		units = append(units, recursiveSectionUnit{a.Location.Position, "An ASSUME"})
	}
	for _, t := range module.Theorems {
		units = append(units, recursiveSectionUnit{t.Location.Position, "A THEOREM"})
	}
	for _, d := range module.ProofDirectives {
		units = append(units, recursiveSectionUnit{d.Location.Position, "A USE or HIDE"})
	}
	for _, child := range module.Modules {
		units = append(units, recursiveSectionUnit{child.Location.Position, "A MODULE"})
	}
	return units
}

func recursiveSectionIntervals(module *parser.Module) []recursiveSectionInterval {
	var intervals []recursiveSectionInterval
	for _, decl := range module.RecursiveDeclarations {
		if decl.Location.Line == 0 {
			continue
		}
		definition := recursiveDefinition(decl, module.Definitions)
		if definition != nil && definition.Location.Line == 0 {
			continue
		}
		interval := recursiveSectionInterval{start: decl.Location.Position}
		if definition != nil {
			interval.end, interval.hasEnd = definition.Location.Position, true
		}
		intervals = append(intervals, interval)
	}
	return intervals
}

func recursiveSectionOpen(position parser.Position, intervals []recursiveSectionInterval) bool {
	if position.Line == 0 {
		return false
	}
	for _, interval := range intervals {
		if !positionAfter(position, interval.start) {
			continue
		}
		if !interval.hasEnd || !positionAfter(position, interval.end) {
			return true
		}
	}
	return false
}
