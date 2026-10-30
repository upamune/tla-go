package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/upamune/tla-go/config"
)

const configExceptionHeading = "TLC threw an unexpected exception.\nThis was probably caused by an error in the spec or model.\nSee the User Output or TLC Console for clues to what happened.\nThe exception was a tlc2.tool.ConfigFileException\n:"

func configErrorDetail(path string, err error) (int, string) {
	if parseErr, ok := errors.AsType[*config.ParseError](err); ok {
		if parseErr.Exception != "" {
			message := "TLC threw an unexpected exception.\nThis was probably caused by an error in the spec or model.\nSee the User Output or TLC Console for clues to what happened.\nThe exception was a " + parseErr.Exception + "\n: " + parseErr.Message
			return 0, message
		}
		return parseErr.Code, parseErr.Diagnostic()
	}
	if errors.Is(err, os.ErrNotExist) {
		return 5001, fmt.Sprintf("TLC encountered the following error when trying to read the configuration file %s:\nFile not found.", filepath.Base(path))
	}
	if pathErr, ok := errors.AsType[*os.PathError](err); ok {
		return 5001, fmt.Sprintf("TLC encountered the following error when trying to read the configuration file %s:\n%s", filepath.Base(path), pathErr.Err)
	}
	return 0, err.Error()
}

func printConfigError(out io.Writer, path string, err error) {
	code, detail := configErrorDetail(path, err)
	if code == 0 {
		fmt.Fprintf(out, "Error: %s\n", detail)
		return
	}
	fmt.Fprintf(out, "Error: %s %s\n", configExceptionHeading, detail)
}

func printToolConfigError(t toolOutput, path string, err error) {
	code, detail := configErrorDetail(path, err)
	if code == 0 {
		t.message(1000, 1, detail)
		return
	}
	t.nestedInlineError(1000, configExceptionHeading, code, detail)
}
