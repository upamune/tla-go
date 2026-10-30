// Package config parses TLC model configuration files.
// Its behavior is based on tla-ts (see UPSTREAM-LICENSE).
package config

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/upamune/tla-go/syntax"
)

// Value is a TLC configuration value. Set elements may themselves be sets.
type Value struct {
	Kind     string
	Text     string
	Elements []Value
}
type Assignment struct {
	Name   string
	Value  Value
	Args   []Value
	Module string
}
type (
	Substitution struct{ Name, Target, Module string }
	Config       struct {
		Init, Next, Specification, Symmetry, View, Alias, Periodic, RLReward             string
		Invariants, Properties, Constraints, ActionConstraints, Postconditions, Possible []string
		Constants                                                                        []Assignment
		Substitutions                                                                    []Substitution
		CheckDeadlock                                                                    bool
	}
)

type token struct {
	kind, text string
	line       int
}
type parser struct {
	tokens []token
	pos    int
}

var keywords = map[string]bool{}

func init() {
	for k := range strings.FieldsSeq("CONSTANT CONSTANTS INIT NEXT SPECIFICATION INVARIANT INVARIANTS PROPERTY PROPERTIES CONSTRAINT CONSTRAINTS ACTION_CONSTRAINT ACTION_CONSTRAINTS SYMMETRY VIEW CHECK_DEADLOCK ALIAS POSTCONDITION POSTCONDITIONS _POSSIBLE _PERIODIC _RL_REWARD") {
		keywords[k] = true
	}
}

func letter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func digit(c byte) bool  { return c >= '0' && c <= '9' }
func word(c byte) bool   { return letter(c) || digit(c) || c == '_' || c == '!' }

type scanner struct {
	source    string
	pos, line int
	tokens    []token
}

func (s *scanner) add(kind, text string, length int) {
	s.tokens = append(s.tokens, token{kind, text, s.line})
	s.pos += length
}
func (s *scanner) rest() string { return s.source[s.pos:] }
func (s *scanner) whitespace() bool {
	if !strings.ContainsRune(" \t\r\n\f", rune(s.source[s.pos])) {
		return false
	}
	if s.source[s.pos] == '\n' {
		s.line++
	}
	s.pos++
	return true
}

func (s *scanner) comment() (bool, error) {
	if strings.HasPrefix(s.rest(), "\\*") {
		end := strings.IndexByte(s.rest(), '\n')
		if end < 0 {
			s.pos = len(s.source)
		} else {
			s.pos += end
		}
		return true, nil
	}
	if !strings.HasPrefix(s.rest(), "(*") {
		return false, nil
	}
	end := strings.Index(s.rest()[2:], "*)")
	if end < 0 {
		return true, fmt.Errorf("Unterminated comment at line %d", s.line)
	}
	length := end + 4
	s.line += strings.Count(s.rest()[:length], "\n")
	s.pos += length
	return true, nil
}

func (s *scanner) scanNumber() bool {
	if !digit(s.source[s.pos]) {
		return false
	}
	end := s.pos
	for end < len(s.source) && digit(s.source[end]) {
		end++
	}
	if end < len(s.source) && (letter(s.source[end]) || s.source[end] == '_') {
		end--
		if end == s.pos {
			return false
		}
	}
	s.add("Number", s.source[s.pos:end], end-s.pos)
	return true
}

func (s *scanner) scanWord() bool {
	if !word(s.source[s.pos]) {
		return false
	}
	end := s.pos
	for end < len(s.source) && word(s.source[end]) {
		end++
	}
	s.add("Word", s.source[s.pos:end], end-s.pos)
	return true
}

func (s *scanner) scanString() error {
	start := s.pos + 1
	end := start
	for end < len(s.source) && s.source[end] != '"' {
		if s.source[end] == '\\' && end+1 < len(s.source) {
			end += 2
		} else {
			end++
		}
	}
	if end >= len(s.source) {
		return fmt.Errorf("Unexpected character %q at line %d", '"', s.line)
	}
	s.add("String", s.source[start:end], end+1-s.pos)
	return nil
}

