package pcal

import "strings"

// changed ports Changed, which counts the assignments to each variable in a
// step.
type changed struct {
	count []int
	vars  []string
}

func newChanged(vars []string) *changed {
	return &changed{count: make([]int, len(vars)), vars: vars}
}

func (c *changed) clone() *changed {
	return &changed{count: append([]int(nil), c.count[:len(c.count)]...), vars: c.vars}
}

func (c *changed) isChanged(s string) bool {
	for i, v := range c.vars[:len(c.count)] {
		if v == s {
			return c.count[i] > 0
		}
	}
	return false
}

func (c *changed) merge(o *changed) {
	assertBug(len(c.count) == len(o.count), "Assertion failure")
	for i := range c.count {
		c.count[i] = max(c.count[i], o.count[i])
	}
}

func (c *changed) set(v string) int {
	for i, x := range c.vars[:len(c.count)] {
		if x == v {
			c.count[i]++
			return c.count[i]
		}
	}
	return 0
}

// unchangedVars lists the variables not changed here; with o, only those
// that o changes.
func (c *changed) unchangedVars(o *changed) []string {
	var vs []string
	for i := range c.count {
		if c.count[i] == 0 && (o == nil || o.count[i] > 0) {
			vs = append(vs, c.vars[i])
		}
	}
	return vs
}

func (c *changed) unchanged(o *changed) string {
	return strings.Join(c.unchangedVars(o), ", ")
}

func (c *changed) numUnchanged(o *changed) int { return len(c.unchangedVars(o)) }

// unchangedLines splits the unchanged variables into lines of about ch
// characters. As in Java, a line can end with ", ".
func (c *changed) unchangedLines(o *changed, ch int) []string {
	var lines []string
	s := ""
	for i, one := range c.unchangedVars(o) {
		if i > 0 {
			s += ", "
		}
		if len(s)+len(one) > ch {
			if s != "" {
				lines = append(lines, s)
			}
			s = one
		} else {
			s += one
		}
	}
	if s != "" {
		lines = append(lines, s)
	}
	return lines
}
