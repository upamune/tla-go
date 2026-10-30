package parser

import (
	"fmt"

	"github.com/upamune/tla-go/syntax"
)

func node(kind, text string, loc Location, args ...*Expression) *Expression {
	return &Expression{Kind: kind, Text: text, Location: loc, Args: args}
}

func (p *parser) expression(minimum int) (*Expression, error) {
	start := p.pos
	left, err := p.prefix()
	if err != nil {
		return nil, err
	}
	var previous *operatorState
	for p.peek().Kind == "Operator" {
		var done bool
		left, previous, done, err = p.expressionTail(start, minimum, left, previous)
		if err != nil {
			return nil, err
		}
		if done {
			break
		}
	}
	return left, nil
}

func (p *parser) expressionTail(start, minimum int, left *Expression, previous *operatorState) (*Expression, *operatorState, bool, error) {
	next, handled, err := p.postfix(start, left, minimum)
	if err != nil || handled {
		return next, previous, false, err
	}
	token := p.peek()
	op, ok := infix[token.Text]
	if !ok || op.low < minimum {
		return left, previous, true, nil
	}
	if err = checkConflict(previous, token, op); err != nil {
		return nil, nil, false, p.error(err.Error())
	}
	p.next()
	right, err := p.infixRight(token, op)
	if err != nil {
		return nil, nil, false, err
	}
	return p.combine(start, token, left, right, previous), &operatorState{token.Text, op}, false, nil
}

func (p *parser) infixRight(token syntax.Token, op precedence) (*Expression, error) {
	if op.low > 12 && p.at("-") {
		return nil, p.error(fmt.Sprintf("Precedence conflict between %s and -", token.Text))
	}
	return p.expression(op.high + 1)
}

type operatorState struct {
	text string
	precedence
}

func checkConflict(prev *operatorState, t syntax.Token, op precedence) error {
	if prev == nil {
		return nil
	}
	overlap := !(prev.low > op.high || prev.high < op.low)
	chain := prev.text == t.Text && (op.associative || t.Text == "\\X")
	if overlap && !chain {
		return fmt.Errorf("Precedence conflict between %s and %s", prev.text, t.Text)
	}
	return nil
}

func infixKind(text string) string {
	switch text {
	case "/\\":
		return "And"
	case "\\/":
		return "Or"
	case "\\X":
		return "Product"
	}
	return "Apply"
}

func (p *parser) combine(start int, t syntax.Token, left, right *Expression, prev *operatorState) *Expression {
	kind := infixKind(t.Text)
	args := []*Expression{left, right}
	location := p.location(t, start)
	if kind != "Apply" && left.Kind == kind && prev != nil && prev.text == t.Text {
		args = append(append([]*Expression(nil), left.Args...), right)
		location = left.Location
		location.Range = p.span(start)
	}
	text := t.Text
	if kind != "Apply" {
		text = ""
	}
	return node(kind, text, location, args...)
}

func postfixOperator(text string) bool {
	return text == "'" || text == "^+" || text == "^*" || text == "^#"
}

func (p *parser) postfix(start int, left *Expression, minimum int) (*Expression, bool, error) {
	t := p.peek()
	if minimum <= 15 && postfixOperator(t.Text) {
		p.next()
		kind, text := "Apply", t.Text
		if t.Text == "'" {
			kind, text = "Prime", ""
		}
		return node(kind, text, p.location(t, start), left), true, nil
	}
	if t.Text == "[" {
		return p.functionApplication(start, left)
	}
	if t.Text == "." {
		return p.fieldAccess(start, left)
	}
	return left, false, nil
}

func (p *parser) functionApplication(start int, left *Expression) (*Expression, bool, error) {
	t := p.next()
	args, err := p.delimited("]", false)
	if err != nil {
		return nil, true, err
	}
	return node("FunctionApplication", "", p.location(t, start), append([]*Expression{left}, args...)...), true, nil
}

func (p *parser) fieldAccess(start int, left *Expression) (*Expression, bool, error) {
	t := p.peek()
	saved := p.pos
	p.next()
	field := p.peek()
	if field.Kind != "Identifier" && field.Kind != "Keyword" {
		p.pos = saved
		return left, false, nil
	}
	p.next()
	return node("FieldAccess", field.Text, p.location(t, start), left), true, nil
}

func (p *parser) prefix() (*Expression, error) {
	t := p.peek()
	if t.Kind == "Operator" && p.operatorCallAhead() {
		return p.identifier()
	}
	if t.Kind == "Operator" && (t.Text == "/\\" || t.Text == "\\/") {
		return p.junction()
	}
	return p.prefixOrPrimary(t)
}

func (p *parser) prefixOrPrimary(t syntax.Token) (*Expression, error) {
	if priority, kind, text, ok := prefixInfo(t.Text); ok && (t.Kind == "Operator" || t.Kind == "Keyword") {
		start := p.pos
		p.next()
		expr, err := p.expression(priority)
		if err != nil {
			return nil, err
		}
		return node(kind, text, p.location(t, start), expr), nil
	}
	return p.primary()
}

func (p *parser) operatorCallAhead() bool {
	t := p.peek()
	switch t.Text {
	case "-", "/\\", "\\/", "'":
		return false
	}
	if p.look(1).Text != "(" {
		return false
	}
	_, infixOperator := infix[t.Text]
	return infixOperator || t.Text == "-." || postfixOperator(t.Text)
}

func prefixInfo(text string) (int, string, string, bool) {
	switch text {
	case "~":
		return 5, "Apply", "~", true
	case "-":
		return 13, "Apply", "-.", true
	case "[]":
		return 16, "Always", "", true
	case "<>":
		return 16, "Eventually", "", true
	case "ENABLED":
		return 16, "Enabled", "", true
	case "UNCHANGED":
		return 16, "Unchanged", "", true
	case "SUBSET", "UNION", "DOMAIN":
		return 9, "Apply", text, true
	}
	return 0, "", "", false
}

func (p *parser) junction() (*Expression, error) {
	start := p.pos
	bullet := p.next()
	previous := p.offside
	p.offside = bullet.Column
	defer func() { p.offside = previous }()
	var operands []*Expression
	for {
		expr, err := p.expression(0)
		if err != nil {
			return nil, err
		}
		operands = append(operands, expr)
		t := p.raw()
		if t.Text != bullet.Text || t.Column != bullet.Column {
			break
		}
		p.pos++
	}
	kind := "And"
	if bullet.Text == "\\/" {
		kind = "Or"
	}
	result := node(kind, "", p.location(bullet, start), operands...)
	result.Bulleted = true
	return result, nil
}
