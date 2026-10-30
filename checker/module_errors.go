package checker

import (
	"errors"
	"fmt"
	"io/fs"
)

func moduleLoadError(kind, name string, err error) error {
	wrapped := fmt.Errorf("load %s %s: %w", kind, name, err)
	if errors.Is(err, fs.ErrNotExist) {
		return &SyntaxError{Err: wrapped}
	}
	return wrapped
}

func moduleNameError(name, declared string) error {
	return &SyntaxError{Err: fmt.Errorf("File name '%s' does not match the name '%s' of the top level module it contains.", name, declared)}
}

func moduleCycleError(name string) error {
	return &SyntaxError{Err: fmt.Errorf("cyclic module dependency involving %s", name)}
}

func instanceCycleError(name string) error {
	return &SyntaxError{Err: fmt.Errorf("cyclic instance dependency involving %s", name)}
}
