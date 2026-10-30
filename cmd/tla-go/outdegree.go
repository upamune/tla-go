package main

import "fmt"

func outDegreeSummary(buckets []int) string {
	observations, total, minimum, maximum := 0, 0, -1, 0
	for degree, count := range buckets {
		observations += count
		total += degree * count
		if count > 0 {
			if minimum < 0 {
				minimum = degree
			}
			maximum = degree
		}
	}
	if observations == 0 {
		return ""
	}
	percentile := outDegreePercentile(buckets, observations*95/100)
	average := (total + observations/2) / observations
	return fmt.Sprintf("The average outdegree of the complete state graph is %d (minimum is %d, the maximum %d and the 95th percentile is %d).", average, minimum, maximum, percentile)
}

func outDegreePercentile(buckets []int, position int) int {
	cumulative := 0
	for degree, count := range buckets {
		cumulative += count
		if cumulative > position {
			return degree
		}
	}
	return 0
}
