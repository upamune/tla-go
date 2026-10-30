package checker

import (
	"slices"
	"unicode/utf16"

	"github.com/upamune/tla-go/syntax/parser"
)

type orderEntry struct {
	name     string
	variable bool
}
type orderTable struct {
	buckets          [][]orderEntry
	names            map[string]bool
	count, threshold int
}

func (m *model) symmetryVariableOrder() []string {
	t := orderTable{buckets: make([][]orderEntry, 11), names: map[string]bool{}, threshold: 8}
	t.includeStandardModules(m.module.Extends)
	for _, entry := range moduleOrderEntries(m.module) {
		t.add(entry.entry)
	}
	result := t.variableNames()
	for _, name := range m.names {
		if !slices.Contains(result, name) {
			result = append(result, name)
		}
	}
	return result
}

func (t *orderTable) includeStandardModules(extended []string) {
	for _, name := range slices.Backward(tlcBuiltinNames) {
		t.add(orderEntry{name: name})
	}
	for _, module := range extended {
		for _, name := range tlcStandardNames[module] {
			t.add(orderEntry{name: name})
		}
	}
}

func (t *orderTable) variableNames() []string {
	var result []string
	for _, bucket := range t.buckets {
		for _, entry := range bucket {
			if entry.variable {
				result = append(result, entry.name)
			}
		}
	}
	return result
}

type locatedOrder struct {
	entry orderEntry
	pos   parser.Position
}

func moduleOrderEntries(module *parser.Module) []locatedOrder {
	var result []locatedOrder
	for _, v := range module.Variables {
		result = append(result, locatedOrder{orderEntry{v.Name, true}, v.Location.Position})
	}
	for _, c := range module.Constants {
		result = append(result, locatedOrder{orderEntry{c.Name, false}, c.Location.Position})
	}
	result = append(result, namedStatements(module)...)
	result = append(result, orderDefinitions(module.Definitions)...)
	slices.SortStableFunc(result, func(a, b locatedOrder) int {
		if a.pos.Line != b.pos.Line {
			return a.pos.Line - b.pos.Line
		}
		return a.pos.Column - b.pos.Column
	})
	return result
}

func namedStatements(module *parser.Module) []locatedOrder {
	var result []locatedOrder
	for _, a := range module.Assumptions {
		if a.Name != "" {
			result = append(result, locatedOrder{orderEntry{a.Name, false}, a.Location.Position})
		}
	}
	for _, thm := range module.Theorems {
		if thm.Name != "" {
			result = append(result, locatedOrder{orderEntry{thm.Name, false}, thm.Location.Position})
		}
	}
	return result
}

func orderDefinitions(defs []parser.Definition) []locatedOrder {
	var result []locatedOrder
	for _, d := range defs {
		if d.Location.Position.Line == 0 || len(d.Name) > 0 && d.Name[0] == '!' {
			continue
		}
		result = append(result, locatedOrder{orderEntry{d.Name, false}, d.Location.Position})
	}
	return result
}

func (t *orderTable) add(entry orderEntry) {
	if t.names[entry.name] {
		return
	}
	if t.count >= t.threshold {
		t.rehash()
	}
	bucket := javaBucket(entry.name, len(t.buckets))
	t.buckets[bucket] = append(t.buckets[bucket], entry)
	t.names[entry.name] = true
	t.count++
}

func (t *orderTable) rehash() {
	buckets := make([][]orderEntry, len(t.buckets)*2+1)
	for _, v := range slices.Backward(t.buckets) {
		for _, entry := range slices.Backward(v) {
			bucket := javaBucket(entry.name, len(buckets))
			buckets[bucket] = append(buckets[bucket], entry)
		}
	}
	t.buckets = buckets
	t.threshold = len(buckets) * 3 / 4
}

func javaBucket(name string, size int) int {
	var hash uint32
	for _, ch := range utf16.Encode([]rune(name)) {
		hash = hash*31 + uint32(ch)
	}
	return int(hash&0x7fffffff) % size
}
