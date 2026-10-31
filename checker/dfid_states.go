package checker

import (
	"math"
	"slices"
)

// dfidStates follows TLC's MemFPIntSet. Each status packs the iteration that
// first found a state with a leveled bit, whose meaning alternates between
// iterations, and a done bit for states whose successors are all done.
type dfidStates struct {
	status       map[string]int32
	fingerprints []uint64
	level        int32
	leveled      int32
}

const (
	dfidNew     int32 = 0
	dfidDone    int32 = 1
	dfidLeveled int32 = 2
)

func newDFIDStates() dfidStates {
	return dfidStates{status: map[string]int32{}, level: 1}
}

// nextLevel is FPIntSet.incLevel, which starts an iteration.
func (s *dfidStates) nextLevel() {
	s.level++
	s.leveled = dfidLeveled - s.leveled
}

// insert is FPIntSet.setStatus(fp, NEW): it returns the previous status, or
// dfidNew after recording a state found in the current iteration.
func (s *dfidStates) insert(key string) (int32, bool) {
	if status, ok := s.status[key]; ok {
		return status, false
	}
	s.status[key] = s.level<<2 | s.leveled
	return dfidNew, true
}

func (s *dfidStates) get(key string) int32 { return s.status[key] }

func (s *dfidStates) setLeveled(key string) {
	s.status[key] = s.status[key]&^dfidLeveled | s.leveled
}

func (s *dfidStates) setDone(key string) { s.status[key] |= dfidDone }

// completed reports a state that needs no more exploration in this iteration.
func (s *dfidStates) completed(status int32) bool {
	return status&dfidLeveled == s.leveled || status&dfidDone != 0
}

// leaf reports a successor found in the current iteration.
func (s *dfidStates) leaf(status int32) bool {
	return status == dfidNew || status>>2 == s.level
}

func dfidLevel(status int32) int { return int(status >> 2) }

func dfidIsDone(status int32) bool { return status&dfidDone != 0 }

// dfidFingerprintDistance reproduces MemFPIntSet.checkFPs, whose minimum
// distance between fingerprints yields TLC's observed collision probability.
// Like TLC, it sign-extends the low word of each fingerprint, compares pairs in
// different hash buckets only when their difference does not overflow, and
// wraps differences within a bucket like Math.abs.
func dfidFingerprintDistance(fingerprints []uint64) int64 {
	distance := int64(math.MaxInt64)
	values := make([]int64, len(fingerprints))
	buckets := map[uint64]dfidBucket{}
	mask := dfidTableMask(len(fingerprints))
	for i, fp := range fingerprints {
		values[i] = int64(int32(fp>>32))<<32 | int64(int32(fp))
		buckets[fp&mask] = buckets[fp&mask].add(values[i])
	}
	slices.Sort(values)
	for i := 1; i < len(values); i++ {
		if gap := values[i] - values[i-1]; gap >= 0 {
			distance = min(distance, gap)
		}
	}
	for _, bucket := range buckets {
		distance = min(distance, bucket.wrappedDistance())
	}
	return distance
}

// dfidBucket keeps the extremes of one MemFPIntSet bucket.
type dfidBucket struct {
	low, high int64
	count     int
}

func (b dfidBucket) add(v int64) dfidBucket {
	if b.count == 0 {
		return dfidBucket{low: v, high: v, count: 1}
	}
	return dfidBucket{low: min(b.low, v), high: max(b.high, v), count: b.count + 1}
}

// wrappedDistance covers pairs whose difference overflows: the widest pair
// of a bucket wraps to the smallest Math.abs value. Other pairs are no closer
// than neighbors in sorted order.
func (b dfidBucket) wrappedDistance() int64 {
	if b.count < 2 {
		return math.MaxInt64
	}
	gap := b.high - b.low
	if gap < 0 {
		gap = -gap
	}
	return gap
}

// dfidTableMask follows MemFPIntSet's growth from 2^16 buckets with a load of 20.
func dfidTableMask(count int) uint64 {
	capacity, threshold := uint64(1)<<16, 20<<16
	for count > threshold {
		capacity, threshold = capacity*2, threshold*2
	}
	return capacity - 1
}
