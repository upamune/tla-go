package evaluator

// JavaRandom implements java.util.Random's 48-bit generator, which TLC uses.
type JavaRandom struct {
	state         uint64
	subsetCache   map[int64]subsetParameters
	modernSubsets bool
}

// SetModernSubsets selects the newer TLC random-subset enumeration algorithm.
func (r *JavaRandom) SetModernSubsets(enabled bool) { r.modernSubsets = enabled }

const (
	javaMultiplier uint64 = 0x5deece66d
	javaSeedMask   uint64 = 1<<48 - 1
)

func NewJavaRandom(seed int64) *JavaRandom {
	r := &JavaRandom{}
	r.SetSeed(seed)
	return r
}

func (r *JavaRandom) SetSeed(seed int64) {
	r.state = (uint64(seed) ^ javaMultiplier) & javaSeedMask
}

func (r *JavaRandom) next(bits uint) uint32 {
	r.state = (r.state*javaMultiplier + 11) & javaSeedMask
	return uint32(r.state >> (48 - bits))
}

// Int returns the next uniformly distributed integer in [0, bound).
func (r *JavaRandom) Int(bound int32) int32 {
	mask := bound - 1
	if bound&mask == 0 {
		return int32((int64(bound) * int64(r.next(31))) >> 31)
	}
	for {
		candidate := int32(r.next(31))
		result := candidate % bound
		if int64(candidate)-int64(result)+int64(mask) <= 1<<31-1 {
			return result
		}
	}
}

// Long returns the next signed 64-bit integer.
func (r *JavaRandom) Long() int64 {
	return int64(int32(r.next(32)))<<32 + int64(int32(r.next(32)))
}

// Float64 returns the next double in [0, 1), using Java's 53-bit construction.
func (r *JavaRandom) Float64() float64 {
	return float64(uint64(r.next(26))<<27|uint64(r.next(27))) / (1 << 53)
}
