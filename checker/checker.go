// Package checker explores reachable states of a finite TLA+ model.
// The implementation is incomplete; unsupported constructs return errors.
package checker

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"runtime/trace"
	"slices"
	"time"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/internal/heapcapture"
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
	ContinuedViolation     Status = "continued-violation"
	// DepthLimit reports that depth-first iterative deepening reached its
	// maximum depth without an error before TLC reported success. Like TLC,
	// it is also the result when the last iteration found no new states.
	DepthLimit Status = "depth-limit"
)

type TraceStep struct {
	Action string
	State  evaluator.State
	// ActionRange is the source range of the action, when available.
	ActionRange parser.Range
	// StateOrder records the display order for state or ALIAS-generated fields.
	StateOrder []string
	// View is the state as TLC prints it through VIEW when Options.ViewStates
	// is set. It is empty when the state is printed directly.
	View string
}
type Violation struct {
	Name    string
	Message string
	// AfterInitial distinguishes later simulation errors after stuttering.
	AfterInitial bool
	// ToolCode identifies a nested Java TLC evaluation message when known.
	ToolCode    int
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
	Coverage                               []CoverageAction
	Status                                 Status
	Violation                              *Violation
	Failure                                *Failure
	GeneratedStates, DistinctStates, Depth int
	QueuedStates                           int
	OutDegrees                             [32]int
	HasOutDegrees                          bool
	// ContinuedViolations counts violations reported while exploration continued.
	ContinuedViolations int
	// FingerprintDistance is the smallest distance between state
	// fingerprints that TLC's depth-first mode reports as the observed
	// collision probability on success, when HasFingerprintDistance is set.
	FingerprintDistance    int64
	HasFingerprintDistance bool
	// TemporalCheckDuration is present when a final liveness progress message
	// was emitted. It lets callers place the end message after a counterexample.
	TemporalCheckDuration *time.Duration
	// TemporalCheckReported indicates that OnTemporalCheck completed reporting.
	TemporalCheckReported bool
	// SimulationCounterexampleStarted indicates that OnCounterexampleStart ran.
	SimulationCounterexampleStarted bool
	// SimulationCounterexampleStates indicates that counterexample states were streamed.
	SimulationCounterexampleStates bool
	// SimulationBehaviorStreamed indicates that the sampled failure behavior was emitted.
	SimulationBehaviorStreamed bool
}

