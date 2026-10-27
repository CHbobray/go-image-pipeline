package timing

import (
	"math"
	"testing"
	"time"
)

func TestSummarizeKnownValues(t *testing.T) {
	durations := []time.Duration{40 * time.Millisecond, 10 * time.Millisecond, 30 * time.Millisecond, 20 * time.Millisecond}
	stats := Summarize(durations)

	checks := map[string][2]float64{
		"mean":   {stats.MeanMs, 25},
		"median": {stats.MedianMs, 25},
		"min":    {stats.MinMs, 10},
		"max":    {stats.MaxMs, 40},
		"p95":    {stats.P95Ms, 40},
		"std":    {stats.StdMs, math.Sqrt(500.0 / 3)},
		"rate":   {stats.PerSecond(4), 160},
	}
	for name, pair := range checks {
		if math.Abs(pair[0]-pair[1]) > 1e-9 {
			t.Errorf("%s: got %v, want %v", name, pair[0], pair[1])
		}
	}
	if stats.Runs != 4 {
		t.Errorf("runs: got %d, want 4", stats.Runs)
	}
}

func TestSummarizeSingleAndEmpty(t *testing.T) {
	single := Summarize([]time.Duration{5 * time.Millisecond})
	if single.MeanMs != 5 || single.StdMs != 0 || single.MedianMs != 5 {
		t.Errorf("single sample: %+v", single)
	}
	if empty := Summarize(nil); empty != (Stats{}) || empty.PerSecond(10) != 0 {
		t.Errorf("empty sample should give zero stats, got %+v", empty)
	}
}
