package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func runModels(ctx context.Context, opts options, out, errout io.Writer) int {
	paths, err := expandModelPaths(opts.paths, opts.configPath != "")
	if err != nil {
		fmt.Fprintln(errout, err)
		return 1
	}
	if len(paths) == 0 {
		usage(errout)
		return 1
	}
	if opts.configPath != "" && len(paths) > 1 {
		fmt.Fprintln(errout, "Error: -config applies to one specification, but several were given.")
		return 1
	}
	return checkModelPaths(ctx, opts, paths, out, errout)
}

func checkModelPaths(ctx context.Context, opts options, paths []string, out, errout io.Writer) int {
	seen := map[string]bool{}
	firstFailure := 0
	for _, path := range paths {
		if seen[path] {
			continue
		}
		if len(seen) > 0 {
			fmt.Fprintln(out)
		}
		seen[path] = true
		opts.path = path
		code := runModel(ctx, opts, out, errout)
		if firstFailure == 0 && code != 0 {
			firstFailure = code
		}
		if ctx.Err() != nil {
			return firstFailure
		}
	}
	return firstFailure
}

func expandModelPaths(paths []string, configured bool) ([]string, error) {
	var result []string
	for _, pattern := range paths {
		matches, globbed, err := matchModelPattern(pattern)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			if !globbed || includeGlobbedModel(match, configured) {
				result = append(result, normalizeModelPath(match))
			}
		}
	}
	return result, nil
}

func matchModelPattern(pattern string) ([]string, bool, error) {
	if !strings.ContainsAny(pattern, "*?[") {
		return []string{pattern}, false, nil
	}
	matches, err := filepath.Glob(pattern)
	return matches, true, err
}

func includeGlobbedModel(path string, configured bool) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".tla" && ext != ".cfg" {
		return false
	}
	if configured {
		return true
	}
	path = normalizeModelPath(path)
	cfg := path[:len(path)-len(filepath.Ext(path))] + ".cfg"
	_, err := os.Stat(cfg)
	return !os.IsNotExist(err)
}

func normalizeModelPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".cfg" {
		path = path[:len(path)-len(ext)] + ".tla"
	} else if ext == "" {
		path += ".tla"
	}
	return filepath.Clean(path)
}
