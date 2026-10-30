package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/upamune/tla-go/syntax"
)

func canonicalProofLabel(label string) string { return strings.TrimRight(label, ".") }

func proofStepLevel(label string) (int, error) {
	end := strings.IndexByte(label, '>')
	if end < 2 || label[0] != '<' {
		return 0, fmt.Errorf("invalid proof step %q", label)
	}
	level, err := strconv.Atoi(label[1:end])
	if err != nil || level < 1 {
		return 0, fmt.Errorf("unsupported proof step level %q", label)
	}
	name := canonicalProofLabel(label[end+1:])
	if name == "*" || name == "-" {
		return 0, fmt.Errorf("automatic proof step names are not supported")
	}
	return level, nil
}

func (p *parser) structuredProof(start int, token syntax.Token) (*Proof, error) {
	level, err := proofStepLevel(p.peek().Text)
	if err != nil {
		return nil, p.error(err.Error())
	}
	proof := &Proof{Kind: "Structured"}
	for {
		step, err := p.structuredStep(level)
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

func (p *parser) structuredStep(level int) (ProofStep, error) {
	start := p.pos
	token, err := p.proofStepToken(level)
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
	if step.Kind == "Define" && step.Proof != nil {
		return ProofStep{}, p.error("DEFINE cannot have a proof")
	}
	step.Location = p.location(token, start)
	return step, nil
}

func (p *parser) proofStepToken(level int) (syntax.Token, error) {
	token := p.peek()
	if token.Kind != "Step" {
		return token, p.error("structured proof requires QED")
	}
	actual, err := proofStepLevel(token.Text)
	if err != nil {
		return token, p.error(err.Error())
	}
	if actual != level {
		return token, p.error("unexpected proof step level")
	}
	p.next()
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
		child, err := proofStepLevel(p.look(offset).Text)
		if err != nil {
			return nil, p.error(err.Error())
		}
		if child <= level {
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
