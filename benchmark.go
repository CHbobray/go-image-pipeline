package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"runtime"
	"slices"
	"time"

	"github.com/CHbobray/go-image-pipeline/pipeline"
	"github.com/CHbobray/go-image-pipeline/timing"
)

var errOutputsDiffer = errors.New("modes produced different output files")

// modeReport is one mode's benchmark result, as written to results/timing.json.
type modeReport struct {
	Mode            pipeline.Mode `json:"mode"`
	Timing          timing.Stats  `json:"timing"`
	ImagesPerSecond float64       `json:"images_per_second"`
	Speedup         float64       `json:"speedup_vs_sequential,omitempty"`
	AllocMBPerRun   float64       `json:"alloc_mb_per_run"`
	AllocsPerRun    float64       `json:"allocs_per_run"`
}

// checkResult summarizes the untimed check run of every mode.
type checkResult struct {
	Results  []pipeline.Result // per-image results of the first mode
	Failures int               // images that failed in the first mode
}

// checkModes runs each mode once, reports failures, and verifies that every
// mode wrote byte-identical output files.
func checkModes(cfg config) (checkResult, error) {
	var check checkResult
	var reference map[string][32]byte
	for _, mode := range cfg.Modes {
		results, err := pipeline.Run(mode, cfg.Tasks, cfg.Options)
		if err != nil {
			return check, err
		}
		digests := outputDigests(results)
		if reference == nil {
			reference = digests
			check.Results = results
			check.Failures = countFailures(results)
			continue
		}
		if !sameDigests(reference, digests) {
			return check, fmt.Errorf("%w: %s does not match %s", errOutputsDiffer, mode, cfg.Modes[0])
		}
	}
	return check, nil
}

// outputDigests hashes each successfully written output file.
func outputDigests(results []pipeline.Result) map[string][32]byte {
	digests := make(map[string][32]byte)
	for _, result := range results {
		if result.Err != nil {
			continue
		}
		data, err := os.ReadFile(result.OutputPath)
		if err == nil {
			digests[result.OutputPath] = sha256.Sum256(data)
		}
	}
	return digests
}

func sameDigests(a, b map[string][32]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for path, digest := range a {
		if b[path] != digest {
			return false
		}
	}
	return true
}

func countFailures(results []pipeline.Result) int {
	failures := 0
	for _, result := range results {
		if result.Err != nil {
			failures++
		}
	}
	return failures
}

// benchmark is a Monte Carlo benchmark: it repeats the whole batch for every
// mode, Warmup untimed rounds then Runs timed rounds, and summarizes the
// samples. Modes run in a freshly shuffled order each round, so slow drift
// (a laptop heating up, background programs) affects every mode alike.
func benchmark(cfg config) ([]modeReport, error) {
	samples := make(map[pipeline.Mode][]time.Duration)
	allocBytes := make(map[pipeline.Mode]uint64)
	allocCount := make(map[pipeline.Mode]uint64)
	rounds := cfg.Warmup + cfg.Runs
	order := slices.Clone(cfg.Modes)

	for round := range rounds {
		shuffler := rand.New(rand.NewPCG(cfg.Seed, uint64(round)))
		shuffler.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
		timed := round >= cfg.Warmup

		for _, mode := range order {
			took, bytes, count, err := timeOneRun(mode, cfg)
			if err != nil {
				return nil, err
			}
			if timed {
				samples[mode] = append(samples[mode], took)
				allocBytes[mode] += bytes
				allocCount[mode] += count
			}
		}
		reportProgress(round+1, rounds, cfg.Warmup)
	}
	return buildReports(cfg, samples, allocBytes, allocCount), nil
}

// timeOneRun runs one mode over the batch and measures wall time and heap
// allocation. Memory statistics are read outside the timed region.
func timeOneRun(mode pipeline.Mode, cfg config) (time.Duration, uint64, uint64, error) {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	started := time.Now()
	_, err := pipeline.Run(mode, cfg.Tasks, cfg.Options)
	took := time.Since(started)
	runtime.ReadMemStats(&after)
	return took, after.TotalAlloc - before.TotalAlloc, after.Mallocs - before.Mallocs, err
}

func reportProgress(round, rounds, warmup int) {
	isLast := round == rounds
	if round%5 != 0 && !isLast {
		return
	}
	phase := "timed"
	if round <= warmup {
		phase = "warm-up"
	}
	fmt.Fprintf(out, "  round %d/%d done (%s)\n", round, rounds, phase)
}

func buildReports(cfg config, samples map[pipeline.Mode][]time.Duration,
	allocBytes, allocCount map[pipeline.Mode]uint64) []modeReport {

	images := len(cfg.Tasks)
	reports := make([]modeReport, 0, len(cfg.Modes))
	for _, mode := range cfg.Modes {
		stats := timing.Summarize(samples[mode])
		reports = append(reports, modeReport{
			Mode:            mode,
			Timing:          stats,
			ImagesPerSecond: stats.PerSecond(images),
			AllocMBPerRun:   float64(allocBytes[mode]) / float64(cfg.Runs) / (1 << 20),
			AllocsPerRun:    float64(allocCount[mode]) / float64(cfg.Runs),
		})
	}
	addSpeedups(reports)
	return reports
}

// addSpeedups fills in each mode's speedup relative to sequential, when the
// sequential mode was measured. A mode whose time measured as zero (possible
// for tiny batches with a coarse clock) gets no speedup rather than a
// division by zero, which JSON could not store.
func addSpeedups(reports []modeReport) {
	index := slices.IndexFunc(reports, func(r modeReport) bool { return r.Mode == pipeline.Sequential })
	if index < 0 {
		return
	}
	baseline := reports[index].Timing.MeanMs
	for i := range reports {
		if reports[i].Timing.MeanMs > 0 {
			reports[i].Speedup = baseline / reports[i].Timing.MeanMs
		}
	}
}