// Failure describes an evaluation that stopped exploration.
type Failure struct {
	Message           string
	Trace             []TraceStep
	InitialState      evaluator.State
	InitialStateOrder []string
	ExitCode          int
	// IncompleteInitialState identifies an initial predicate that emitted a partial state.
	IncompleteInitialState bool
	// ToolCode is the Java TLC error message code when known.
	ToolCode int
	// NestedToolCode identifies the nested evaluation message when known.
	NestedToolCode int
	// NestedMessage is the inner message when the outer diagnostic is multiline.
	NestedMessage string
	// SuppressStatistics indicates that TLC stops before reporting search statistics.
	SuppressStatistics bool
	// SuppressSummary indicates that TLC's simulator reports its final
	// progress but not its summary.
	SuppressSummary bool
	// FingerprintDetail records TLC's source stack and underlying cause, if any.
	FingerprintDetail string
	// FingerprintCause and FingerprintCauseCode describe a nested TLC message.
	FingerprintCause     string
	FingerprintCauseCode int
	// HeaderReported marks an error header emitted before a diagnostic replay.
	HeaderReported bool
	// EvaluationDetail records the expression stack for an unexpected failure.
	EvaluationDetail string
	// SuppressDepth matches TLC failures that report counts without search depth.
	SuppressDepth bool
	// TemporalViolation reports that TLC printed the start of a liveness
	// counterexample, whose states are TemporalTrace, before this failure.
	TemporalViolation bool
	TemporalTrace     []TraceStep
	evaluationSources []*parser.Expression
}
type model struct {
	checkpointIdentity   *checkpointIdentity
	checkpointDriver     *checkpointDriver
	coverage             *coverageCollector
	progress             *LiveProgress
	module               *parser.Module
	cfg                  config.Config
	descriptionConfig    config.Config
	sourceConfig         config.Config
	program              *evaluator.Program
	defs                 map[string]parser.Definition
	levelExpansions      map[*parser.Expression]*parser.Expression // Temporal validation only; nil in changed lexical scopes.
	variables            map[string]bool
	constants            map[string]value.Value
	trueConfigured       map[string]bool // Configured predicates already evaluated as constant TRUE.
	startupAttempts      map[string]bool
	startupFailures      map[string]bool
	symmetry             []permutation
	symmetryOrder        []string
	valueRanks           map[string]int
	sourceUnits          map[string]*parser.Module
	rankedSets           map[value.Value][]value.Value
	rankedFunctions      map[value.Value][]value.Pair
	names                []string
	ctx                  context.Context
	actionCache          map[*parser.Expression]*parser.Expression
	instanceActions      map[string]*parser.Expression // Sources of substituted instance actions.
	funcActionCache      map[*parser.Expression]parser.Definition
	letCache             map[*parser.Expression]*parser.Expression
	tlcFingerprint       bool
	initialProductBound  int
	fingerprintAlgorithm *value.TLCFingerprintAlgorithm
	fromSpecification    bool
	checkpointSpecSafe   bool
	tlcRuntime           *tlcRuntime
	enabledCallback      evaluator.EnabledCallback
	properties           []temporalProperty
	livenessOrder        []temporalProperty
	// propertyCalls counts the parameterized operators being expanded in a
	// property; TLC does not split a property below such a call.
	propertyCalls         int
	java17PureChecked     bool
	java17Pure            bool
	emptyTemporalProperty bool
	safety                []temporalProperty
	guarded               []temporalProperty
	fairness              []fairnessCondition
	possibleStates        []possiblePredicate
	possibleActions       []possiblePredicate
	debug                 func(string)
	graph                 func(GraphEvent) error
	livenessGraph         func(GraphEvent) error
	dotGraph              bool
	textGraph             bool
	searchEvent           func(SearchEvent)
	checkpointObserver    *checkpointObserver
	warning               func(Warning)
	livenessWarning       func(Warning) // Model checking only, as in Java TLC's AbstractChecker.
	viewStates            bool
	terseValues           bool // -terse: values print as Java TLC's unexpanded formulas.
	continueChecking      bool
	onContinuedViolation  func(Result)
	onFailureHeader       func(Failure)

	// stateKeyBuffer and valueKeyBuffer are reused to encode state keys.
	stateKeyBuffer, valueKeyBuffer []byte
	// symmetryPrefix is reused for permuted values during symmetry reduction.
	symmetryPrefix []value.Value
	symmetryState  evaluator.State
	sharedPartial  evaluator.State
	work           workArena
	unchangedCache map[*parser.Expression][]string
	// stateDependentCalls caches stateDependentCall by call expression.
	stateDependentCalls map[*parser.Expression]bool
	sharedPaths         map[sharedPathKey]*workPath
	fairAngles          map[[2]*parser.Expression]*parser.Expression
	stutterings         map[*parser.Expression]*parser.Expression
}

// ModuleLoader returns the source of an external module by its declared name.
// Return an error matching fs.ErrNotExist to allow standard-module fallback.
type ModuleLoader func(context.Context, string) (string, error)

