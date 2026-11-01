package checker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// checkpointIdentityVersion identifies both this encoding and the checker and
// built-in module semantics. Bump it whenever those semantics can change.
const checkpointIdentityVersion = "tla-go-checkpoint-identity-v1"

type (
	checkpointSource          struct{ Name, Source string }
	checkpointIdentityOptions struct {
		Coverage                      bool
		Seed                          int64
		Fingerprint                   int
		Continue, ModernRandomSubsets bool
		PrintStyle                    evaluator.PrintStyle
		MaxSetSize, Workers           int
		SerialExploration, ViewStates bool
	}
)

func (s *instanceSession) recordSource(name, source string) error {
	if s.sources == nil {
		return nil
	} // Ordinary checks do not retain source text.
	if old, ok := s.sources[name]; ok && old != source {
		return fmt.Errorf("module %s loaded with inconsistent source", name)
	}
	s.sources[name] = source
	return nil
}

// resolveCheckpointIdentity resolves the same module graph as a normal check,
// capturing source as it is loaded, without evaluating the model. The effective
// config is supplied separately to capture command-line overrides as well as the
// complete config source. Callers must use the returned module and session for
// the search, never reload them after constructing the identity.
func resolveCheckpointIdentity(ctx context.Context, source, configSource string, effective config.Config, options Options) (*parser.Module, *instanceSession, checkpointIdentity, error) {
	var zero checkpointIdentity
	if source == "" {
		return nil, nil, zero, fmt.Errorf("checkpoint requires complete module source")
	}
	if _, err := config.Parse(configSource); err != nil {
		return nil, nil, zero, err
	}
	if err := checkpointIdentityOptionsSupported(options); err != nil {
		return nil, nil, zero, err
	}
	module, session, rootName, err := resolveCheckpointSources(ctx, source, options.Loader)
	if err != nil {
		return nil, nil, zero, err
	}
	identity, err := hashCheckpointInputs(rootName, session.sources, configSource, effective, options)
	if err != nil {
		return nil, nil, zero, err
	}
	return module, session, identity, nil
}

func resolveCheckpointSources(ctx context.Context, source string, loader ModuleLoader) (*parser.Module, *instanceSession, string, error) {
	root, err := parser.ParseModule(source)
	if err != nil {
		return nil, nil, "", err
	}
	session := newInstanceSession(root.Name)
	session.sources = map[string]string{root.Name: source}
	session.sourceUnits[root.Name] = root
	module, err := resolveExtensionsScoped(ctx, root, loader, session)
	if err != nil {
		return nil, nil, "", err
	}
	module, err = resolveInstancesScoped(ctx, module, loader, session)
	if err != nil {
		return nil, nil, "", err
	}
	if session.builtinTLC {
		copy := *module
		copy.HasTLCInstance = true
		module = &copy
	}
	return module, session, root.Name, nil
}

func hashCheckpointInputs(rootName string, sourceMap map[string]string, configSource string, effective config.Config, options Options) (checkpointIdentity, error) {
	names := make([]string, 0, len(sourceMap))
	for name := range sourceMap {
		names = append(names, name)
	}
	sort.Strings(names)
	sources := make([]checkpointSource, 0, len(names))
	for _, name := range names {
		sources = append(sources, checkpointSource{name, sourceMap[name]})
	}
	modelData, err := json.Marshal(struct {
		Version, Root string
		Sources       []checkpointSource
	}{checkpointIdentityVersion, rootName, sources})
	if err != nil {
		return checkpointIdentity{}, err
	}
	bound := options.MaxSetSize
	if bound <= 0 {
		bound = evaluator.DefaultSetBound
	}
	identityOptions := checkpointIdentityOptions{options.Coverage, options.Seed, options.Fingerprint, options.Continue, options.ModernRandomSubsets, options.PrintStyle, bound, max(options.Workers, 1), options.SerialExploration, options.ViewStates}
	configData, err := json.Marshal(struct {
		Version, Source string
		Effective       config.Config
		Options         checkpointIdentityOptions
	}{checkpointIdentityVersion, configSource, effective, identityOptions})
	if err != nil {
		return checkpointIdentity{}, err
	}
	return checkpointIdentity{Model: sha256.Sum256(modelData), Config: sha256.Sum256(configData)}, nil
}

func checkpointIdentityOptionsSupported(o Options) error {
	// New option fields must be classified before identities can be constructed.
	known := map[string]bool{"Coverage": true, "LiveProgress": true, "Loader": true, "OnPrint": true, "OnPause": true, "ReadFile": true, "WriteFile": true, "Seed": true, "Fingerprint": true, "Simulation": true, "DepthFirst": true, "Continue": true, "OnViolation": true, "ModernRandomSubsets": true, "PrintStyle": true, "OnDebug": true, "OnGraph": true, "OnSearch": true, "OnTemporalCheck": true, "OnTemporalViolation": true, "MaxSetSize": true, "Workers": true, "SerialExploration": true, "ViewStates": true, "OnWarning": true}
	value := reflect.ValueOf(o)
	typ := value.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !known[field.Name] {
			return fmt.Errorf("unclassified checkpoint option %s", field.Name)
		}
		if field.Name != "Loader" && (field.Type.Kind() == reflect.Func || field.Type.Kind() == reflect.Pointer) && !value.Field(i).IsNil() {
			return fmt.Errorf("checkpoint identity cannot establish semantics of %s", field.Name)
		}
	}

	return nil
}

// prepareCheckpointModel binds the identity to the exact resolved model used by
// the search. It is intentionally private until persisted recovery is supported.
func prepareCheckpointModel(ctx context.Context, source, configSource string, effective config.Config, options Options) (*model, error) {
	return prepareCheckpointModelObserved(ctx, source, configSource, effective, options, nil)
}

// prepareCheckpointModelObserved installs only checker-owned recording callbacks,
// after rejecting every callback supplied in options. No caller-provided code is
// invoked by the observer. The observer must remain private to the search.
func prepareCheckpointModelObserved(ctx context.Context, source, configSource string, effective config.Config, options Options, observer *checkpointObserver) (*model, error) {
	module, session, identity, err := resolveCheckpointIdentity(ctx, source, configSource, effective, options)
	if err != nil {
		return nil, err
	}
	if err := checkpointPreflight(module, session.sourceUnits, effective); err != nil {
		return nil, &CheckpointUnsupportedError{Err: err}
	}
	if observer != nil {
		options.OnSearch = observer.search
		options.OnWarning = observer.warning
		options.OnPrint = observer.print
	}
	m, err := prepareWithSources(module, effective, options, session.sourceUnits)
	if err != nil {
		return nil, err
	}
	m.ctx = ctx
	m.checkpointObserver = observer
	m.checkpointIdentity = &identity
	return m, nil
}
