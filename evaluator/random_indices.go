package evaluator

import "sync"

var randomPrimes struct {
	once sync.Once
	data []int64
}

type subsetParameters struct {
	modulus, multiplier int64
}

func subsetIndices(count int, size int64, random *JavaRandom) []int64 {
	if !random.modernSubsets {
		return java174SubsetIndices(count, size, random)
	}
	return modernSubsetIndices(count, size, random)
}

// TLC 1.7.4's SubsetEnumerator uses a fixed prime as the stride. At the
// maximum int size the stride equals the modulus, so all picks coincide.
func java174SubsetIndices(count int, size int64, random *JavaRandom) []int64 {
	if size == 0 {
		return nil
	}
	start := int64(random.Int(int32(size)))
	stride := int64(2147483647)
	if size < 191 {
		stride = 191
	}
	indices := make([]int64, count)
	for i := range indices {
		indices[i] = (int64(i)*stride + start) % size
	}
	return indices
}

func modernSubsetIndices(count int, size int64, random *JavaRandom) []int64 {
	if size == 0 {
		return nil
	}
	params := random.subsetParameters(size)
	modulus, multiplier := params.modulus, params.multiplier
	index := int64(random.Int(int32(size)))
	increment := randomPrime(random)
	indices := make([]int64, count)
	for i := range indices {
		for {
			index = (multiplier*index + increment) % modulus
			if index < size {
				break
			}
		}
		indices[i] = index
	}
	return indices
}

func (r *JavaRandom) subsetParameters(size int64) subsetParameters {
	if params, ok := r.subsetCache[size]; ok {
		return params
	}
	modulus, multiplier := subsetModulus(size)
	params := subsetParameters{modulus, multiplier}
	if r.subsetCache == nil {
		r.subsetCache = make(map[int64]subsetParameters)
	}
	r.subsetCache[size] = params
	return params
}

func subsetModulus(size int64) (int64, int64) {
	for modulus := max(size, 9); ; modulus++ {
		multiplier, repeated := subsetMultiplier(modulus)
		if modulus%4 != 0 && repeated {
			return modulus, multiplier
		}
	}
}

func subsetMultiplier(n int64) (int64, bool) {
	product := int64(1)
	repeated := false
	for divisor := int64(2); divisor*divisor <= n; divisor++ {
		if n%divisor != 0 {
			continue
		}
		product *= divisor
		n /= divisor
		for n%divisor == 0 {
			repeated = true
			n /= divisor
		}
	}
	if n > 1 {
		product *= n
	}
	return product + 1, repeated
}

func randomPrime(random *JavaRandom) int64 {
	randomPrimes.once.Do(initRandomPrimes)
	return randomPrimes.data[int(random.Float64()*float64(len(randomPrimes.data)))]
}

func initRandomPrimes() {
	for candidate := int64(1277011); len(randomPrimes.data) < 1341; candidate++ {
		if randomIsPrime(candidate) {
			randomPrimes.data = append(randomPrimes.data, candidate)
		}
	}
}

func randomIsPrime(n int64) bool {
	for divisor := int64(2); divisor*divisor <= n; divisor++ {
		if n%divisor == 0 {
			return false
		}
	}
	return true
}
