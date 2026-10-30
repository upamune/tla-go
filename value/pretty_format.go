// Formatting follows tla-ts's pretty-print.ts; see UPSTREAM-LICENSE.
package value

import (
	"strings"
)

type printedDelimiters struct {
	open, close, pad, separator, divider, dividerPad string
}

var printedStyles = map[string]printedDelimiters{
	"Set":      {"{ ", " }", "  ", ",", "", ""},
	"Sequence": {"<< ", " >>", "   ", ",", "", ""},
	"Subset":   {"SUBSET ", " ", " ", ",", "", ""},
	"Function": {"( ", " )", "  ", " @@", " :>", "    "},
	"Record":   {"[ ", " ]", "  ", ",", " |->", "    "},
}

// PrettyPrint applies TLC's 80-column layout to a rendered value. If the
// rendered text cannot be parsed as a TLC value, it is returned unchanged.
func PrettyPrint(text string) string {
	if len(text) <= 80 {
		return text
	}
	p := printedParser{text: []rune(text)}
	node := p.parse(0)
	if node == nil || node.end != len(p.text) {
		return text
	}
	result, ok := p.format(node, 80, 0, "")
	if !ok {
		return text
	}
	return result
}

func (p *printedParser) nodeText(node *printedNode) string {
	return string(p.text[node.first:node.end])
}

func (p *printedParser) nodeWidth(node *printedNode) int {
	width := 0
	for _, r := range p.text[node.first:node.end] {
		width++
		if r > 0xffff {
			width++
		}
	}
	return width
}

func (p *printedParser) format(node *printedNode, width, trailer int, pad string) (string, bool) {
	if p.nodeWidth(node) <= width-trailer || node.kind == "Constant" {
		return p.nodeText(node), true
	}
	style, ok := printedStyles[node.kind]
	if !ok {
		return "", false
	}
	innerWidth := width - max(len(style.open), len(style.pad))
	innerTrailer := trailer + len(style.close)
	innerPad := pad + style.pad
	var inner string
	if node.kind == "Function" || node.kind == "Record" {
		inner, ok = p.formatPairs(node.children, innerWidth, innerTrailer, innerPad, style)
	} else {
		inner, ok = p.formatItems(node.children, innerWidth, innerTrailer, innerPad, style.separator)
	}
	return style.open + inner + style.close, ok
}

func (p *printedParser) formatItems(items []*printedNode, width, trailer int, pad, separator string) (string, bool) {
	var result strings.Builder
	for i, item := range items {
		last := i == len(items)-1
		trailing := len(separator)
		if last {
			trailing = trailer
		}
		text, ok := p.format(item, width, trailing, pad)
		if !ok {
			return "", false
		}
		result.WriteString(text)
		if !last {
			result.WriteString(separator + "\n" + pad)
		}
	}
	return result.String(), true
}

func (p *printedParser) formatPairs(pairs []*printedNode, width, trailer int, pad string, style printedDelimiters) (string, bool) {
	var result strings.Builder
	for i, pair := range pairs {
		last := i == len(pairs)-1
		text, ok := p.formatPair(pair, width, trailer, pad, style, last)
		if !ok {
			return "", false
		}
		result.WriteString(text)
		if !last {
			result.WriteString(style.separator + "\n" + pad)
		}
	}
	return result.String(), true
}

func (p *printedParser) formatPair(pair *printedNode, width, trailer int, pad string, style printedDelimiters, last bool) (string, bool) {
	left, right := pair.children[0], pair.children[1]
	length := p.nodeWidth(left) + len(style.divider) + 1 + p.nodeWidth(right)
	available := width - len(style.separator)
	if last {
		available = width - trailer
	}
	if length <= available {
		return p.nodeText(left) + style.divider + " " + p.nodeText(right), true
	}
	return p.formatSplitPair(left, right, width, trailer, pad, style, last)
}

func (p *printedParser) formatSplitPair(left, right *printedNode, width, trailer int, pad string, style printedDelimiters, last bool) (string, bool) {
	leftText := p.nodeText(left)
	if p.nodeWidth(left)+len(style.divider) > width {
		var ok bool
		leftText, ok = p.format(left, width, trailer+len(style.divider), pad)
		if !ok {
			return "", false
		}
	}
	trailing := len(style.separator)
	if last {
		trailing = trailer
	}
	rightText, ok := p.format(right, width-len(style.dividerPad), trailing, pad+style.dividerPad)
	if !ok {
		return "", false
	}
	return leftText + style.divider + "\n" + pad + style.dividerPad + rightText, true
}
