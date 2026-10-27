// Package pipeline runs a four-stage image job (load → resize → grayscale →
// save) over a batch of files in one of three modes, so the same work can be
// timed with and without goroutines:
//
//   - Sequential: no goroutines; each image goes through all four stages
//     before the next one starts.
//   - Pipeline: one goroutine per stage connected by channels, as in the
//     original CODEHEIM program. Stages overlap: while image 2 is being
//     resized, image 1 can be converted to grayscale.
//   - Parallel: the same pipeline, but every stage runs several worker
//     goroutines (fan-out), so several images can be in the same stage at once.
//
// All three modes call the same stage functions and produce identical files.
package pipeline

import (
	"fmt"
	"image"
	"slices"
	"time"

	"github.com/CHbobray/go-image-pipeline/imageprocessing"
)

// Mode selects how the stages are scheduled.
type Mode string

// The available modes.
const (
	Sequential Mode = "sequential"
	Pipeline   Mode = "pipeline"
	Parallel   Mode = "parallel"
)

// Modes lists every mode, in the order they are reported.
var Modes = []Mode{Sequential, Pipeline, Parallel}

// ParseMode converts a name such as "parallel" to a Mode.
func ParseMode(name string) (Mode, error) {
	mode := Mode(name)
	if slices.Contains(Modes, mode) {
		return mode, nil
	}
	return "", fmt.Errorf("unknown mode %q (choose sequential, pipeline, or parallel)", name)
}

// Task names one input file and where its processed version should be written.
type Task struct {
	InputPath  string
	OutputPath string
}

// Options controls the image transformation and, for Parallel, the fan-out.
type Options struct {
	LongSide   int  // pixels on the longer output side (both sides if KeepAspect is false)
	KeepAspect bool // preserve each image's shape instead of forcing a square
	Workers    int  // goroutines per stage in Parallel mode; ignored by the other modes
}

// StageTimes records how long one image spent in each stage.
type StageTimes struct {
	Load, Resize, Grayscale, Save time.Duration
}

// Result reports the outcome for one task. Err is nil on success.
type Result struct {
	Task
	Width, Height int // output size; zero if the task failed
	Times         StageTimes
	Err           error
}

// Run processes every task in the given mode and returns one Result per task,
// in the same order as tasks. A failure on one image never stops the others.
func Run(mode Mode, tasks []Task, opts Options) ([]Result, error) {
	if opts.LongSide < 1 {
		return nil, fmt.Errorf("output size must be at least 1 pixel, got %d", opts.LongSide)
	}
	switch mode {
	case Sequential:
		return runSequential(tasks, opts), nil
	case Pipeline:
		return runConcurrent(tasks, opts, 1), nil
	case Parallel:
		if opts.Workers < 1 {
			return nil, fmt.Errorf("parallel mode needs at least 1 worker, got %d", opts.Workers)
		}
		return runConcurrent(tasks, opts, opts.Workers), nil
	}
	return nil, fmt.Errorf("unknown mode %q", mode)
}

// job is the unit of work passed between stages. index remembers the task's
// position so results can be returned in input order even when workers finish
// out of order.
type job struct {
	index int
	task  Task
	image image.Image
	times StageTimes
	err   error
}

func runSequential(tasks []Task, opts Options) []Result {
	results := make([]Result, len(tasks))
	for i, task := range tasks {
		finished := save(grayscale(resize(load(job{index: i, task: task}), opts)))
		results[i] = finished.result()
	}
	return results
}

// runConcurrent wires the four stages together with channels, each stage
// running `workers` goroutines, and collects the results.
func runConcurrent(tasks []Task, opts Options, workers int) []Result {
	jobs := make(chan job)
	go func() {
		for i, task := range tasks {
			jobs <- job{index: i, task: task}
		}
		close(jobs)
	}()

	loaded := runStage(jobs, workers, load)
	resized := runStage(loaded, workers, func(j job) job { return resize(j, opts) })
	grayed := runStage(resized, workers, grayscale)
	saved := runStage(grayed, workers, save)

	results := make([]Result, len(tasks))
	for finished := range saved {
		results[finished.index] = finished.result()
	}
	return results
}

// The stage functions below each do one step and record how long it took. A
// job that already failed passes through untouched, so its first error is
// the one reported.

func load(j job) job {
	started := time.Now()
	j.image, j.err = imageprocessing.Read(j.task.InputPath)
	j.times.Load = time.Since(started)
	return j
}

func resize(j job, opts Options) job {
	if j.err != nil {
		return j
	}
	started := time.Now()
	bounds := j.image.Bounds()
	width, height := imageprocessing.ScaledSize(bounds.Dx(), bounds.Dy(), opts.LongSide, opts.KeepAspect)
	j.image = imageprocessing.Resize(j.image, width, height)
	j.times.Resize = time.Since(started)
	return j
}

func grayscale(j job) job {
	if j.err != nil {
		return j
	}
	started := time.Now()
	j.image = imageprocessing.Grayscale(j.image)
	j.times.Grayscale = time.Since(started)
	return j
}

func save(j job) job {
	if j.err != nil {
		return j
	}
	started := time.Now()
	j.err = imageprocessing.Write(j.task.OutputPath, j.image)
	j.times.Save = time.Since(started)
	return j
}

func (j job) result() Result {
	result := Result{Task: j.task, Times: j.times, Err: j.err}
	if j.err == nil {
		bounds := j.image.Bounds()
		result.Width, result.Height = bounds.Dx(), bounds.Dy()
	}
	return result
}