var configSymbols = []string{"<-", "<<", ">>", "=", "{", "}", ",", "[", "]", "(", ")"}

func (s *scanner) scanSymbol() bool {
	for _, symbol := range configSymbols {
		if strings.HasPrefix(s.rest(), symbol) {
			s.add("Symbol", symbol, len(symbol))
			return true
		}
	}
	return false
}

func (s *scanner) scanOperator() bool {
	if s.scanSymbol() {
		return true
	}
	text, n := syntax.MatchOperator(s.rest())
	if n == 0 {
		return false
	}
	s.add("Word", text, n)
	return true
}

func (s *scanner) scan() error {
	if s.whitespace() {
		return nil
	}
	if ok, err := s.comment(); ok {
		return err
	}
	if s.scanNumber() || s.scanWord() {
		return nil
	}
	if s.source[s.pos] == '"' {
		return s.scanString()
	}
	if s.scanOperator() {
		return nil
	}
	r, _ := utf8.DecodeRuneInString(s.rest())
	return fmt.Errorf("Unexpected character %q at line %d", r, s.line)
}

func tokenize(source string) ([]token, error) {
	s := scanner{source: source, line: 1}
	for s.pos < len(source) {
		if err := s.scan(); err != nil {
			return nil, err
		}
	}
	return append(s.tokens, token{"End", "", s.line}), nil
}
func (p *parser) peek() token { return p.tokens[p.pos] }
func (p *parser) next() token {
	t := p.peek()
	if p.pos < len(p.tokens)-1 {
		p.pos++
	}
	return t
}
func (p *parser) nameAhead() bool { t := p.peek(); return t.kind == "Word" && !keywords[t.text] }
func (p *parser) name() (string, error) {
	t := p.next()
	if t.kind != "Word" || keywords[t.text] {
		return "", fmt.Errorf("Expected a name but found %q at line %d", t.text, t.line)
	}
	return t.text, nil
}

func (p *parser) expect(s string) error {
	t := p.next()
	if t.text != s {
		return fmt.Errorf("Expected %q but found %q at line %d", s, t.text, t.line)
	}
	return nil
}

func (p *parser) set() (Value, error) {
	v := Value{Kind: "Set"}
	if p.peek().text == "}" {
		p.next()
		return v, nil
	}
	for {
		e, err := p.value()
		if err != nil {
			return Value{}, err
		}
		v.Elements = append(v.Elements, e)
		if p.peek().text != "," {
			break
		}
		p.next()
	}
	return v, p.expect("}")
}

func (p *parser) value() (Value, error) {
	t := p.next()
	if t.text == "{" {
		return p.set()
	}
	if t.text == "TRUE" || t.text == "FALSE" {
		return Value{Kind: "Boolean", Text: t.text}, nil
	}
	if t.kind == "Number" || t.kind == "String" {
		return Value{Kind: t.kind, Text: t.text}, nil
	}
	if t.kind == "Word" && !keywords[t.text] {
		return Value{Kind: "ModelValue", Text: t.text}, nil
	}
	return Value{}, fmt.Errorf("Expected a value but found %q at line %d", t.text, t.line)
}

func (p *parser) scope() (string, error) {
	if p.peek().text != "[" {
		return "", nil
	}
	p.next()
	name, err := p.name()
	if err != nil {
		return "", err
	}
	return name, p.expect("]")
}

func (p *parser) arguments() ([]Value, error) {
	if p.peek().text != "(" {
		return nil, nil
	}
	p.next()
	args := []Value{}
	for p.peek().text != ")" {
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		args = append(args, v)
		if p.peek().text != ")" {
			if err := p.expect(","); err != nil {
				return nil, err
			}
		}
	}
	p.next()
	return args, nil
}

