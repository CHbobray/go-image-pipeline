package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/CHbobray/go-image-pipeline/imageprocessing"
	"github.com/CHbobray/go-image-pipeline/pipeline"
)

// compareAll is the -mode value that benchmarks every mode side by side.
const compareAll = "compare"

// Limits on user-supplied numbers.
const (
	maxRuns     = 1000
	maxWarmup   = 100
	minLongSide = 16
	maxLongSide = 10_000
	maxWorkers  = 256
)

// config holds every setting for one run of the program.
type config struct {
	InputDir   string
	OutputDir  string
	ResultsDir string
	Modes      []pipeline.Mode
	Runs       int
	Warmup     int
	Options    pipeline.Options
	Seed       uint64
	CPUProfile string
	MemProfile string
	Pause      bool
	Tasks      []pipeline.Task // filled in by validate from InputDir
	Skipped    []string        // files in InputDir that are not JPEG or PNG
}

// parseConfig reads command-line flags and checks them. Relative default
// paths are resolved against root, the folder that holds images/.
func parseConfig(args []string, root string, usageOut io.Writer) (config, error) {
	var cfg config
	var modeName string
	flags := flag.NewFlagSet("imagepipeline", flag.ContinueOnError)
	flags.SetOutput(usageOut)
	flags.StringVar(&cfg.InputDir, "in", filepath.Join(root, "images"), "folder of input images (.jpg, .jpeg, .png)")
	flags.StringVar(&cfg.OutputDir, "out", filepath.Join(root, "images", "output"), "folder for processed images (created if missing)")
	flags.StringVar(&cfg.ResultsDir, "results", filepath.Join(root, "results"), "folder for the run log and timing files")
	flags.StringVar(&modeName, "mode", compareAll, "sequential, pipeline, parallel, or compare (all three)")
	flags.IntVar(&cfg.Runs, "runs", 20, fmt.Sprintf("timed runs per mode (1-%d)", maxRuns))
	flags.IntVar(&cfg.Warmup, "warmup", 2, fmt.Sprintf("untimed warm-up runs per mode (0-%d)", maxWarmup))
	flags.IntVar(&cfg.Options.LongSide, "size", 500, fmt.Sprintf("output pixels on the longer side (%d-%d)", minLongSide, maxLongSide))
	flags.BoolVar(&cfg.Options.KeepAspect, "keep-aspect", true, "keep each image's shape; -keep-aspect=false makes every output size x size like the original program")
	flags.IntVar(&cfg.Options.Workers, "workers", runtime.NumCPU(), fmt.Sprintf("goroutines per stage in parallel mode (1-%d)", maxWorkers))
	flags.Uint64Var(&cfg.Seed, "seed", 1, "seed for shuffling the order of modes in each timed round")
	flags.StringVar(&cfg.CPUProfile, "cpuprofile", "", "write a CPU profile to this file")
	flags.StringVar(&cfg.MemProfile, "memprofile", "", "write a heap profile to this file")
	flags.BoolVar(&cfg.Pause, "pause", len(args) == 0,
		"wait for Enter before exiting (default on when started with no arguments, e.g. by double-click)")
	flags.Usage = func() {
		fmt.Fprintf(usageOut, "Usage: imagepipeline [flags]\n\n"+
			"Resizes and grayscales every image in a folder, with and without goroutines,\n"+
			"checks that all modes produce identical files, and compares their run times.\n\nFlags:\n")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		return cfg, err
	}
	if flags.NArg() > 0 {
		return cfg, fmt.Errorf("unexpected argument %q (all options are flags, e.g. -runs 10)", flags.Arg(0))
	}
	cfg.Modes = modesFor(modeName)
	return cfg, cfg.validate(modeName)
}

// modesFor turns the -mode value into the list of modes to run.
func modesFor(name string) []pipeline.Mode {
	if name == compareAll {
		return pipeline.Modes
	}
	mode, err := pipeline.ParseMode(name)
	if err != nil {
		return nil
	}
	return []pipeline.Mode{mode}
}

// validate checks every setting, collecting all problems so the user can fix
// them in one pass, and builds the task list from the input folder.
func (cfg *config) validate(modeName string) error {
	var problems []string
	if len(cfg.Modes) == 0 {
		problems = append(problems, fmt.Sprintf("-mode %q is not one of sequential, pipeline, parallel, compare", modeName))
	}
	problems = append(problems, checkRange("-runs", cfg.Runs, 1, maxRuns)...)
	problems = append(problems, checkRange("-warmup", cfg.Warmup, 0, maxWarmup)...)
	problems = append(problems, checkRange("-size", cfg.Options.LongSide, minLongSide, maxLongSide)...)
	problems = append(problems, checkRange("-workers", cfg.Options.Workers, 1, maxWorkers)...)
	problems = append(problems, cfg.checkFolders()...)

	if len(problems) > 0 {
		return errors.New("invalid input:\n  - " + strings.Join(problems, "\n  - "))
	}
	return nil
}

func checkRange(flagName string, value, low, high int) []string {
	if value < low || value > high {
		return []string{fmt.Sprintf("%s must be between %d and %d (got %d)", flagName, low, high, value)}
	}
	return nil
}

// checkFolders verifies the input folder has images and that output would not
// overwrite the inputs, then fills in Tasks and Skipped.
func (cfg *config) checkFolders() []string {
	entries, err := os.ReadDir(cfg.InputDir)
	if err != nil {
		return []string{fmt.Sprintf("-in: cannot read folder %s (%v)", cfg.InputDir, err)}
	}
	if sameFolder(cfg.InputDir, cfg.OutputDir) {
		return []string{"-out must be a different folder from -in, or the inputs would be overwritten"}
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			continue
		}
		if !imageprocessing.SupportedExtension(name) {
			cfg.Skipped = append(cfg.Skipped, name)
			continue
		}
		cfg.Tasks = append(cfg.Tasks, pipeline.Task{
			InputPath:  filepath.Join(cfg.InputDir, name),
			OutputPath: filepath.Join(cfg.OutputDir, name),
		})
	}
	if len(cfg.Tasks) == 0 {
		return []string{fmt.Sprintf("-in: no .jpg, .jpeg, or .png files found in %s", cfg.InputDir)}
	}
	return nil
}

func sameFolder(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	return errA == nil && errB == nil && filepath.Clean(absA) == filepath.Clean(absB)
}

// findProjectRoot returns the first of the working directory, the
// executable's folder, or its parent that contains an images folder, so the
// .exe also works when double-clicked inside bin\.
func findProjectRoot() string {
	candidates := []string{"."}
	if executable, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(executable)
		candidates = append(candidates, exeDir, filepath.Dir(exeDir))
	}
	for _, dir := range candidates {
		if info, err := os.Stat(filepath.Join(dir, "images")); err == nil && info.IsDir() {
			return dir
		}
	}
	return "."
}

// modeNames lists modes as plain strings for messages.
func modeNames(modes []pipeline.Mode) string {
	names := make([]string, len(modes))
	for i, mode := range modes {
		names[i] = string(mode)
	}
	return strings.Join(names, ", ")
}
