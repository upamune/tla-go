package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"slices"
	"strings"

	"github.com/upamune/tla-go/internal/heapcapture"
)

type profileFile struct {
	file *os.File
	err  error
}

func (f *profileFile) Write(p []byte) (int, error) {
	n, err := f.file.Write(p)
	if err != nil && f.err == nil {
		f.err = err
	}
	return n, err
}

type profiles struct {
	trace, cpu, heap, recorder *profileFile
	traceStarted, cpuStarted   bool
	heapCaptured               bool
	heapErr                    error
}

func startProfiles(opts options) (*profiles, error) {
	p := &profiles{}
	paths := []struct {
		name, path string
		dst        **profileFile
	}{{"trace", opts.tracePath, &p.trace}, {"CPU profile", opts.cpuPath, &p.cpu}, {"heap profile", opts.heapPath, &p.heap}, {"recorder diagnostics", recorderPath(opts), &p.recorder}}
	for _, item := range paths {
		if err := createProfile(item.name, item.path, item.dst); err != nil {
			p.discard()
			return nil, err
		}
	}
	if err := startRecorderOutput(p.recorder); err != nil {
		p.discard()
		return nil, err
	}
	if p.trace != nil {
		if err := trace.Start(p.trace); err != nil {
			p.discard()
			return nil, fmt.Errorf("start trace: %w", err)
		}
		p.traceStarted = true
	}
	if p.cpu != nil {
		if err := pprof.StartCPUProfile(p.cpu); err != nil {
			p.discard()
			return nil, fmt.Errorf("start CPU profile: %w", err)
		}
		p.cpuStarted = true
	}
	return p, nil
}

func createProfile(name, path string, dst **profileFile) error {
	if path == "" {
		return nil
	}
	// Never create a model or configuration file, even if it does not exist yet.
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".tla" || ext == ".cfg" {
		return fmt.Errorf("%s: output must not be a .tla or .cfg file", name)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	*dst = &profileFile{file: f}
	return nil
}

func (p *profiles) discard() {
	if p.cpuStarted {
		pprof.StopCPUProfile()
	}
	if p.traceStarted {
		trace.Stop()
	}
	for _, f := range []*profileFile{p.trace, p.cpu, p.heap, p.recorder} {
		if f != nil {
			name := f.file.Name()
			_ = f.file.Close()
			_ = os.Remove(name)
		}
	}
}

func (p *profiles) close() error {
	if p.cpuStarted {
		pprof.StopCPUProfile()
	}
	if p.traceStarted {
		trace.Stop()
	}
	var errs []error
	if p.heap != nil {
		p.captureHeap()
		errs = append(errs, p.heapErr)
	}
	for _, f := range []*profileFile{p.trace, p.cpu, p.recorder} {
		errs = append(errs, closeProfile(f))
	}
	errs = append(errs, p.closeHeap())
	return errors.Join(errs...)
}

func (p *profiles) closeHeap() error {
	closeErr := closeProfile(p.heap)
	if p.heap != nil && (p.heapErr != nil || closeErr != nil) {
		if err := os.Remove(p.heap.file.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return errors.Join(closeErr, fmt.Errorf("remove incomplete heap profile: %w", err))
		}
	}
	return closeErr
}

// captureHeap records exactly one snapshot, choosing the first search that runs.
func (p *profiles) captureHeap() {
	if p.heap == nil || p.heapCaptured {
		return
	}
	p.heapCaptured = true
	runtime.GC()
	var buf bytes.Buffer
	if err := pprof.WriteHeapProfile(&buf); err != nil {
		p.heapErr = fmt.Errorf("write heap profile: %w", err)
		return
	}
	if _, err := p.heap.Write(buf.Bytes()); err != nil {
		p.heapErr = fmt.Errorf("write heap profile: %w", err)
	}
}

func closeProfile(f *profileFile) error {
	if f == nil {
		return nil
	}
	var errs []error
	if f.err != nil {
		errs = append(errs, fmt.Errorf("write %s: %w", f.file.Name(), f.err))
	}
	if err := f.file.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close %s: %w", f.file.Name(), err))
	}
	return errors.Join(errs...)
}

func profileRegion(ctx context.Context, name string) func() {
	if !trace.IsEnabled() {
		return func() {}
	}
	region := trace.StartRegion(ctx, name)
	return region.End
}

// resolvedOutput resolves existing symlinks, including dangling final links,
// while allowing the destination and its parent directories not to exist yet.
func resolvedOutput(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return resolveOutputPath(filepath.Clean(path), 0)
}

