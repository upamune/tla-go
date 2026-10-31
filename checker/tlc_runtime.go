package checker

import (
	"fmt"
	"strings"
	"time"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/value"
)

type tlcRuntime struct {
	registers          map[int32]value.Value
	named              map[string]value.Value
	properties         map[string]string
	possibleCounts     map[string]int
	cfg                config.Config
	options            Options
	startedAt          time.Time
	started, stopped   bool
	search             *search
	traceCurrent       int
	traceCandidate     evaluator.State
	traceIndices       map[uintptr]int
	viewKeys           []string
	viewLivenessSeen   []bool
	viewLastCandidate  evaluator.State
	viewCandidateLevel int
	traceOverride      []evaluator.State
	aliasEvaluating    bool
	counterExample     *Violation
	model              *model
	initial, level     int
	levelGraph         map[uintptr]tlcGraphLevel
	needsLevelGraph    bool
	useBaseLevel       bool
	outputMuted        bool
	viewTraceReplayed  bool
	startupRandom      *evaluator.JavaRandom
	workerRandom       *evaluator.JavaRandom
	randomIndex        int
	randomOrder        []string
	randomStartup      bool
	simulation         bool
}

func newTLCRuntime(cfg config.Config, options Options) *tlcRuntime {
	seed := options.Seed
	if options.Simulation != nil {
		seed = 0
	}
	startup := evaluator.NewJavaRandom(seed)
	worker := evaluator.NewJavaRandom(seed)
	startup.SetModernSubsets(options.ModernRandomSubsets)
	worker.SetModernSubsets(options.ModernRandomSubsets)
	return &tlcRuntime{cfg: cfg, options: options, startedAt: time.Now(), traceCurrent: -1, randomIndex: -1, startupRandom: startup, workerRandom: worker, simulation: options.Simulation != nil}
}

func (r *tlcRuntime) Get(key value.Value, context evaluator.Context, source evaluator.State, primed bool) (value.Value, error) {
	if n, ok := key.Int(); ok && n >= 0 {
		v, found := r.registers[n]
		if !found {
			return value.Value{}, tlcRuntimeError(fmt.Sprintf("TLCGet(%d) was undefined.", n))
		}
		return v, nil
	}
	name, ok := key.Text()
	if !ok || key.Kind() != value.StringKind {
		return value.Value{}, tlcRuntimeError(fmt.Sprintf("The argument of TLCGet should be a nonnegative integer, but instead it is:\n%s", key))
	}
	return r.getNamed(name, context, source, primed)
}

func (r *tlcRuntime) getNamed(name string, context evaluator.Context, source evaluator.State, primed bool) (value.Value, error) {
	switch name {
	case "action":
		return unnamedAction()
	case "level":
		return value.Integer(int64(r.contextLevel(context, source, primed)))
	case "duration":
		return value.Integer(int64(time.Since(r.startedAt).Seconds()))
	case "all":
		return r.allRegisters()
	case "all:named":
		return r.allNamedRegisters()
	case "config":
		return r.configValue()
	case "revision":
		return r.revisionValue()
	default:
		return r.getOther(name)
	}
}

func (r *tlcRuntime) getOther(name string) (value.Value, error) {
	if name == "spec" {
		return r.specificationValue()
	}
	if isStatisticsKey(name) {
		if !r.started {
			return value.Value{}, tlcRuntimeError(fmt.Sprintf("TLCGet(%s) was undefined.", name))
		}
		return r.statisticsValue(name)
	}
	return r.getRegister(name)
}

func (r *tlcRuntime) getRegister(name string) (value.Value, error) {
	if name == "s:_possible" && len(r.possibleCounts) > 0 {
		return r.possibleCountValue()
	}
	if strings.HasPrefix(name, "s:") {
		if v, ok := r.named[name]; ok {
			return v, nil
		}
	}
	if strings.HasPrefix(name, "-D") {
		if v, ok := r.properties[name[2:]]; ok {
			return value.String(v), nil
		}
		return value.String(name), nil
	}
	return value.Value{}, tlcRuntimeError(fmt.Sprintf("TLCGet(%s) was undefined.", name))
}

func (r *tlcRuntime) Set(key, val value.Value, context evaluator.Context) error {
	if n, ok := key.Int(); ok && n >= 0 {
		if r.registers == nil {
			r.registers = make(map[int32]value.Value)
		}
		r.registers[n] = val
		return nil
	}
	name, ok := key.Text()
	if !ok || key.Kind() != value.StringKind {
		return tlcSetKeyError(key)
	}
	return r.setNamed(name, key, val, context)
}

func (r *tlcRuntime) setNamed(name string, key, val value.Value, context evaluator.Context) error {
	switch {
	case name == "exit":
		if b, ok := val.Bool(); ok && b {
			r.stopped = true
		}
	case name == "pause":
		r.pause(val, context)
	case strings.HasPrefix(name, "s:"):
		r.setNamedRegister(name, val)
	case strings.HasPrefix(name, "-D"):
		r.setProperty(name[2:], val)
	default:
		return tlcSetKeyError(key)
	}
	return nil
}

func (r *tlcRuntime) setNamedRegister(name string, val value.Value) {
	if r.named == nil {
		r.named = make(map[string]value.Value)
	}
	r.named[name] = val
}

func (r *tlcRuntime) setProperty(name string, val value.Value) {
	if r.properties == nil {
		r.properties = make(map[string]string)
	}
	r.properties[name] = val.String()
}

func (r *tlcRuntime) pause(val value.Value, context evaluator.Context) {
	if r.simulation {
		return
	}
	if b, ok := val.Bool(); !ok || !b {
		return
	}
	if context.OnPrint != nil {
		context.OnPrint("Press enter to resume model checking.")
	}
	if r.options.OnPause != nil {
		r.options.OnPause()
	}
}

func tlcSetKeyError(key value.Value) error {
	return tlcRuntimeError(fmt.Sprintf("The first argument of TLCSet should be a nonnegative integer, but instead it is:\n%s", key))
}

func tlcRuntimeError(message string) error {
	return &evaluator.RuntimeError{Message: message, Standard: true}
}

func (r *tlcRuntime) GetRegister(key value.Value) (value.Value, bool) {
	if n, ok := key.Int(); ok {
		v, found := r.registers[n]
		return v, found
	}
	if name, ok := key.Text(); ok && key.Kind() == value.StringKind && strings.HasPrefix(name, "s:") {
		v, found := r.named[name]
		return v, found
	}
	return value.Value{}, false
}