// Options configures external module loading and TLC output.
type Options struct {
	// Coverage enables experimental action and expression coverage.
	Coverage bool
	// LiveProgress opts a coverage check into publishing concurrent snapshots.
	// Supply a fresh value for one check; see LiveProgress.
	LiveProgress *LiveProgress
	Loader       ModuleLoader
	OnPrint      func(string)
	OnPause      func()
	ReadFile     func(string) (string, error)
	WriteFile    func(string, string) error
	Seed         int64
	// Fingerprint selects a Java TLC FP64 polynomial (0 through 130).
	Fingerprint int
	// Simulation selects bounded single-worker random simulation when non-nil.
	Simulation *SimulationOptions
	// DepthFirst selects depth-first iterative deepening when non-nil and
	// Simulation is nil. In this mode QueuedStates is -1, Depth is the last
	// iteration level started, and OnGraph and OnDebug are not called.
	DepthFirst *DepthFirstOptions
	// Continue explores other states after invariant and action violations.
	// It applies to model checking; simulation uses SimulationOptions.Continue.
	Continue bool
	// OnViolation receives each continued model-checking violation synchronously.
	// Trace states are read-only and must be copied before modification.
	OnViolation func(Result)
	// OnFailureHeader receives an error before its diagnostic replay.
	// It is called synchronously; retained state values must not be modified.
	OnFailureHeader func(Failure)
	// ModernRandomSubsets uses the newer TLC random-subset algorithm.
	ModernRandomSubsets bool
	// PrintStyle selects how TLC operators render values.
	PrintStyle evaluator.PrintStyle
	// OnDebug receives JSON lines describing liveness exploration when set.
	OnDebug func(string)
	// OnGraph receives states and transitions synchronously during exploration.
	// State and Order must be copied by callers that retain them after the call.
	OnGraph func(GraphEvent) error
	// OnLivenessGraph receives synchronous events for a completed, pure,
	// single-solution non-tableau liveness graph without VIEW or symmetry.
	// This experimental callback is not invoked for every model. Callers that
	// retain State or Order must copy them, as with OnGraph.
	OnLivenessGraph func(GraphEvent) error
	// DOTGraph enables Java DOT writer fingerprint evaluation and rank events.
	// DOTGraph and TextGraph are mutually exclusive, including when OnGraph is nil.
	DOTGraph bool
	// TextGraph follows Java text dumps without computing graph fingerprints.
	TextGraph bool
	// OnSearch receives synchronous initial-state progress events when set.
	OnSearch func(SearchEvent)
	// OnTemporalCheck receives the completed liveness result; on a violation,
	// postconditions run between OnTemporalViolation and OnTemporalCheck.
	OnTemporalCheck func(Result)
	// OnTemporalViolation receives the counterexample before postconditions.
	OnTemporalViolation func(Result)
	// MaxSetSize is Java TLC's -maxSetSize bound on sets that are built element
	// by element. A nonpositive value selects TLC's default of 1,000,000.
	MaxSetSize int
	// Workers is the -workers count that model checking reports through
	// TLCGet("config"). Eligible BFS models can generate successors in parallel;
	// other models use serial exploration. A nonpositive value means 1.
	Workers int
	// SerialExploration disables speculative BFS generation while retaining the requested worker count in TLCGet.
	SerialExploration bool
	// ViewStates prints trace states through the configured VIEW, as TLC's
	// -view option does, by setting TraceStep.View.
	ViewStates bool
	// OnWarning receives Java TLC warnings. As in Java TLC, simulation reports
	// the symmetry and EXCEPT warnings 2300 and 2144 but not the model
	// checker's liveness warnings 2279 and 2284.
	OnWarning func(Warning)
}

