package checker

import (
	"time"

	"github.com/upamune/tla-go/value"
)

type tlcCounts struct{ generated, distinct, queue, diameter, initial int }

func isStatisticsKey(name string) bool {
	switch name {
	case "generated", "distinct", "queue", "diameter", "stats":
		return true
	default:
		return false
	}
}

func (r *tlcRuntime) statisticsValue(name string) (value.Value, error) {
	counts := r.counts()
	switch name {
	case "generated":
		return value.Integer(int64(counts.generated))
	case "distinct":
		return value.Integer(int64(counts.distinct))
	case "queue":
		return value.Integer(int64(counts.queue))
	case "diameter":
		return value.Integer(int64(counts.diameter))
	default:
		return r.statisticsRecord(counts)
	}
}

func (r *tlcRuntime) counts() tlcCounts {
	counts := tlcCounts{initial: r.initial, diameter: 1}
	if r.depthFirst != nil {
		// TLC's depth-first checker does not track these statistics.
		return tlcCounts{generated: r.depthFirst.generated, distinct: -1, queue: -1, diameter: -1, initial: r.initial}
	}
	if r.search != nil {
		result := r.search.result
		counts.generated, counts.distinct, counts.queue, counts.diameter = result.GeneratedStates, result.DistinctStates, result.QueuedStates, max(result.Depth, 1)
	}
	return counts
}

func (r *tlcRuntime) statisticsRecord(counts tlcCounts) (value.Value, error) {
	return makeRecord(map[string]value.Value{"generated": tlcInt(counts.generated), "distinct": tlcInt(counts.distinct), "queue": tlcInt(counts.queue), "diameter": tlcInt(counts.diameter), "initial": tlcInt(counts.initial), "duration": tlcInt(int(time.Since(r.startedAt).Seconds())), "worker": tlcInt(0)})
}

func tlcInt(n int) value.Value {
	v, _ := value.Integer(int64(n))
	return v
}
