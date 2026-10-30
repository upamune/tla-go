package checker

import (
	"maps"
	"slices"
	"strings"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (m *model) prepareValueOrdering() {
	m.valueRanks = sourceStringRanks(m.sourceConfig, m.module, m.sourceUnits)
	m.rankedSets = make(map[value.Value][]value.Value)
	m.rankedFunctions = make(map[value.Value][]value.Pair)
}

func (m *model) boundConfigValue(v config.Value) (value.Value, error) {
	if v.Kind != "Set" {
		return configValue(v)
	}
	elements := make([]value.Value, 0, len(v.Elements))
	for _, item := range v.Elements {
		x, err := m.boundConfigValue(item)
		if err != nil {
			return value.Value{}, err
		}
		elements = append(elements, x)
	}
	set, err := value.Set(elements)
	if err != nil {
		return value.Value{}, err
	}
	return set.OrderSet(m.compareOrderedValues)
}

// TLC interns configuration values first, then source strings in parse order.
func sourceStringRanks(cfg config.Config, module *parser.Module, units map[string]*parser.Module) map[string]int {
	ranks := initialStringRanks(cfg)
	if len(module.SourceTokens) > 0 {
		rankModuleSources(module, units, ranks)
		return ranks
	}
	var literals []*parser.Expression
	for _, def := range module.Definitions {
		collectStringLiterals(def.Body, &literals)
	}
	for _, assumption := range module.Assumptions {
		collectStringLiterals(assumption.Expression, &literals)
	}
	slices.SortStableFunc(literals, func(a, b *parser.Expression) int {
		if a.Location.Line != b.Location.Line {
			return a.Location.Line - b.Location.Line
		}
		return a.Location.Column - b.Location.Column
	})
	for _, literal := range literals {
		addStringRank(ranks, quotedSourceString(literal.Text))
	}
	for _, literal := range literals {
		addStringRank(ranks, literal.Text)
	}
	return ranks
}

var startupStringRanks = func() map[string]int {
	ranks := make(map[string]int, len(tlcStartupStrings))
	rankStrings(tlcStartupStrings, ranks)
	return ranks
}()

func initialStringRanks(cfg config.Config) map[string]int {
	if len(cfg.Constants) == 0 {
		return maps.Clone(startupStringRanks)
	}
	ranks := make(map[string]int, len(tlcStartupStrings)+len(cfg.Constants))
	for _, assignment := range cfg.Constants {
		for _, arg := range assignment.Args {
			rankConfigValue(arg, ranks)
		}
		rankConfigValue(assignment.Value, ranks)
	}
	rankStrings(tlcStartupStrings, ranks)
	return ranks
}

func rankParsedTokens(tokens []syntax.Token, ranks map[string]int, root bool) {
	count := 0
	for i, token := range tokens {
		if skipRankToken(tokens, i) {
			continue
		}
		if root && count == 3 {
			rankStrings(sanyNodeNames, ranks)
		}
		count++
		text := token.Text
		if token.Kind == "String" {
			text = quotedSourceString(text)
		}
		addStringRank(ranks, text)
	}
}

func rankStrings(texts []string, ranks map[string]int) {
	for _, text := range texts {
		addStringRank(ranks, text)
	}
}

func rankSemanticTokens(tokens []syntax.Token, ranks map[string]int) {
	for _, token := range tokens {
		if token.Kind == "String" {
			addStringRank(ranks, token.Text)
		}
	}
}

func skipRankToken(tokens []syntax.Token, i int) bool {
	token := tokens[i]
	return token.Kind == "End" || token.Text == "MODULE" && i > 0 && tokens[i-1].Kind == "Separator"
}

func quotedSourceString(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`, "\r", `\r`, "\f", `\f`)
	return `"` + replacer.Replace(s) + `"`
}

func rankConfigValue(v config.Value, ranks map[string]int) {
	if v.Kind == "ModelValue" || v.Kind == "String" {
		addStringRank(ranks, v.Text)
	}
	for _, element := range v.Elements {
		rankConfigValue(element, ranks)
	}
}

func collectStringLiterals(expr *parser.Expression, result *[]*parser.Expression) {
	if expr == nil {
		return
	}
	if expr.Kind == "String" {
		*result = append(*result, expr)
	}
	walkChildren(expr, func(child *parser.Expression) { collectStringLiterals(child, result) })
}

func addStringRank(ranks map[string]int, text string) {
	if _, ok := ranks[text]; !ok {
		ranks[text] = len(ranks)
	}
}

func (m *model) compareOrderedValues(a, b value.Value) (int, error) {
	if a == b {
		return 0, nil
	}
	if a.Kind() == b.Kind() {
		switch a.Kind() {
		case value.ModelKind:
			return m.compareRankedModels(a, b)
		case value.StringKind:
			x, _ := a.Text()
			y, _ := b.Text()
			return m.compareRankedStrings(x, y), nil
		}
	}
	return m.compareRankedComplex(a, b)
}

func (m *model) compareRankedModels(a, b value.Value) (int, error) {
	if _, err := value.Compare(a, b); err != nil {
		return 0, err
	}
	x, _ := a.Text()
	y, _ := b.Text()
	return m.compareRankedStrings(x, y), nil
}

func (m *model) compareRankedComplex(a, b value.Value) (int, error) {
	if a.IsSet() && b.IsSet() && a.IsEnumerable() && b.IsEnumerable() {
		return m.compareRankedSets(a, b)
	}
	if isRankedFunction(a) && isRankedFunction(b) {
		return m.compareRankedFunctions(a, b)
	}
	return value.Compare(a, b)
}

func (m *model) compareRankedStrings(a, b string) int {
	if a == b {
		return 0
	}
	ar, aok := m.valueRanks[a]
	br, bok := m.valueRanks[b]
	if aok && bok {
		return ar - br
	}
	if aok {
		return -1
	}
	if bok {
		return 1
	}
	return strings.Compare(a, b)
}

func isRankedFunction(v value.Value) bool {
	return v.Kind() == value.FunctionKind || v.Kind() == value.LazyFunctionKind
}

func (m *model) compareRankedSets(a, b value.Value) (int, error) {
	left, err := m.rankedSetElements(a)
	if err != nil {
		return 0, err
	}
	right, err := m.rankedSetElements(b)
	if err != nil {
		return 0, err
	}
	if len(left) != len(right) {
		return len(left) - len(right), nil
	}
	return m.compareRankedValues(left, right)
}

func (m *model) rankedSetElements(v value.Value) ([]value.Value, error) {
	if elements, ok := m.rankedSets[v]; ok {
		return elements, nil
	}
	elements, err := v.Elements()
	if err != nil {
		return nil, err
	}
	if err := m.sortRankedValues(elements); err != nil {
		return nil, err
	}
	if len(m.rankedSets) < 8192 {
		m.rankedSets[v] = elements
	}
	return elements, nil
}

func (m *model) compareRankedValues(left, right []value.Value) (int, error) {
	for i := range left {
		c, err := m.compareOrderedValues(left[i], right[i])
		if err != nil || c != 0 {
			return c, err
		}
	}
	return 0, nil
}

func (m *model) sortRankedValues(values []value.Value) error {
	var failure error
	slices.SortStableFunc(values, func(a, b value.Value) int {
		if failure != nil {
			return 0
		}
		var c int
		c, failure = m.compareOrderedValues(a, b)
		return c
	})
	return failure
}

func (m *model) compareRankedFunctions(a, b value.Value) (int, error) {
	left, err := m.rankedFunctionPairs(a)
	if err != nil {
		return 0, err
	}
	right, err := m.rankedFunctionPairs(b)
	if err != nil {
		return 0, err
	}
	if len(left) != len(right) {
		return value.Compare(a, b)
	}
	if c, err := m.compareRankedPairPart(left, right, true); err != nil || c != 0 {
		return c, err
	}
	return m.compareRankedPairPart(left, right, false)
}

func (m *model) rankedFunctionPairs(v value.Value) ([]value.Pair, error) {
	if pairs, ok := m.rankedFunctions[v]; ok {
		return pairs, nil
	}
	pairs, err := v.Pairs()
	if err != nil {
		return nil, err
	}
	if err := m.sortRankedPairs(pairs); err != nil {
		return nil, err
	}
	if len(m.rankedFunctions) < 8192 {
		m.rankedFunctions[v] = pairs
	}
	return pairs, nil
}

func (m *model) sortRankedPairs(pairs []value.Pair) error {
	var failure error
	slices.SortStableFunc(pairs, func(a, b value.Pair) int {
		if failure != nil {
			return 0
		}
		var c int
		c, failure = m.compareOrderedValues(a.Key, b.Key)
		return c
	})
	return failure
}

func (m *model) compareRankedPairPart(a, b []value.Pair, keys bool) (int, error) {
	for i := range a {
		left, right := a[i].Value, b[i].Value
		if keys {
			left, right = a[i].Key, b[i].Key
		}
		c, err := m.compareOrderedValues(left, right)
		if err != nil || c != 0 {
			return c, err
		}
	}
	return 0, nil
}

type modelOrdering struct{ model *model }

func (o modelOrdering) Compare(a, b value.Value) (int, error) {
	return o.model.compareOrderedValues(a, b)
}

func (o modelOrdering) Intern(text string) { addStringRank(o.model.valueRanks, text) }