// SimulationOptions bounds the number and length of simulated traces. A
// nonpositive Traces value runs one trace, and Depth -1 has no limit.
// After initial state generation, DistinctStates and QueuedStates are -1 and
// Depth is not populated; GeneratedStates counts states including repeats.
type SimulationOptions struct {
	Traces int
	Depth  int
	// OmitLassoPropertyNames skips replay used to identify properties violated by
	// a recovered lasso. Such violations have an empty Name and nil Properties.
	// Without ALIAS and with a compatible VIEW, graph leaves are evaluated without
	// caching. Other models retain the previous path and property names. A VIEW
	// with effects, TLCGet, unstable reads, or reachable higher-order operators
	// and operator constants is conservatively treated as incompatible.
	// This does not guarantee Java-compatible effects for every model. The default is false.
	OmitLassoPropertyNames bool
	// OnCounterexampleStart runs synchronously after a liveness counterexample
	// is found and before its states are recovered. Recovery may subsequently
	// fail. It is not called for finite safety violations. The default is nil.
	// It requires OmitLassoPropertyNames and the compatible ALIAS/VIEW conditions above.
	OnCounterexampleStart func()
	// OnCounterexampleStates receives the complete recovered prefix, then the
	// complete trace after each recovered cycle state. The slice, steps, state maps,
	// and StateOrder slices are read-only; copy them before modifying. Previously
	// returned prefixes may be retained and remain unchanged as recovery proceeds.
	// The second argument is the first new index.
	// A failed prefix is never reported; a failed cycle retains prior reports.
	// It requires OmitLassoPropertyNames and the compatible ALIAS/VIEW conditions above.
	OnCounterexampleStates func([]TraceStep, int)
	// OnBehaviorStart and at least one behavior state callback opt in together
	// to streaming after counterexample recovery fails. They run synchronously
	// and require OmitLassoPropertyNames and the compatible ALIAS/VIEW conditions above.
	// State numbers are one based. State maps and slices are read-only; copy them
	// before modifying. When both callbacks are set, OnBehaviorState runs first.
	OnBehaviorStart func(*Failure)
	OnBehaviorState func(TraceStep, int)
	// OnBehaviorStateWithPrevious also receives the previous sampled step,
	// including when it was suppressed by fingerprint equality. Its StateOrder,
	// ActionRange, and optional View are prepared like the current step. Previous
	// is nil for the initial state. It may be used instead of or alongside
	// OnBehaviorState; the same activation and read-only rules apply. Both steps
	// may be retained after the callback returns.
	OnBehaviorStateWithPrevious func(TraceStep, *TraceStep, int)
	// UnlimitedTraces removes the trace-count limit and reports -1 to TLCGet.
	UnlimitedTraces bool
	// Aril advances the master random stream by this many double draws.
	// Negative values have the same effect as zero, matching Java TLC.
	Aril int64
	// Continue resumes with a new trace after a safety violation or deadlock.
	// Initial-state violations, evaluation failures, and liveness violations stop.
	Continue bool
	// OnViolation receives each continued violation synchronously when Continue
	// is true. Trace states are read-only and must be copied before modification.
	// Without a callback, ContinuedViolations still records the count.
	OnViolation func(Result)
	// OnTrace receives each completed, nonviolating trace synchronously. The
	// index counts all attempted traces, including those with violations.
	// States are read-only and must be copied by callers that retain them.
	// Under Options.ViewStates, steps carry their VIEW text.
	OnTrace func(index int, trace []TraceStep) error
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

// ConfigurationError is a configuration failure reported by Java TLC.
type ConfigurationError struct {
	Message string
	Err     error
	// Code is the Java TLC message code when the failure has a known equivalent.
	Code int
}

func (e *ConfigurationError) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return e.Err.Error()
}
func (e *ConfigurationError) Unwrap() error { return e.Err }

// ModelValidationError describes a Java TLC model validation failure.
type ModelValidationError struct {
	Message  string
	Code     int
	ExitCode int
}

func (e *ModelValidationError) Error() string { return e.Message }

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

// CheckModuleWithLoader checks a parsed module and configuration. Use the module
// returned by parser.ParseModule directly: the JSON AST projection omits LET
// recursive declarations, so a JSON round trip loses their validation metadata.
func CheckModuleWithLoader(ctx context.Context, module *parser.Module, cfg config.Config, loader ModuleLoader) (Result, error) {
	if module == nil {
		return Result{}, fmt.Errorf("missing module")
	}
	return checkParsedWithOptions(ctx, module, cfg, Options{Loader: loader})
}

// CheckModuleWithOptions checks a parsed module with an output callback. As with
// CheckModuleWithLoader, a JSON round trip loses LET recursive declaration metadata.
func CheckModuleWithOptions(ctx context.Context, module *parser.Module, cfg config.Config, options Options) (Result, error) {
	if module == nil {
		return Result{}, fmt.Errorf("missing module")
	}
	return checkParsedWithOptions(ctx, module, cfg, options)
}

func checkParsedWithOptions(ctx context.Context, module *parser.Module, cfg config.Config, options Options) (Result, error) {
	if options.DOTGraph && options.TextGraph {
		return Result{}, fmt.Errorf("DOTGraph and TextGraph are mutually exclusive")
	}
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
	if session.builtinTLC {
		copy := *module
		copy.HasTLCInstance = true
		module = &copy
	}
	return checkParsedWithOutput(ctx, module, cfg, options, session.sourceUnits)
}

