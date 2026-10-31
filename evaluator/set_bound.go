package evaluator

import "fmt"

// DefaultSetBound is Java TLC's default -maxSetSize.
const DefaultSetBound = 1000000

// javaValueVecCapacity is the initial capacity of Java TLC's ValueVec, which
// checks the bound only when it grows.
const javaValueVecCapacity = 10

// SetBound limits sets that TLC builds element by element: {e : x \in S} and
// {x \in S : P} over an enumerated set or interval. A nonpositive bound
// selects DefaultSetBound.
func (p *Program) SetBound(bound int) {
	p.setBound = bound
}

// setLimit returns the largest number of elements Java TLC's ValueVec accepts
// before its growth check fails.
func (p *Program) setLimit() int {
	bound := p.setBound
	if bound <= 0 {
		bound = DefaultSetBound
	}
	return max(bound, javaValueVecCapacity)
}

func (p *Program) checkSetSize(count int) error {
	if count <= p.setLimit() {
		return nil
	}
	bound := p.setBound
	if bound <= 0 {
		bound = DefaultSetBound
	}
	message := fmt.Sprintf("Attempted to construct a set with too many elements (>%d).", bound)
	return &RuntimeError{Message: message, WrongInvocation: true}
}
