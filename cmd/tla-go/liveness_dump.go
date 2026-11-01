package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/upamune/tla-go/checker"
)

// livenessDump stages the graph so unsupported models and incomplete searches
// do not leave a misleading liveness graph behind.
type livenessDump struct {
	path, temporary string
	dump            *dotDump
	failure         error
	opts            options
	checked, skip   bool
}

func prepareGraphDumps(opts options) (stateDump, *livenessDump, error) {
	graph, err := openStateDump(opts.dumpPath, opts.dumpDOT)
	path := livenessDumpPath(opts)
	if err != nil || path == "" {
		return graph, nil, err
	}
	return graph, &livenessDump{opts: opts}, nil
}

func finishGraphDumps(graph stateDump, live *livenessDump, result checker.Result, err error) error {
	if graph != nil {
		if closeErr := graph.close(graphShouldFinish(result, err)); err == nil {
			err = closeErr
		}
	}
	if live != nil {
		if closeErr := live.close(err == nil); err == nil {
			err = closeErr
		}
	}
	return err
}

func livenessDumpPath(opts options) string {
	if opts.dumpDOT == nil || strings.Count(opts.dumpPath, ".dot") != 1 || !strings.HasSuffix(opts.dumpPath, ".dot") {
		return ""
	}
	return strings.TrimSuffix(opts.dumpPath, ".dot") + "_liveness.dot"
}

func safeLivenessDumpPath(opts options) (string, error) {
	path := livenessDumpPath(opts)
	if path == "" {
		return "", nil
	}
	spec, cfg, err := inputPaths(opts)
	if err != nil {
		return "", err
	}
	for _, other := range []string{opts.dumpPath, spec, cfg} {
		alias, err := outputAliases(path, other)
		if err != nil {
			return "", err
		}
		if alias {
			return "", nil
		}
	}
	if !livenessDestinationAvailable(path) {
		return "", nil
	}
	return path, nil
}

func (d *livenessDump) event(event checker.GraphEvent) error {
	if !d.checked {
		d.checked = true
		path, err := safeLivenessDumpPath(d.opts)
		if err != nil || path == "" || protectProfileOutput(d.opts, path) != nil {
			d.skip = true
		} else {
			d.path = path
		}
	}
	if d.skip {
		return nil
	}
	if d.failure == nil {
		d.failure = d.writeEvent(event)
	}
	return nil
}

func (d *livenessDump) writeEvent(event checker.GraphEvent) error {
	if d.dump == nil {
		file, err := os.CreateTemp(filepath.Dir(d.path), ".tla-liveness-*.tmp")
		if err != nil {
			return fmt.Errorf("create liveness graph: %w", err)
		}
		d.temporary = file.Name()
		d.dump, err = newDOTDump(file, dotFormat{liveness: true})
		if err != nil {
			return err
		}
	}
	return d.dump.event(event)
}

func livenessDestinationAvailable(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return true
	}
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	return file.Close() == nil
}

func (d *livenessDump) close(success bool) error {
	if d.temporary == "" {
		return d.failure
	}
	defer os.Remove(d.temporary)
	if d.dump == nil {
		return d.failure
	}
	if err := d.dump.close(success && d.failure == nil); err != nil {
		return err
	}
	if d.failure != nil {
		return d.failure
	}
	if !success {
		return nil
	}
	return writeLivenessDestination(d.temporary, d.path)
}

func writeLivenessDestination(temporary, path string) error {
	source, err := os.Open(temporary)
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return fmt.Errorf("write liveness graph: %w", err)
	}
	_, err = io.Copy(destination, source)
	if closeErr := destination.Close(); err == nil {
		err = closeErr
	}
	return err
}
