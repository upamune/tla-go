package checker

import (
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

func TestRecursiveProofExpressionFields(t *testing.T) {
	cases := map[string]func(*parser.Expression) parser.Module{
		"theorem statement": func(e *parser.Expression) parser.Module {
			return parser.Module{Theorems: []parser.Theorem{{Statement: e}}}
		},
		"proof fact": func(e *parser.Expression) parser.Module {
			return proofModule(parser.Proof{Facts: []*parser.Expression{e}})
		},
		"proof def": func(e *parser.Expression) parser.Module {
			return proofModule(parser.Proof{Defs: []*parser.Expression{e}})
		},
		"step statement": func(e *parser.Expression) parser.Module { return stepModule(parser.ProofStep{Statement: e}) },
		"step bound": func(e *parser.Expression) parser.Module {
			return stepModule(parser.ProofStep{Bounds: []parser.Bound{{Set: e}}})
		},
		"step argument": func(e *parser.Expression) parser.Module {
			return stepModule(parser.ProofStep{Arguments: []*parser.Expression{e}})
		},
		"step definition": func(e *parser.Expression) parser.Module {
			return stepModule(parser.ProofStep{Definitions: []parser.Definition{{Body: e}}})
		},
		"step definition bound": func(e *parser.Expression) parser.Module {
			return stepModule(parser.ProofStep{Definitions: []parser.Definition{{Bounds: []parser.Bound{{Set: e}}}}})
		},
		"step directive fact": func(e *parser.Expression) parser.Module {
			return stepModule(parser.ProofStep{Directive: &parser.ProofDirective{Facts: []*parser.Expression{e}}})
		},
		"step directive def": func(e *parser.Expression) parser.Module {
			return stepModule(parser.ProofStep{Directive: &parser.ProofDirective{Defs: []*parser.Expression{e}}})
		},
		"nested proof": func(e *parser.Expression) parser.Module {
			return stepModule(parser.ProofStep{Proof: &parser.Proof{Steps: []parser.ProofStep{{Statement: e}}}})
		},
		"module directive fact": func(e *parser.Expression) parser.Module {
			return parser.Module{ProofDirectives: []parser.ProofDirective{{Facts: []*parser.Expression{e}}}}
		},
		"module directive def": func(e *parser.Expression) parser.Module {
			return parser.Module{ProofDirectives: []parser.ProofDirective{{Defs: []*parser.Expression{e}}}}
		},
	}
	for name, makeModule := range cases {
		t.Run(name, func(t *testing.T) {
			for _, valid := range []bool{false, true} {
				expr := &parser.Expression{RecursiveDeclarations: []parser.Declaration{{Name: "F", Arity: 1}}}
				if valid {
					expr.Definitions = []parser.Definition{{Name: "F", Params: []parser.Declaration{{Name: "x"}}}}
				}
				module := makeModule(expr)
				if err := validateRecursiveModule(&module); (err == nil) != valid {
					t.Fatalf("valid %t: %v", valid, err)
				}
			}
		})
	}
}

func proofModule(proof parser.Proof) parser.Module {
	return parser.Module{Theorems: []parser.Theorem{{Proof: &proof}}}
}

func stepModule(step parser.ProofStep) parser.Module {
	return proofModule(parser.Proof{Steps: []parser.ProofStep{step}})
}