func resolveOutputPath(path string, depth int) (string, error) {
	if depth > 255 {
		return "", fmt.Errorf("too many symbolic links: %s", path)
	}
	if path == filepath.Dir(path) {
		return path, nil
	}
	parent, err := resolveOutputPath(filepath.Dir(path), depth+1)
	if err != nil {
		return "", err
	}
	path = filepath.Join(parent, filepath.Base(path))
	return resolveOutputLink(path, parent, depth)
}

func resolveOutputLink(path, parent string, depth int) (string, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return path, nil
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return path, nil
	}
	target, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(parent, target)
	}
	return resolveOutputPath(filepath.Clean(target), depth+1)
}

func outputAliases(a, b string) (bool, error) {
	x, err := resolvedOutput(a)
	if err != nil {
		return false, err
	}
	y, err := resolvedOutput(b)
	if err != nil {
		return false, err
	}
	if x == y {
		return true, nil
	}
	return existingFileAliases(a, b)
}

func existingFileAliases(a, b string) (bool, error) {
	ai, ae := os.Stat(a)
	bi, be := os.Stat(b)
	if ae != nil && !os.IsNotExist(ae) {
		return false, ae
	}
	if be != nil && !os.IsNotExist(be) {
		return false, be
	}
	return ae == nil && be == nil && os.SameFile(ai, bi), nil
}

func protectProfileOutput(opts options, path string) error {
	if path == "" {
		return nil
	}
	for _, profile := range []string{opts.tracePath, opts.cpuPath, opts.heapPath, recorderPath(opts)} {
		if profile == "" {
			continue
		}
		same, err := outputAliases(profile, path)
		if err != nil {
			return fmt.Errorf("validate diagnostic output: %w", err)
		}
		if same {
			return fmt.Errorf("diagnostic output %q conflicts with writer %q", profile, path)
		}
	}
	return nil
}

func validateProfilePaths(opts options) error {
	profiles := []string{opts.tracePath, opts.cpuPath, opts.heapPath, recorderPath(opts)}
	if !hasProfilePaths(profiles) {
		return nil
	}
	if err := validateMetadataDump(opts.dumpPath); err != nil {
		return err
	}
	paths, err := expandModelPaths(opts.paths, opts.configPath != "")
	if err != nil {
		return err
	}
	for _, writer := range profileWriters(opts, paths) {
		if err := protectProfileOutput(opts, writer); err != nil {
			return err
		}
	}
	return protectProfileDirectories(profiles, profileCleanupRoots(opts, paths))
}

func hasProfilePaths(paths []string) bool {
	for _, path := range paths {
		if path != "" {
			return true
		}
	}
	return false
}

func validateMetadataDump(path string) error {
	suffix, ok := strings.CutPrefix(path, "${metadir}")
	if !ok {
		return nil
	}
	if slices.Contains(strings.Split(filepath.ToSlash(suffix), "/"), "..") {
		return fmt.Errorf("diagnostics cannot be combined with a metadata dump path containing ..")
	}
	return nil
}

func profileWriters(opts options, paths []string) []string {
	writers := []string{opts.compat.userFile}
	if !strings.HasPrefix(opts.dumpPath, "${metadir}") {
		writers = append(writers, opts.dumpPath)
		writers = append(writers, livenessDumpPath(opts))
	}
	if opts.compat.specTE != nil {
		for _, path := range paths {
			writers = append(writers, filepath.Join(filepath.Dir(path), "SpecTE.tla"), filepath.Join(filepath.Dir(path), "SpecTE.cfg"))
		}
	}
	return writers
}

func profileCleanupRoots(opts options, paths []string) []string {
	var roots []string
	if opts.compat.cleanup {
		roots = append(roots, "states")
	}
	if opts.metaRoot != "" {
		roots = append(roots, opts.metaRoot)
	} else if opts.checkpointInterval > 0 {
		for _, path := range paths {
			roots = append(roots, filepath.Join(filepath.Dir(path), "states"))
		}
	}
	if opts.recoverDir != "" {
		roots = append(roots, opts.recoverDir)
	}
	return roots
}

func protectProfileDirectories(profiles, roots []string) error {
	for _, profile := range profiles {
		if profile == "" {
			continue
		}
		for _, root := range roots {
			if err := protectProfileDirectory(profile, root); err != nil {
				return err
			}
		}
	}
	return nil
}

func protectProfileDirectory(profile, root string) error {
	p, err := resolvedOutput(profile)
	if err != nil {
		return err
	}
	r, err := resolvedOutput(root)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(r, p)
	if err != nil {
		return err
	}
	if rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("diagnostic output %q is inside metadata or cleanup directory %q", profile, root)
	}
	return nil
}

func (p *profiles) context(ctx context.Context) context.Context {
	if p.heap == nil {
		return ctx
	}
	return heapcapture.With(ctx, p.captureHeap)
}
