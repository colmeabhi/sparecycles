package main

import (
	"math"
	"sort"
	"time"
)

// summary describes one load run.
type summary struct {
	Jobs       int
	Failed     int
	Units      int
	Total      time.Duration // wall time from the first submit to the last result
	Mean       time.Duration // mean job time, submit to result
	P95        time.Duration // 95th percentile job time
	JobsPerMin float64
	UnitsPerS  float64
}

// summarize computes the summary from per-job durations of the successful jobs.
func summarize(durations []time.Duration, failed, units int, total time.Duration) summary {
	s := summary{Jobs: len(durations) + failed, Failed: failed, Units: units, Total: total}
	if len(durations) > 0 {
		s.Mean = mean(durations)
		s.P95 = percentile(durations, 0.95)
	}
	if total > 0 {
		s.JobsPerMin = float64(len(durations)) / total.Minutes()
		s.UnitsPerS = float64(units) / total.Seconds()
	}
	return s
}

func mean(d []time.Duration) time.Duration {
	var sum time.Duration
	for _, x := range d {
		sum += x
	}
	return sum / time.Duration(len(d))
}

// percentile returns the nearest-rank percentile (p in 0..1) of d. d is not changed.
func percentile(d []time.Duration, p float64) time.Duration {
	sorted := append([]time.Duration(nil), d...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	rank := int(math.Ceil(p * float64(len(sorted))))
	if rank < 1 {
		rank = 1
	}
	return sorted[rank-1]
}
