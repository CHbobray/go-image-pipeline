package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/CHbobray/go-image-pipeline/pipeline"
)

// timingFile is the content of results/timing.json.
type timingFile struct {
	Platform   string        `json:"platform"`
	GoVersion  string        `json:"go"`
	CPUs       int           `json:"cpus"`
	Images     int           `json:"images"`
	LongSide   int           `json:"size"`
	KeepAspect bool          `json:"keep_aspect"`
	Workers    int           `json:"workers"`
	Warmup     int           `json:"warmup_runs"`
	Modes      []modeReport  `json:"modes"`
	Memory     programMemory `json:"program_memory"`
}

// programMemory is the Go runtime's view of the whole program's memory after
// the benchmark, in megabytes.
type programMemory struct {
	SysMB        float64 `json:"sys_mb"`         // obtained from the operating system
	HeapInUseMB  float64 `json:"heap_in_use_mb"` // live heap at the end
	TotalAllocMB float64 `json:"total_alloc_mb"` // allocated over the whole run
	NumGC        uint32  `json:"num_gc"`
}

func sampleProgramMemory() programMemory {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	const mb = 1 << 20
	return programMemory{
		SysMB:        float64(stats.Sys) / mb,
		HeapInUseMB:  float64(stats.HeapInuse) / mb,
		TotalAllocMB: float64(stats.TotalAlloc) / mb,
		NumGC:        stats.NumGC,
	}
}

// printImageResults lists each image's outcome from the check run.
func printImageResults(results []pipeline.Result) {
	fmt.Fprintln(out, "\nImages processed:")
	for _, result := range results {
		name := filepath.Base(result.InputPath)
		if result.Err != nil {
			fmt.Fprintf(out, "  FAILED  %-24s %v\n", name, result.Err)
			continue
		}
		fmt.Fprintf(out, "  ok      %-24s -> %4d x %-4d %s\n", name, result.Width, result.Height, result.OutputPath)
	}
}

// stageTable shows where the time goes inside one sequential pass, the
// processing cost of each pipeline component.
func stageTable(results []pipeline.Result) string {
	var load, resize, gray, save time.Duration
	for _, result := range results {
		load += result.Times.Load
		resize += result.Times.Resize
		gray += result.Times.Grayscale
		save += result.Times.Save
	}
	total := load + resize + gray + save
	if total == 0 {
		return ""
	}

	var table strings.Builder
	table.WriteString("| Stage | Time for all images (ms) | Share |\n|---|---:|---:|\n")
	for _, stage := range []struct {
		name string
		took time.Duration
	}{{"Load (decode)", load}, {"Resize", resize}, {"Grayscale", gray}, {"Save (encode)", save}} {
		fmt.Fprintf(&table, "| %s | %.1f | %.0f%% |\n", stage.name, ms(stage.took), 100*float64(stage.took)/float64(total))
	}
	return table.String()
}

// timingTable renders the benchmark results as a Markdown table.
func timingTable(reports []modeReport) string {
	var table strings.Builder
	table.WriteString("| Mode | Mean (ms) | Std (ms) | Median (ms) | p95 (ms) | Images/s | Speedup vs sequential | Allocated per run (MB) |\n")
	table.WriteString("|---|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, report := range reports {
		s := report.Timing
		speedup := "—"
		if report.Speedup > 0 {
			speedup = fmt.Sprintf("%.2fx", report.Speedup)
		}
		fmt.Fprintf(&table, "| %s | %.1f | %.1f | %.1f | %.1f | %.1f | %s | %.1f |\n",
			report.Mode, s.MeanMs, s.StdMs, s.MedianMs, s.P95Ms, report.ImagesPerSecond, speedup, report.AllocMBPerRun)
	}
	return table.String()
}

// writeResults saves the timing JSON and the Markdown tables.
func writeResults(cfg config, reports []modeReport, memory programMemory, stages string) error {
	summary := timingFile{
		Platform:   runtime.GOOS + " " + runtime.GOARCH,
		GoVersion:  runtime.Version(),
		CPUs:       runtime.NumCPU(),
		Images:     len(cfg.Tasks),
		LongSide:   cfg.Options.LongSide,
		KeepAspect: cfg.Options.KeepAspect,
		Workers:    cfg.Options.Workers,
		Warmup:     cfg.Warmup,
		Modes:      reports,
		Memory:     memory,
	}
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(cfg.ResultsDir, "timing.json"), append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing timing.json: %w", err)
	}

	markdown := fmt.Sprintf("Run on %s, %d CPUs, %s; %d images, %d timed runs per mode after %d warm-ups; parallel mode used %d workers per stage.\n\n%s%s",
		summary.Platform, summary.CPUs, summary.GoVersion, summary.Images, cfg.Runs, cfg.Warmup, cfg.Options.Workers,
		timingTable(reports), stageSection(stages))
	return os.WriteFile(filepath.Join(cfg.ResultsDir, "timing_comparison.md"), []byte(markdown), 0o644)
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// stageSection labels the per-stage table, or is empty if it was not measured.
func stageSection(stages string) string {
	if stages == "" {
		return ""
	}
	return "\nTime per stage in one sequential pass:\n\n" + stages
}
