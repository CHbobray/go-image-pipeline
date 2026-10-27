// Command imagepipeline resizes and grayscales a folder of images with and
// without goroutines, checks that every mode produces identical files, and
// compares their run times with a Monte Carlo benchmark.
//
// It extends the CODEHEIM Episode #21 pipeline example
// (github.com/code-heim/go_21_goroutines_pipeline). Run with -h for the flags.
// Exit codes: 0 success, 1 runtime error or failed image, 2 invalid input,
// 3 modes produced different files.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/CHbobray/go-image-pipeline/pipeline"
)

const (
	exitOK            = 0
	exitError         = 1
	exitBadInput      = 2
	exitOutputsDiffer = 3
)

var errImagesFailed = errors.New("some images could not be processed (see FAILED lines above)")

// out receives all progress messages: the console, plus the run log once
// run() has opened it.
var out io.Writer = os.Stdout

func main() {
	cfg, err := parseConfig(os.Args[1:], findProjectRoot(), os.Stderr)
	code := exitOK
	switch {
	case errors.Is(err, flag.ErrHelp):
		// Usage was already printed.
	case err != nil:
		fmt.Fprintf(os.Stderr, "%v\n\nRun with -h to see all options.\n", err)
		code = exitBadInput
	default:
		code = exitCode(run(cfg))
	}
	if cfg.Pause {
		fmt.Print("\nPress Enter to exit...")
		bufio.NewReader(os.Stdin).ReadString('\n')
	}
	os.Exit(code)
}

func exitCode(err error) int {
	if err == nil {
		return exitOK
	}
	fmt.Fprintln(os.Stderr, "error:", err)
	if errors.Is(err, errOutputsDiffer) {
		return exitOutputsDiffer
	}
	return exitError
}

// run performs the whole job, one step per helper function. Any error it
// returns is also recorded at the end of the run log.
func run(cfg config) (runErr error) {
	for _, dir := range []string{cfg.OutputDir, cfg.ResultsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating folder: %w", err)
		}
	}
	closeLog, err := openRunLog(filepath.Join(cfg.ResultsDir, "run_log.txt"))
	if err != nil {
		return err
	}
	defer func() { closeLog(runErr) }()

	if cfg.CPUProfile != "" {
		stopProfile, err := startCPUProfile(cfg.CPUProfile)
		if err != nil {
			return err
		}
		defer stopProfile()
	}

	printSetup(cfg)
	check, err := checkModes(cfg)
	if err != nil {
		return err
	}
	printImageResults(check.Results)
	fmt.Fprintf(out, "All %d modes wrote identical output files.\n", len(cfg.Modes))

	stages := ""
	if cfg.Modes[0] == pipeline.Sequential {
		stages = stageTable(check.Results)
		fmt.Fprintf(out, "\nTime per stage in one sequential pass:\n\n%s", stages)
	}

	fmt.Fprintf(out, "\nBenchmarking: %d warm-up + %d timed rounds; each round runs every mode once, in shuffled order.\n",
		cfg.Warmup, cfg.Runs)
	reports, err := benchmark(cfg)
	if err != nil {
		return fmt.Errorf("benchmark: %w", err)
	}
	fmt.Fprintf(out, "\nTime to process all %d images (%s, %d CPUs):\n\n%s", len(cfg.Tasks),
		runtime.GOOS+" "+runtime.GOARCH, runtime.NumCPU(), timingTable(reports))

	if err := writeResults(cfg, reports, stages); err != nil {
		return err
	}
	fmt.Fprintf(out, "\nProcessed images are in %s; timing and run log are in %s\n", cfg.OutputDir, cfg.ResultsDir)

	if cfg.MemProfile != "" {
		if err := writeHeapProfile(cfg.MemProfile); err != nil {
			return err
		}
	}
	if check.Failures > 0 {
		return errImagesFailed
	}
	return nil
}

func printSetup(cfg config) {
	fmt.Fprintf(out, "Found %d images in %s\n", len(cfg.Tasks), cfg.InputDir)
	for _, name := range cfg.Skipped {
		fmt.Fprintf(out, "  skipping %s (not a .jpg, .jpeg, or .png file)\n", name)
	}
	shape := "keeping each image's shape"
	if !cfg.Options.KeepAspect {
		shape = "stretched to a square"
	}
	fmt.Fprintf(out, "Output: grayscale, %d pixels on the longer side, %s\n", cfg.Options.LongSide, shape)
	fmt.Fprintf(out, "Modes: %s (parallel uses %d workers per stage; this computer has %d CPUs)\n",
		modeNames(cfg.Modes), cfg.Options.Workers, runtime.NumCPU())
}

// openRunLog copies everything printed to out into a log file as well,
// starting with a timestamp and the exact command line. The returned
// function records how the run ended and closes the file.
func openRunLog(path string) (closeLog func(runErr error), err error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("creating run log: %w", err)
	}
	started := time.Now()
	out = io.MultiWriter(os.Stdout, file)
	fmt.Fprintf(file, "imagepipeline run started %s\ncommand: %q\n\n", started.Format(time.RFC3339), os.Args)
	return func(runErr error) {
		out = os.Stdout
		outcome := "finished successfully"
		if runErr != nil {
			outcome = "failed: " + runErr.Error()
		}
		fmt.Fprintf(file, "\nrun %s after %s\n", outcome, time.Since(started).Round(time.Millisecond))
		file.Close()
	}, nil
}

func startCPUProfile(path string) (stop func(), err error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("creating CPU profile: %w", err)
	}
	if err := pprof.StartCPUProfile(file); err != nil {
		file.Close()
		return nil, fmt.Errorf("starting CPU profile: %w", err)
	}
	return func() {
		pprof.StopCPUProfile()
		file.Close()
	}, nil
}

func writeHeapProfile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating heap profile: %w", err)
	}
	defer file.Close()
	runtime.GC() // report live objects, not garbage awaiting collection
	return pprof.WriteHeapProfile(file)
}