func (p *parser) substitution(c *Config, name string) error {
	p.next()
	module, err := p.scope()
	if err != nil {
		return err
	}
	target, err := p.name()
	if err != nil {
		return err
	}
	c.Substitutions = append(c.Substitutions, Substitution{name, target, module})
	return nil
}

func (p *parser) assignment(c *Config, name string, args []Value) error {
	if err := p.expect("="); err != nil {
		return err
	}
	module, err := p.scope()
	if err != nil {
		return err
	}
	v, err := p.value()
	if err != nil {
		return err
	}
	c.Constants = append(c.Constants, Assignment{name, v, args, module})
	return nil
}

func (p *parser) constants(c *Config) error {
	for p.nameAhead() {
		name := p.next().text
		args, err := p.arguments()
		if err != nil {
			return err
		}
		if p.peek().text == "<-" && args == nil {
			err = p.substitution(c, name)
		} else {
			err = p.assignment(c, name, args)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Parse parses a TLC configuration. Duplicate singleton directives and mixed
// SPECIFICATION/INIT/NEXT directives are rejected as in tla-ts.
func Parse(source string) (Config, error) {
	ts, err := tokenize(source)
	if err != nil {
		return Config{}, err
	}
	p := parser{tokens: ts}
	c := Config{CheckDeadlock: true}
	if err = p.directives(&c); err != nil {
		return Config{}, err
	}
	if c.Specification != "" && (c.Init != "" || c.Next != "") {
		return Config{}, fmt.Errorf("SPECIFICATION cannot be combined with INIT or NEXT")
	}
	return c, nil
}

func (p *parser) directives(c *Config) error {
	singles := map[string]*string{"INIT": &c.Init, "NEXT": &c.Next, "SPECIFICATION": &c.Specification, "SYMMETRY": &c.Symmetry, "VIEW": &c.View, "ALIAS": &c.Alias, "_PERIODIC": &c.Periodic, "_RL_REWARD": &c.RLReward}
	lists := map[string]*[]string{"INVARIANT": &c.Invariants, "INVARIANTS": &c.Invariants, "PROPERTY": &c.Properties, "PROPERTIES": &c.Properties, "CONSTRAINT": &c.Constraints, "CONSTRAINTS": &c.Constraints, "ACTION_CONSTRAINT": &c.ActionConstraints, "ACTION_CONSTRAINTS": &c.ActionConstraints, "POSTCONDITION": &c.Postconditions, "POSTCONDITIONS": &c.Postconditions, "_POSSIBLE": &c.Possible}
	seen := map[string]bool{}
	for p.peek().kind != "End" {
		if err := p.directive(c, singles, lists, seen); err != nil {
			return err
		}
	}
	return nil
}

func (p *parser) deadlock(c *Config) error {
	v := p.next().text
	if v != "TRUE" && v != "FALSE" {
		return fmt.Errorf("CHECK_DEADLOCK expects TRUE or FALSE but found %q", v)
	}
	c.CheckDeadlock = v == "TRUE"
	return nil
}

func (p *parser) singleton(c *Config, t token, singles map[string]*string, seen map[string]bool) error {
	if _, ok := singles[t.text]; !ok && t.text != "CHECK_DEADLOCK" {
		return fmt.Errorf("Unexpected %q at line %d", t.text, t.line)
	}
	if seen[t.text] {
		return fmt.Errorf("The keyword %s appears twice", t.text)
	}
	seen[t.text] = true
	if t.text == "CHECK_DEADLOCK" {
		return p.deadlock(c)
	}
	name, err := p.name()
	if err == nil {
		*singles[t.text] = name
	}
	return err
}

func (p *parser) directive(c *Config, singles map[string]*string, lists map[string]*[]string, seen map[string]bool) error {
	t := p.next()
	if t.text == "CONSTANT" || t.text == "CONSTANTS" {
		return p.constants(c)
	}
	if list, ok := lists[t.text]; ok {
		for p.nameAhead() {
			*list = append(*list, p.next().text)
		}
		return nil
	}
	return p.singleton(c, t, singles, seen)
}
