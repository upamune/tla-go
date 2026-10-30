package parser

import (
	"fmt"
	"strconv"
)

func (p *parser) currentProofLevel() int {
	if len(p.proofLevels) == 0 {
		return -1
	}
	return p.proofLevels[len(p.proofLevels)-1]
}

func (p *parser) firstProofLevel(label string) (int, error) {
	kind, name, err := proofStepParts(label)
	if err != nil {
		return 0, err
	}
	parent := p.currentProofLevel()
	if kind == "*" || kind == "+" {
		if err := checkRelativeStepName(name); err != nil {
			return 0, err
		}
		return parent + 1, nil
	}
	level, err := numericProofLevel(kind)
	if err != nil {
		return 0, err
	}
	if level <= parent {
		return 0, fmt.Errorf("proof step level must exceed its parent")
	}
	return level, nil
}

func proofSiblingLevel(label string, current int) (int, error) {
	kind, name, err := proofStepParts(label)
	if err != nil {
		return 0, err
	}
	if kind == "*" {
		if err := checkRelativeStepName(name); err != nil {
			return 0, err
		}
		return current, nil
	}
	if kind == "+" {
		return 0, fmt.Errorf("relative plus starts a new proof")
	}
	return numericProofLevel(kind)
}

func proofCurrentLevel(label string, current int, first bool) (int, error) {
	kind, name, err := proofStepParts(label)
	if err != nil {
		return 0, err
	}
	if first && (kind == "+" || kind == "*") {
		return current, checkRelativeStepName(name)
	}
	return proofSiblingLevel(label, current)
}

func checkRelativeStepName(name string) error {
	if name != "" && name != "*" && name != "-" {
		return fmt.Errorf("relative proof step cannot be named")
	}
	return nil
}

func numericProofLevel(kind string) (int, error) {
	level, err := strconv.Atoi(kind)
	if err != nil || level < 0 {
		return 0, fmt.Errorf("invalid proof step level %q", kind)
	}
	return level, nil
}

func proofHeaderLabel(label string, level int) string {
	_, name, _ := proofStepParts(label)
	if name == "*" || name == "-" {
		name = ""
	}
	return fmt.Sprintf("<%d>%s", level, name)
}

func beginsNestedProof(label string, current int, explicit bool) (bool, error) {
	kind, _, err := proofStepParts(label)
	if err != nil {
		return false, err
	}
	switch kind {
	case "+":
		return true, nil
	case "*":
		return explicit, nil
	default:
		level, err := numericProofLevel(kind)
		return level > current, err
	}
}

func (p *parser) proofStepReference(label string) (string, error) {
	kind, name, err := proofStepParts(label)
	if err != nil {
		return "", err
	}
	if name == "" || name == "*" || name == "-" {
		return "", fmt.Errorf("unnumbered proof step cannot be referenced")
	}
	return p.proofReferenceLevel(kind, name, label)
}

func (p *parser) proofReferenceLevel(kind, name, label string) (string, error) {
	if kind == "+" {
		return "", fmt.Errorf("relative plus cannot be used in a proof reference")
	}
	if kind == "*" {
		level := p.currentProofLevel()
		if level < 0 {
			return "", fmt.Errorf("relative proof reference outside a proof")
		}
		return fmt.Sprintf("<%d>%s", level, name), nil
	}
	if _, err := numericProofLevel(kind); err != nil {
		return "", err
	}
	return canonicalProofLabel(label), nil
}
