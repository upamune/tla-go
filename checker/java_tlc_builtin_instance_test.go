package checker

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
)

func TestStandardTLCInstanceWithoutLoader(t *testing.T) {
	source := "---- MODULE M ----\nX == INSTANCE TLC\nASSUME X!TLCSet(1, 42) /\\ X!TLCGet(1) = 42 /\\ X!PrintT(\"hello\")\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n===="
	var output []string
	result, err := CheckWithOptions(context.Background(), source, "INIT Init\nNEXT Next\n", Options{OnPrint: func(line string) { output = append(output, line) }})
	if err != nil || result.Status != Success || result.DistinctStates != 1 || strings.Join(output, "\n") != `"hello"` {
		t.Fatalf("result %+v, error %v, output %q", result, err, output)
	}
}

func TestStandardTLCInstanceLoaderFallback(t *testing.T) {
	source := "---- MODULE M ----\nX == INSTANCE TLC\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n===="
	configuration := "INIT Init\nNEXT Next\n"
	missing := func(context.Context, string) (string, error) { return "", fs.ErrNotExist }
	result, err := CheckWithLoader(context.Background(), source, configuration, missing)
	if err != nil || result.Status != Success {
		t.Fatalf("missing standard module: result %+v, error %v", result, err)
	}
	denied := func(context.Context, string) (string, error) { return "", fs.ErrPermission }
	_, err = CheckWithLoader(context.Background(), source, configuration, denied)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("loader error %v, want permission error", err)
	}
}
