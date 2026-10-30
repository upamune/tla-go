package checker

import (
	"errors"
	"strings"
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

func TestRecordFieldsValidation(t *testing.T) {
	tests := []struct {
		name, expression string
		duplicate        bool
	}{
		{"distinct record", "[a |-> 1, b |-> 2]", false},
		{"distinct record set", "[a : {1}, b : {2}]", false},
		{"same field in nested records", "[a |-> [a |-> 1]]", false},
		{"duplicate record", "[a |-> 1, a |-> 2]", true},
		{"duplicate record set", "[a : {1}, a : {2}]", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nUnused == " + tc.expression + "\n====\n"
			_, err := Check(source, "")
			_, semantic := errors.AsType[*SemanticError](err)
			if semantic != tc.duplicate || (!tc.duplicate && err != nil) {
				t.Fatalf("error %v, want duplicate %t", err, tc.duplicate)
			}
		})
	}
}

func TestRecordFieldsValidationWithoutParserMetadata(t *testing.T) {
	expr := &parser.Expression{Kind: "Record", Fields: []parser.Field{{Name: "a"}, {Name: "a"}}}
	err := validateRecordFields(expr)
	if _, ok := errors.AsType[*SemanticError](err); !ok || !strings.Contains(err.Error(), "Non-unique fields") {
		t.Fatalf("unexpected error: %v", err)
	}
}