func checkParsed(ctx context.Context, module *parser.Module, cfg config.Config) (Result, error) {
	return checkParsedWithOutput(ctx, module, cfg, Options{}, nil)
}

func checkParsedWithOutput(ctx context.Context, module *parser.Module, cfg config.Config, options Options, sourceUnits map[string]*parser.Module) (Result, error) {
	model, err := prepareTraced(ctx, module, cfg, options, sourceUnits)
	if err != nil {
		return Result{}, err
	}
	model.ctx = ctx
	defer searchHeapCapture(ctx, model)()
	if err := validateCoverageMode(options); err != nil {
		return Result{}, err
	}
	defer traceExploration(ctx)()
	if options.Simulation != nil {
		result, err := model.simulateWithOptions(*options.Simulation, options.Seed)
		if err != nil {
			return result, err
		}
		model.finishSimulationResult(&result)
		return result, nil
	}
	if options.DepthFirst != nil {
		return model.checkDepthFirst(*options.DepthFirst)
	}
	if err := model.configureCoverage(options); err != nil {
		return Result{}, err
	}
	return model.checkWithCoverage(options)
}

func (model *model) checkWithCoverage(options Options) (result Result, err error) {
	model.configureLiveProgress(options.LiveProgress)
	completed := false
	if options.LiveProgress != nil {
		defer func() {
			if completed {
				options.LiveProgress.finishCoverage(result.Coverage)
			} else {
				options.LiveProgress.finishCoverage(nil)
			}
		}()
	}
	result, err = model.check()
	if err != nil {
		completed = true
		return result, err
	}
	result, err = model.finishCoverageCheck(result, options)
	completed = true
	return result, err
}

func traceExploration(ctx context.Context) func() {
	if !trace.IsEnabled() {
		return func() {}
	}
	return trace.StartRegion(ctx, "exploration and liveness").End
}

func prepareTraced(ctx context.Context, module *parser.Module, cfg config.Config, options Options, sourceUnits map[string]*parser.Module) (*model, error) {
	if trace.IsEnabled() {
		defer trace.StartRegion(ctx, "model initialization").End()
	}
	return prepareWithSources(module, cfg, options, sourceUnits)
}

func (model *model) finishCoverageCheck(result Result, options Options) (Result, error) {
	if model.coverage == nil || result.Status == Success || !model.coverage.hasExistential || !model.coverageReplayUnreliable(result) {
		model.attachCoverage(&result)
	}
	model.adjustContinuedEvaluationFailure(&result)
	return model.finalizeCheckResult(result, options.OnTemporalCheck, options.OnTemporalViolation)
}

func (model *model) coverageReplayUnreliable(result Result) bool {
	return result.Status == EvaluationFailure || model.coverage.existentialEvaluated() || result.Violation != nil && len(result.Violation.Trace) > 1
}

func (m *model) finalizeCheckResult(result Result, onTemporalCheck, onTemporalViolation func(Result)) (Result, error) {
	if m.needsViewRecovery(result) {
		if err := m.recoverTemporalView(&result); err != nil {
			return result, err
		}
	}
	m.presentTraces(&result)
	if reportTemporalViolation(result, onTemporalCheck, onTemporalViolation) {
		return m.finishTemporalViolation(result, onTemporalCheck, onTemporalViolation)
	}
	if result.TemporalCheckDuration != nil && onTemporalCheck != nil {
		onTemporalCheck(result)
		result.TemporalCheckReported = true
	}
	if m.skipPostconditions(result) {
		return result, nil
	}
	return m.checkPostconditions(result)
}

func (m *model) skipPostconditions(result Result) bool {
	return result.Status == AssumptionViolation || result.Status == EvaluationFailure || m.noStateSearch()
}

func (m *model) recoverTemporalView(result *Result) error {
	if err := m.recoverViewResult(result); err != nil {
		return err
	}
	return m.updateRecoveredTableauProperties(result)
}

func reportTemporalViolation(result Result, onTemporalCheck, onTemporalViolation func(Result)) bool {
	return result.TemporalCheckDuration != nil && result.Status == LivenessViolation && onTemporalCheck != nil && onTemporalViolation != nil
}

