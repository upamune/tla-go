// Package checker explores reachable states of a finite TLA+ model.
// The implementation is incomplete; unsupported constructs return errors.
package checker

import (
	"context"
	"fmt"
	"slices"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

type Status string

const (
	Success                Status = "success"
	Deadlock               Status = "deadlock"
	InvariantViolation     Status = "invariant-violation"
	AssumptionViolation    Status = "assumption-violation"
	LivenessViolation      Status = "liveness-violation"
	PropertyViolation      Status = "property-violation"
	PostconditionViolation Status = "postcondition-violation"
	EvaluationFailure      Status = "error"
)

type TraceStep struct {
	Action string
	State  evaluator.State
	// ActionRange is the source range of the action, when available.
	ActionRange parser.Range
	// StateOrder records the display order for state or ALIAS-generated fields.
	StateOrder []string
}
type Violation struct {
	Name        string
	Message     string
	Counts      string
	Unwitnessed *PossibleViolation
	Trace       []TraceStep
	// Properties lists the properties violated by the reported lasso, in configuration order.
	Properties []string
	// LoopStart is the zero-based index of the state reached after the last
	// trace step for a liveness counterexample. It is nil for finite traces.
	LoopStart       *int
	LoopAction      string
	LoopActionRange parser.Range
}

// PossibleViolation reports an unwitnessed _POSSIBLE condition.
type PossibleViolation struct {
	Name, Message, Counts string
}

type Result struct {
	Status                                 Status
	Violation                              *Violation
	Failure                                *Failure
	GeneratedStates, DistinctStates, Depth int
	QueuedStates                           int
	OutDegrees                             [32]int
	HasOutDegrees                          bool
}

// Failure describes an evaluation that stopped exploration.
type Failure struct {
	Message           string
	Trace             []TraceStep
	InitialState      evaluator.State
	InitialStateOrder []string
	ExitCode          int
	// SuppressStatistics indicates that TLC stops before reporting search statistics.
	SuppressStatistics bool
	// FingerprintDetail records TLC's source stack and underlying cause, if any.
	FingerprintDetail string
	// EvaluationDetail records the expression stack for an unexpected failure.
	EvaluationDetail string
	// SuppressDepth matches TLC failures that report counts without search depth.
	SuppressDepth     bool
	evaluationSources []*parser.Expression
}
type model struct {
	module            *parser.Module
	cfg               config.Config
	descriptionConfig config.Config
	sourceConfig      config.Config
	program           *evaluator.Program
	defs              map[string]parser.Definition
	variables         map[string]bool
	constants         map[string]value.Value
	symmetry          []permutation
	symmetryOrder     []string
	valueRanks        map[string]int
	sourceUnits       map[string]*parser.Module
	rankedSets        map[value.Value][]value.Value
	rankedFunctions   map[value.Value][]value.Pair
	names             []string
	ctx               context.Context
	actionCache       map[*parser.Expression]*parser.Expression
	letCache          map[*parser.Expression]*parser.Expression
	tlcFingerprint    bool
	fromSpecification bool
	tlcRuntime        *tlcRuntime
	enabledCallback   evaluator.EnabledCallback
	properties        []temporalProperty
	livenessOrder     []temporalProperty
	safety            []temporalProperty
	guarded           []temporalProperty
	fairness          []fairnessCondition
	possibleStates    []possiblePredicate
	possibleActions   []possiblePredicate
	debug             func(string)
}

// ModuleLoader returns the source of an external module by its declared name.
type ModuleLoader func(context.Context, string) (string, error)

// Options configures external module loading and TLC output.
type Options struct {
	Loader    ModuleLoader
	OnPrint   func(string)
	OnPause   func()
	ReadFile  func(string) (string, error)
	WriteFile func(string, string) error
	Seed      int64
	// ModernRandomSubsets uses the newer TLC random-subset algorithm.
	ModernRandomSubsets bool
	// OnDebug receives JSON lines describing liveness exploration when set.
	OnDebug func(string)
}

// Check parses a module and a TLC configuration and explores the model.
// Unsupported features return an error rather than an incomplete success.
func Check(source, configuration string) (Result, error) {
	return CheckContext(context.Background(), source, configuration)
}

// CheckContext supports cancellation during state exploration.
// SyntaxError wraps a TLA+ parsing failure.
type SyntaxError struct{ Err error }

func (e *SyntaxError) Error() string { return e.Err.Error() }
func (e *SyntaxError) Unwrap() error { return e.Err }

// SemanticError wraps a TLA+ semantic validation failure.
type SemanticError struct{ Err error }

func (e *SemanticError) Error() string { return e.Err.Error() }
func (e *SemanticError) Unwrap() error { return e.Err }

// CheckContext supports cancellation during state exploration.
func CheckContext(ctx context.Context, source, configuration string) (Result, error) {
	return CheckWithLoader(ctx, source, configuration, nil)
}

// CheckConfig accepts an already parsed TLC configuration.
func CheckConfig(ctx context.Context, source string, cfg config.Config) (Result, error) {
	return CheckConfigWithLoader(ctx, source, cfg, nil)
}

// CheckWithLoader checks a model with external modules supplied by loader.
func CheckWithLoader(ctx context.Context, source, configuration string, loader ModuleLoader) (Result, error) {
	module, err := parser.ParseModule(source)
	if err != nil {
		return Result{}, &SyntaxError{Err: err}
	}
	cfg, err := config.Parse(configuration)
	if err != nil {
		return Result{}, err
	}
	return checkParsedWithOptions(ctx, module, cfg, Options{Loader: loader})
}

// CheckWithOptions checks a model with an output callback and optional loader.
func CheckWithOptions(ctx context.Context, source, configuration string, options Options) (Result, error) {
	module, err := parser.ParseModule(source)
	if err != nil {
		return Result{}, &SyntaxError{Err: err}
	}
	cfg, err := config.Parse(configuration)
	if err != nil {
		return Result{}, err
	}
	return checkParsedWithOptions(ctx, module, cfg, options)
}

// CheckConfigWithLoader checks a parsed configuration and loads EXTENDS modules.
func CheckConfigWithLoader(ctx context.Context, source string, cfg config.Config, loader ModuleLoader) (Result, error) {
	module, err := parser.ParseModule(source)
	if err != nil {
		return Result{}, &SyntaxError{Err: err}
	}
	return checkParsedWithOptions(ctx, module, cfg, Options{Loader: loader})
}

// CheckModuleWithLoader checks a parsed module and configuration.
func CheckModuleWithLoader(ctx context.Context, module *parser.Module, cfg config.Config, loader ModuleLoader) (Result, error) {
	if module == nil {
		return Result{}, fmt.Errorf("missing module")
	}
	return checkParsedWithOptions(ctx, module, cfg, Options{Loader: loader})
}

// CheckModuleWithOptions checks a parsed module with an output callback.
func CheckModuleWithOptions(ctx context.Context, module *parser.Module, cfg config.Config, options Options) (Result, error) {
	if module == nil {
		return Result{}, fmt.Errorf("missing module")
	}
	return checkParsedWithOptions(ctx, module, cfg, options)
}

func checkParsedWithOptions(ctx context.Context, module *parser.Module, cfg config.Config, options Options) (Result, error) {
	session := newInstanceSession(module.Name)
	session.sourceUnits[module.Name] = module
	module, err := resolveExtensionsScoped(ctx, module, options.Loader, session)
	if err != nil {
		return Result{}, err
	}
	module, err = resolveInstancesScoped(ctx, module, options.Loader, session)
	if err != nil {
		return Result{}, err
	}
	return checkParsedWithOutput(ctx, module, cfg, options, session.sourceUnits)
}

func checkParsed(ctx context.Context, module *parser.Module, cfg config.Config) (Result, error) {
	return checkParsedWithOutput(ctx, module, cfg, Options{}, nil)
}

func checkParsedWithOutput(ctx context.Context, module *parser.Module, cfg config.Config, options Options, sourceUnits map[string]*parser.Module) (Result, error) {
	model, err := prepareWithSources(module, cfg, options, sourceUnits)
	if err != nil {
		return Result{}, err
	}
	model.ctx = ctx
	result, err := model.check()
	if err != nil {
		return result, err
	}
	return model.finalizeCheckResult(result)
}

func (m *model) finalizeCheckResult(result Result) (Result, error) {
	if m.needsViewRecovery(result) {
		if err := m.recoverViewResult(&result); err != nil {
			return result, err
		}
	}
	if m.cfg.Alias != "" {
		m.applyAlias(&result)
	}
	m.applyTraceOrder(&result)
	m.applyActionRanges(&result)
	if result.Status == AssumptionViolation || result.Status == EvaluationFailure || m.noStateSearch() {
		return result, nil
	}
	return m.checkPostconditions(result)
}

func (m *model) needsViewRecovery(result Result) bool {
	return m.hasReduction() && (result.Status != LivenessViolation || !m.tlcRuntime.viewTraceReplayed)
}

func (m *model) noStateSearch() bool {
	return len(m.names) == 0 && m.cfg.Init == "!spec!init"
}

func prepare(module *parser.Module, cfg config.Config) (*model, error) {
	return prepareWithOutput(module, cfg, Options{})
}

func prepareWithOutput(module *parser.Module, cfg config.Config, options Options) (*model, error) {
	return prepareWithSources(module, cfg, options, nil)
}

func prepareWithSources(module *parser.Module, cfg config.Config, options Options, sourceUnits map[string]*parser.Module) (*model, error) {
	var err error
	var fairness []fairnessCondition
	module, err = resolveSubexpressions(module)
	if err != nil {
		return nil, err
	}
	sourceConfig := cfg
	fromSpecification := cfg.Specification != ""
	if err = validateConfigTables(cfg); err != nil {
		return nil, err
	}
	cfg = discardIneffectiveScopedAssignments(module, cfg)
	module, cfg, err = applyConfigOverrides(module, cfg)
	if err != nil {
		return nil, err
	}
	descriptionConfig := cfg
	cfg, fairness, err = lowerSpecification(module, cfg)
	if err != nil {
		return nil, err
	}
	cfg = defaultEmptyModel(module, cfg)
	if err = validate(module, cfg); err != nil {
		return nil, err
	}
	if err = validateResolved(module); err != nil {
		return nil, err
	}
	runtime := newTLCRuntime(cfg, options)
	m := &model{ctx: context.Background(), module: module, cfg: cfg, descriptionConfig: descriptionConfig, sourceConfig: sourceConfig, fairness: fairness, fromSpecification: fromSpecification, tlcRuntime: runtime, program: newEvaluatorProgram(module, options.OnPrint, runtime), defs: map[string]parser.Definition{}, variables: map[string]bool{}, constants: map[string]value.Value{}, actionCache: map[*parser.Expression]*parser.Expression{}, letCache: map[*parser.Expression]*parser.Expression{}, debug: options.OnDebug}
	m.program.SetJSONFileSystem(options.ReadFile, options.WriteFile)
	m.sourceUnits = sourceUnits
	runtime.model = m
	m.enabledCallback = m.enabled
	m.tlcFingerprint = useTLCFingerprint(module, cfg)
	if err := m.bindAndValidate(); err != nil {
		return nil, err
	}
	m.tlcRuntime.needsLevelGraph = m.temporalUsesTLC()
	return m, nil
}

func (m *model) bindAndValidate() error {
	m.prepareValueOrdering()
	m.program.SetOrdering(modelOrdering{model: m})
	if err := m.bindModel(); err != nil {
		return err
	}
	m.prepareRandomConstants()
	if err := m.prepareSymmetry(); err != nil {
		return err
	}
	return m.validateModel()
}

func newEvaluatorProgram(module *parser.Module, onPrint func(string), runtime *tlcRuntime) *evaluator.Program {
	if onPrint != nil {
		forward := onPrint
		onPrint = func(text string) {
			if !runtime.outputMuted {
				forward(text)
			}
		}
	}
	if slices.Contains(module.Extends, "TLC") || slices.Contains(module.Extends, "TLCExt") || slices.Contains(module.Extends, "Randomization") {
		return evaluator.NewWithRuntime(module, onPrint, runtime)
	}
	return evaluator.NewWithPrint(module, onPrint)
}

func defaultEmptyModel(module *parser.Module, cfg config.Config) config.Config {
	if len(module.Variables) != 0 || cfg.Init != "" || cfg.Next != "" || cfg.Specification != "" {
		return cfg
	}
	parts := specificationParts{module: module}
	truth := &parser.Expression{Kind: "Identifier", Text: "TRUE"}
	cfg.Init = parts.addWrapper("init", truth)
	cfg.Next = parts.addWrapper("next", truth)
	return cfg
}

func applyConfigOverrides(module *parser.Module, cfg config.Config) (*parser.Module, config.Config, error) {
	original := cfg
	module, cfg, err := applyConfigAssignments(module, cfg)
	if err != nil || len(cfg.Substitutions) == 0 {
		return module, cfg, err
	}
	return applyConfigSubstitutions(module, cfg, original)
}

func (m *model) validateModel() error {
	if err := m.validateDefinitions(); err != nil {
		return err
	}
	if err := m.validateEnabledLevels(); err != nil {
		return err
	}
	if err := m.validatePredicates(); err != nil {
		return err
	}
	if err := m.validateActions(); err != nil {
		return err
	}
	if err := m.prepareFairness(); err != nil {
		return err
	}
	if err := m.prepareProperties(); err != nil {
		return err
	}
	return m.validatePossibleAndConstants()
}

func (m *model) validatePossibleAndConstants() error {
	if err := m.preparePossible(); err != nil {
		return err
	}
	if err := m.validateStateDependence(); err != nil {
		return err
	}
	return m.validateConstantPredicates()
}

func (m *model) bindModel() error {
	for _, v := range m.module.Variables {
		m.variables[v.Name] = true
		m.names = append(m.names, v.Name)
	}
	for _, d := range m.module.Definitions {
		m.defs[d.Name] = d
	}
	for _, assignment := range m.cfg.Constants {
		v, err := m.boundConfigValue(assignment.Value)
		if err != nil {
			return err
		}
		m.constants[assignment.Name] = v
	}
	for _, d := range m.module.Constants {
		if _, ok := m.constants[d.Name]; !ok {
			return fmt.Errorf("constant %s is not assigned", d.Name)
		}
	}
	return nil
}

func (m *model) validateDefinitions() error {
	names := append([]string{m.cfg.Init, m.cfg.Next}, m.cfg.Invariants...)
	names = append(names, m.cfg.Constraints...)
	names = append(names, m.cfg.ActionConstraints...)
	names = append(names, m.cfg.Properties...)
	names = append(names, m.cfg.Postconditions...)
	if m.cfg.View != "" {
		names = append(names, m.cfg.View)
	}
	for _, name := range names {
		if _, err := m.definition(name); err != nil {
			return err
		}
	}
	return nil
}

func (m *model) definition(name string) (*parser.Expression, error) {
	d, ok := m.defs[name]
	if !ok || d.Kind != "Operator" || len(d.Params) != 0 {
		return nil, fmt.Errorf("configuration operator %q is undefined or requires arguments", name)
	}
	return d.Body, nil
}

func (m *model) eval(expr *parser.Expression, current, next evaluator.State) (value.Value, error) {
	return m.program.Evaluate(expr, evaluator.Context{Current: current, Next: next, Constants: m.constants, Enabled: m.enabledCallback})
}

func (m *model) holds(expr *parser.Expression, current, next evaluator.State) (bool, error) {
	v, err := m.eval(expr, current, next)
	return predicateBoolean(v, err)
}

func predicateBoolean(v value.Value, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	b, ok := v.Bool()
	if !ok {
		return false, fmt.Errorf("predicate must be boolean, got %s", v)
	}
	return b, nil
}
