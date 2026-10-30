package checker

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"testing"
)

func TestModuleLoaderErrorClassification(t *testing.T) {
	for _, dependency := range []string{"EXTENDS A", "I == INSTANCE A"} {
		for _, tc := range []struct {
			name       string
			err        error
			parseError bool
		}{
			{"missing", fmt.Errorf("loader: %w", fs.ErrNotExist), true},
			{"cancelled", context.Canceled, false},
			{"permission denied", fs.ErrPermission, false},
		} {
			t.Run(dependency+"/"+tc.name, func(t *testing.T) {
				loader := func(context.Context, string) (string, error) { return "", tc.err }
				_, err := CheckWithLoader(context.Background(), "---- MODULE M ----\n"+dependency+"\n====\n", "", loader)
				if !errors.Is(err, tc.err) {
					t.Fatalf("error %v does not preserve %v", err, tc.err)
				}
				_, isSyntax := errors.AsType[*SyntaxError](err)
				if isSyntax != tc.parseError {
					t.Fatalf("error %v: syntax classification %v, want %v", err, isSyntax, tc.parseError)
				}
			})
		}
	}
}