func (m *model) finishTemporalViolation(result Result, onTemporalCheck, onTemporalViolation func(Result)) (Result, error) {
	onTemporalViolation(result)
	updated, err := m.checkPostconditions(result)
	if err != nil {
		return updated, err
	}
	onTemporalCheck(updated)
	updated.TemporalCheckReported = true
	return updated, nil
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
	fingerprintAlgorithm, err := value.NewTLCFingerprintAlgorithm(options.Fingerprint)
	if err != nil {
		return nil, err
	}
	return prepareWithFingerprint(module, cfg, options, sourceUnits, fingerprintAlgorithm)
}

func prepareWithFingerprint(module *parser.Module, cfg config.Config, options Options, sourceUnits map[string]*parser.Module, fingerprintAlgorithm *value.TLCFingerprintAlgorithm) (*model, error) {
	var err error
	var fairness []fairnessCondition
	module, err = resolveTemporalExpressions(module)
	if err != nil {
		return nil, err
	}
	sourceConfig := cfg
	fromSpecification := cfg.Specification != ""
	if err = validateConfigTables(cfg); err != nil {
		return nil, err
	}
	cfg = discardIneffectiveScopedAssignments(module, cfg)
	module, cfg, err = applyValidatedConfigOverrides(module, cfg)
	if err != nil {
		return nil, err
	}
	descriptionConfig := cfg
	cfg, fairness, err = lowerSpecification(module, cfg)
	if err != nil {
		return nil, semanticBeforeConfigError(module, err)
	}
	cfg = defaultEmptyModel(module, cfg)
	if err = validate(module, cfg); err != nil {
		return nil, semanticBeforeConfigError(module, err)
	}
	if err = validateResolved(module); err != nil {
		return nil, err
	}
	runtime := newTLCRuntime(cfg, options)
	m := &model{ctx: context.Background(), module: module, cfg: cfg, descriptionConfig: descriptionConfig, sourceConfig: sourceConfig, fairness: fairness, fromSpecification: fromSpecification, tlcRuntime: runtime, program: newEvaluatorProgram(module, options.OnPrint, runtime), defs: map[string]parser.Definition{}, variables: map[string]bool{}, constants: map[string]value.Value{}, actionCache: map[*parser.Expression]*parser.Expression{}, letCache: map[*parser.Expression]*parser.Expression{}, debug: options.OnDebug, graph: options.OnGraph, dotGraph: options.DOTGraph, textGraph: options.TextGraph, searchEvent: options.OnSearch, warning: options.OnWarning, livenessWarning: modelWarning(options), viewStates: options.ViewStates, onFailureHeader: options.OnFailureHeader}
	m.configureContinuation(options)
	m.livenessGraph = options.OnLivenessGraph
	m.program.SetJSONFileSystem(options.ReadFile, options.WriteFile)
	m.program.SetBound(options.MaxSetSize)
	m.initialProductBound = options.MaxSetSize
	m.program.SetPrintStyle(options.PrintStyle)
	m.terseValues = options.PrintStyle.Terse
	m.program.SetWarningHandler(m.evaluationWarningHandler())
	m.sourceUnits = sourceUnits
	runtime.model = m
	m.enabledCallback = m.enabled
	m.fingerprintAlgorithm = fingerprintAlgorithm
	m.tlcFingerprint = useTLCFingerprint(module, cfg)
	if err := m.bindAndValidate(); err != nil {
		return nil, err
	}
	m.tlcRuntime.needsLevelGraph = m.temporalUsesTLC()
	return m, nil
}

// SANY resolves the module before TLC reports configuration model errors.
func semanticBeforeConfigError(module *parser.Module, err error) error {
	if _, ok := errors.AsType[*ConfigurationError](err); ok {
		if semantic := validateResolved(module); semantic != nil {
			return semantic
		}
	}
	return err
}

