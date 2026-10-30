// Formatting follows tla-ts's pretty-print.ts; see UPSTREAM-LICENSE.
package value

import (
	"unicode"
)

type printedNode struct {
	kind       string
	first, end int
	children   []*printedNode
}

type printedParser struct{ text []rune }

func (p *printedParser) skip(index int) int {
	for index < len(p.text) && printedSpace(p.text[index]) {
		index++
	}
	return index
}

func printedSpace(r rune) bool {
	return r != '\u2007' && (unicode.IsSpace(r) || r >= 0x1c && r <= 0x20)
}

func (p *printedParser) starts(index int, text string) bool {
	for _, r := range text {
		if index >= len(p.text) || p.text[index] != r {
			return false
		}
		index++
	}
	return true
}

func (p *printedParser) parse(index int) *printedNode {
	first := p.skip(index)
	if first >= len(p.text) {
		return nil
	}
	switch p.text[first] {
	case '{':
		return p.items("Set", first, first+1, '}', ",", "")
	case '<':
		return p.sequence(first)
	case '[':
		return p.items("Record", first, first+1, ']', ",", "|->")
	case '(':
		return p.items("Function", first, first+1, ')', "@@", ":>")
	default:
		return p.atom(first)
	}
}

func (p *printedParser) sequence(first int) *printedNode {
	node := p.items("Sequence", first, first+2, '>', ",", "")
	if node == nil || node.end >= len(p.text) {
		return nil
	}
	node.end++
	return node
}

func (p *printedParser) items(kind string, first, start int, close rune, separator, divider string) *printedNode {
	node := &printedNode{kind: kind, first: first}
	pos := p.skip(start)
	for pos < len(p.text) && p.text[pos] != close {
		item := p.item(pos, divider)
		if item == nil {
			return nil
		}
		node.children = append(node.children, item)
		pos = p.skip(item.end)
		if p.starts(pos, separator) {
			pos = p.skip(pos + len(separator))
		}
	}
	if pos >= len(p.text) {
		return nil
	}
	node.end = pos + 1
	return node
}

func (p *printedParser) item(index int, divider string) *printedNode {
	left := p.parse(index)
	if left == nil || divider == "" {
		return left
	}
	pos := p.skip(left.end)
	if !p.starts(pos, divider) {
		return nil
	}
	right := p.parse(pos + len(divider))
	if right == nil {
		return nil
	}
	return &printedNode{kind: "Pair", first: index, end: right.end, children: []*printedNode{left, right}}
}

func (p *printedParser) atom(first int) *printedNode {
	if p.starts(first, "SUBSET") {
		child := p.parse(first + len("SUBSET"))
		if child == nil {
			return nil
		}
		return &printedNode{kind: "Subset", first: first, end: child.end, children: []*printedNode{child}}
	}
	end := p.constantEnd(first)
	if end <= first {
		return nil
	}
	return &printedNode{kind: "Constant", first: first, end: end}
}

func (p *printedParser) constantEnd(first int) int {
	r := p.text[first]
	switch {
	case r == '"':
		return p.quotedEnd(first)
	case unicode.IsDigit(r):
		return p.numberEnd(first)
	case r == '-':
		return p.digitsEnd(first + 1)
	case unicode.IsLetter(r):
		return p.identifierEnd(first)
	default:
		return first
	}
}

func (p *printedParser) quotedEnd(first int) int {
	for i := first + 1; i < len(p.text); i++ {
		if p.text[i] == '"' {
			return i + 1
		}
	}
	return first
}

func (p *printedParser) identifierEnd(first int) int {
	end := first + 1
	for end < len(p.text) && (unicode.IsLetter(p.text[end]) || unicode.IsDigit(p.text[end])) {
		end++
	}
	return end
}

func (p *printedParser) numberEnd(first int) int {
	end := p.digitsEnd(first)
	if p.starts(end, "..") && end+2 < len(p.text) && unicode.IsDigit(p.text[end+2]) {
		return p.digitsEnd(end + 2)
	}
	return end
}

func (p *printedParser) digitsEnd(start int) int {
	for start < len(p.text) && unicode.IsDigit(p.text[start]) {
		start++
	}
	return start
}
