package parser

import (
	"fmt"
	"strings"

	"github.com/upamune/tla-go/syntax"
)

func canonicalProofLabel(label string) string { return strings.TrimRight(label, ".") }

func proofStepParts(label string) (string, string, error) {
	end := strings.IndexByte(label, '>')
	if end < 2 || label[0] != '<' {
		return "", "", fmt.Errorf("invalid proof step %q", label)
	}
	return label[1:end], canonicalProofLabel(label[end+1:]), nil
}

func (p *parser) structuredProof(start int, token syntax.Token) (*Proof, error) {
	level, err := p.firstProofLevel(p.peek().Text)
	if err != nil {
		return nil, p.error(err.Error())
	}
	proof := &Proof{Kind: "Structured"}
	p.proofLevels = append(p.proofLevels, level)
	defer func() { p.proofLevels = p.proofLevels[:len(p.proofLevels)-1] }()
	for {
		step, err := p.structuredStep(level, len(proof.Steps) == 0)
		if err != nil {
			return nil, err
		}
		proof.Steps = append(proof.Steps, step)
		if step.Kind == "Qed" {
			break
		}
	}
	proof.Location = p.location(token, start)
	return proof, nil
}

func (p *parser) structuredStep(level int, first bool) (ProofStep, error) {
	start := p.pos
	token, err := p.proofStepToken(level, first)
	if err != nil {
		return ProofStep{}, err
	}
	step, err := p.proofStepBody(token)
	if err != nil {
		return ProofStep{}, err
	}
	step.Proof, err = p.proofBelow(level)
	if err != nil {
		return ProofStep{}, err
	}
	if proofStepHasNoProof(step.Kind) && step.Proof != nil {
		return ProofStep{}, p.error("this proof step cannot have a proof")
	}
	step.Location = p.location(token, start)
	return step, nil
}

func proofStepHasNoProof(kind string) bool {
	return kind == "Define" || kind == "Use" || kind == "Hide" || kind == "Take" || kind == "Have" || kind == "Witness"
}

func (p *parser) proofStepToken(level int, first bool) (syntax.Token, error) {
	token := p.peek()
	if token.Kind != "Step" {
		return token, p.error("structured proof requires QED")
	}
	actual, err := proofCurrentLevel(token.Text, level, first)
	if err != nil {
		return token, p.error(err.Error())
	}
	if actual != level {
		return token, p.error("unexpected proof step level")
	}
	p.next()
	token.Text = proofHeaderLabel(token.Text, level)
	return token, nil
}

func (p *parser) proofStepBody(token syntax.Token) (ProofStep, error) {
	step := ProofStep{Label: canonicalProofLabel(token.Text), Kind: "Assert"}
	if p.accept("DEFINE") {
		return p.proofDefineStep(step)
	}
	if p.accept("QED") {
		step.Kind = "Qed"
		return step, nil
	}
	if isProofDirective(p.peek().Text) {
		return p.structuredDirective(step)
	}
	if isProofSpecialStep(p.peek().Text) {
		return p.proofSpecialStep(step)
	}
	if p.accept("SUFFICES") {
		step.Kind = "Suffices"
	}
	statement, err := p.theoremStatement()
	if err != nil {
		return ProofStep{}, err
	}
	step.Statement = statement
	return step, nil
}

func (p *parser) structuredDirective(step ProofStep) (ProofStep, error) {
	directive, err := p.parseProofDirective(true)
	if err != nil {
		return ProofStep{}, err
	}
	step.Kind = "Use"
	if directive.Kind == "HIDE" {
		step.Kind = "Hide"
	}
	step.Directive = &directive
	return step, nil
}

func (p *parser) proofDefineStep(step ProofStep) (ProofStep, error) {
	step.Kind = "Define"
	for {
		definition, err := p.proofDefinition()
		if err != nil {
			return ProofStep{}, err
		}
		step.Definitions = append(step.Definitions, definition)
		if p.proofDefinitionEnd() {
			return step, nil
		}
	}
}

func (p *parser) proofDefinition() (Definition, error) {
	definition, instance, err := p.definition(false)
	if err != nil {
		return Definition{}, err
	}
	if instance != nil {
		return Definition{}, p.error("proof-local instances are not supported")
	}
	return definition, nil
}

func (p *parser) proofDefinitionEnd() bool {
	switch p.peek().Kind {
	case "Step", "End", "ModuleEnd", "Separator":
		return true
	}
	return p.simpleProofKeywordAhead()
}

func (p *parser) proofBelow(level int) (*Proof, error) {
	offset := 0
	if p.at("PROOF") {
		offset = 1
	}
	if p.look(offset).Kind == "Step" {
		child, err := beginsNestedProof(p.look(offset).Text, level, offset != 0)
		if err != nil {
			return nil, p.error(err.Error())
		}
		if !child {
			return nil, nil
		}
		return p.simpleProof()
	}
	if p.simpleProofKeywordAhead() {
		return p.simpleProof()
	}
	return nil, nil
}

func (p *parser) simpleProofKeywordAhead() bool {
	switch p.peek().Text {
	case "BY", "OBVIOUS", "OMITTED", "PROOF":
		return true
	default:
		return false
	}
}
