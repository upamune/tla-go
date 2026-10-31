package checker

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/upamune/tla-go/value"
)

func makeRecord(fields map[string]value.Value) (value.Value, error) {
	pairs := make([]value.Pair, 0, len(fields))
	for key, val := range fields {
		pairs = append(pairs, value.Pair{Key: value.String(key), Value: val})
	}
	return value.Function(pairs)
}

func unnamedAction() (value.Value, error) {
	zero, _ := value.Integer(0)
	location, err := makeRecord(map[string]value.Value{"beginLine": zero, "beginColumn": zero, "endLine": zero, "endColumn": zero, "module": value.String("--TLA+ BUILTINS--")})
	if err != nil {
		return value.Value{}, err
	}
	return makeRecord(map[string]value.Value{"name": value.String("UnnamedAction"), "location": location})
}

func (r *tlcRuntime) allRegisters() (value.Value, error) {
	pairs := make([]value.Pair, 0, len(r.registers))
	for key, val := range r.registers {
		k, _ := value.Integer(int64(key))
		wrapped, err := value.Tuple([]value.Value{val})
		if err != nil {
			return value.Value{}, err
		}
		pairs = append(pairs, value.Pair{Key: k, Value: wrapped})
	}
	return value.Function(pairs)
}

func (r *tlcRuntime) allNamedRegisters() (value.Value, error) {
	pairs := make([]value.Pair, 0, len(r.named))
	for key, val := range r.named {
		if key == "s:_possible" && len(r.possibleCounts) > 0 {
			continue
		}
		wrapped, err := value.Tuple([]value.Value{val})
		if err != nil {
			return value.Value{}, err
		}
		pairs = append(pairs, value.Pair{Key: value.String(key), Value: wrapped})
	}
	if len(r.possibleCounts) > 0 {
		pair, err := r.possibleCountRegister()
		if err != nil {
			return value.Value{}, err
		}
		pairs = append(pairs, pair)
	}
	return value.Function(pairs)
}

func (r *tlcRuntime) possibleCountRegister() (value.Pair, error) {
	counts, err := r.possibleCountValue()
	if err != nil {
		return value.Pair{}, err
	}
	wrapped, err := value.Tuple([]value.Value{counts})
	if err != nil {
		return value.Pair{}, err
	}
	return value.Pair{Key: value.String("s:_possible"), Value: wrapped}, nil
}

func (r *tlcRuntime) configValue() (value.Value, error) {
	if !r.started {
		return value.Value{}, tlcRuntimeError("TLCGet(config) was undefined.")
	}
	if r.simulation {
		return r.simulationConfigValue()
	}
	one, _ := value.Integer(1)
	return makeRecord(map[string]value.Value{"mode": value.String("bfs"), "deadlock": value.Bool(r.cfg.CheckDeadlock), "worker": one, "seed": value.String(strconv.FormatInt(r.options.Seed, 10)), "fingerprint": value.String(strconv.FormatInt(r.model.fingerprintAlgorithm.Polynomial(), 10)), "install": value.String("unknown")})
}

func (r *tlcRuntime) simulationConfigValue() (value.Value, error) {
	opts := r.options.Simulation
	traces := opts.Traces
	if opts.UnlimitedTraces {
		traces = -1
	}
	traceValue, err := value.Integer(int64(traces))
	if err != nil {
		return value.Value{}, err
	}
	depth := opts.Depth
	if depth >= math.MaxInt32 {
		depth = -1
	}
	depthValue, err := value.Integer(int64(depth))
	if err != nil {
		return value.Value{}, err
	}
	one, _ := value.Integer(1)
	pairs := []value.Pair{
		{Key: value.String("install"), Value: value.String("unknown")},
		{Key: value.String("mode"), Value: value.String("simulate")},
		{Key: value.String("deadlock"), Value: value.Bool(r.cfg.CheckDeadlock)},
		{Key: value.String("seed"), Value: value.String(strconv.FormatInt(r.options.Seed, 10))},
		{Key: value.String("worker"), Value: one},
		{Key: value.String("traces"), Value: traceValue},
		{Key: value.String("depth"), Value: depthValue},
		{Key: value.String("aril"), Value: value.String("0")},
		{Key: value.String("sched"), Value: value.String("random")},
	}
	return value.FunctionWithFieldOrder(pairs, []string{"install", "mode", "deadlock", "seed", "worker", "traces", "depth", "aril", "sched"})
}

func (r *tlcRuntime) revisionValue() (value.Value, error) {
	now := time.Now().UTC()
	stamp, err := value.Integer(now.Unix())
	if err != nil {
		return value.Value{}, err
	}
	zero, _ := value.Integer(0)
	date := fmt.Sprintf("%s.%dZ", now.Format("2006-01-02T15:04:05"), now.Nanosecond()/1_000_000)
	return makeRecord(map[string]value.Value{"count": zero, "timestamp": stamp, "date": value.String(date), "tag": value.String("development"), "calver": value.String(r.startedAt.UTC().Format("2006.01.02.150405"))})
}

func (r *tlcRuntime) possibleCountValue() (value.Value, error) {
	pairs := make([]value.Pair, 0, len(r.possibleCounts))
	for name, count := range r.possibleCounts {
		pairs = append(pairs, value.Pair{Key: value.String(name), Value: tlcInt(count)})
	}
	return value.Function(pairs)
}
