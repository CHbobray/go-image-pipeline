// Package timing summarizes repeated measurements of the same workload.
//
// Timing a program once gives a noisy number; timing it many times and
// treating each run as a random sample (a Monte Carlo benchmark) shows both
// the typical time and how much it varies. The package has no dependencies
// on the rest of this project and can be reused elsewhere.
package timing

import (
	"math"
	"slices"
	"time"
)

// Stats describes a sample of run durations, in milliseconds.
type Stats struct {
	Runs     int     `json:"runs"`
	MeanMs   float64 `json:"mean_ms"`
	StdMs    float64 `json:"std_ms"` // sample standard deviation
	MinMs    float64 `json:"min_ms"`
	MedianMs float64 `json:"median_ms"`
	P95Ms    float64 `json:"p95_ms"`
	MaxMs    float64 `json:"max_ms"`
}

// Summarize computes Stats for durations. It returns the zero Stats for an
// empty sample.
func Summarize(durations []time.Duration) Stats {
	if len(durations) == 0 {
		return Stats{}
	}
	ms := make([]float64, len(durations))
	for i, d := range durations {
		ms[i] = float64(d) / float64(time.Millisecond)
	}
	slices.Sort(ms)

	mean := sum(ms) / float64(len(ms))
	return Stats{
		Runs:     len(ms),
		MeanMs:   mean,
		StdMs:    sampleStd(ms, mean),
		MinMs:    ms[0],
		MedianMs: median(ms),
		P95Ms:    ms[min(len(ms)-1, int(0.95*float64(len(ms))))],
		MaxMs:    ms[len(ms)-1],
	}
}

// PerSecond converts a mean run time into a rate, e.g. images per second
// when each run processes itemsPerRun images. It returns 0 for a zero mean.
func (s Stats) PerSecond(itemsPerRun int) float64 {
	if s.MeanMs <= 0 {
		return 0
	}
	return float64(itemsPerRun) / (s.MeanMs / 1000)
}

func sum(values []float64) float64 {
	total := 0.0
	for _, v := range values {
		total += v
	}
	return total
}

func sampleStd(values []float64, mean float64) float64 {
	if len(values) < 2 {
		return 0
	}
	squares := 0.0
	for _, v := range values {
		squares += (v - mean) * (v - mean)
	}
	return math.Sqrt(squares / float64(len(values)-1))
}

func median(sorted []float64) float64 {
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}