func (m *model) bindAndValidate() error {
	m.prepareValueOrdering()
	m.program.SetOrdering(modelOrdering{model: m})
	if err := m.bindModel(); err != nil {
		return err
	}
	m.prepareRandomConstants()
	if err := m.validateConstantSubstitutions(); err != nil {
		return err
	}
	if err := m.prepareSymmetry(); err != nil {
		return err
	}
	m.precomputeEffectfulDefinitions()
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
	if usesTLCRuntime(module) {
		return evaluator.NewWithRuntime(module, onPrint, runtime)
	}
	return evaluator.NewWithPrint(module, onPrint)
}

func usesTLCRuntime(module *parser.Module) bool {
	return module.HasTLCInstance || hasTLCRuntimeModule(module.Extends) || hasTLCRuntimeModule(module.InstanceStandardModules)
}

func hasTLCRuntimeModule(modules []string) bool {
	return slices.Contains(modules, "TLC") || slices.Contains(modules, "TLCExt") || slices.Contains(modules, "Randomization")
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
	if err := m.validateInvariantLevels(); err != nil {
		return err
	}
	if err := m.validateDefinitions(); err != nil {
		return err
	}
	if err := m.validateEnabledLevels(); err != nil {
		return err
	}
	if err := m.validatePredicates(); err != nil {
		return err
	}
	if err := m.prepareFairness(); err != nil {
		return err
	}
	if err := m.prepareConfiguredProperties(); err != nil {
		return err
	}
	if err := m.validateNextRequiredForChecks(); err != nil {
		return err
	}
	return m.validatePossibleAndStateDependence()
}

// modelWarning returns the callback for the liveness warnings of Java TLC's
// AbstractChecker, which its simulator does not report.
func modelWarning(options Options) func(Warning) {
	if options.Simulation != nil {
		return nil
	}
	return options.OnWarning
}

func (m *model) prepareConfiguredProperties() error {
	if err := m.validateConstantPredicates(); err != nil {
		return err
	}
	err := m.prepareProperties()
	if failure, ok := errors.AsType[*ModelValidationError](err); ok && failure.Code == 2253 {
		// Java TLC's AbstractChecker warns before LiveCheck rejects a tautology.
		m.emitLivenessWarnings()
	}
	return err
}

func (m *model) validatePossibleAndStateDependence() error {
	if err := m.preparePossible(); err != nil {
		return err
	}
	return m.validateStateDependence()
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
	if err := m.validateConfiguredConstants(); err != nil {
		return err
	}
	if err := m.validateConfiguredStateOperators(); err != nil {
		return err
	}
	if err := m.validateConfiguredChecks(); err != nil {
		return err
	}
	for _, name := range m.cfg.Postconditions {
		if _, err := m.definition(name); err != nil {
			return err
		}
	}
	return nil
}

func (m *model) validateConfiguredChecks() error {
	for _, item := range []struct {
		kind  string
		names []string
	}{{"property", m.cfg.Properties}, {"constraint", m.cfg.Constraints}, {"action constraint", m.cfg.ActionConstraints}} {
		if err := m.validateConfiguredOperators(item.kind, item.names); err != nil {
			return err
		}
	}
	if m.cfg.View != "" {
		if err := m.validateConfiguredOperators("view function", []string{m.cfg.View}); err != nil {
			return err
		}
	}
	return nil
}

func (m *model) definition(name string) (*parser.Expression, error) {
	d, ok := m.defs[name]
	if !ok {
		if _, exists := m.constants[name]; exists {
			return &parser.Expression{Kind: "Identifier", Text: name}, nil
		}
	}
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
	if err != nil {
		return false, err
	}
	if b, ok := v.Bool(); ok {
		return b, nil
	}
	return false, m.nonBooleanPredicateError(expr, v)
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

// searchHeapCapture does not retain the model when profiling is disabled.
func searchHeapCapture(ctx context.Context, m *model) func() {
	if !heapcapture.Enabled(ctx) {
		return func() {}
	}
	return func() { captureSearchHeap(ctx, m) }
}

// captureSearchHeap runs only after exploration has stopped, before the model
// and its search graph become unreachable. It does not affect callbacks.
func captureSearchHeap(ctx context.Context, m *model) {
	if m.tlcRuntime.search != nil || m.tlcRuntime.depthFirst != nil {
		heapcapture.Capture(ctx)
		runtime.KeepAlive(m)
	}
}
