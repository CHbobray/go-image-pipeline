package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CHbobray/go-image-pipeline/imageprocessing"
	"github.com/CHbobray/go-image-pipeline/pipeline"
	"github.com/CHbobray/go-image-pipeline/timing"
)

// fakeProject creates a project folder whose images/ holds count small
// generated images plus one non-image file, and returns its path.
func fakeProject(t *testing.T, count int) string {
	t.Helper()
	root := t.TempDir()
	imagesDir := filepath.Join(root, "images")
	os.MkdirAll(imagesDir, 0o755)
	for i := range count {
		img := image.NewRGBA(image.Rect(0, 0, 80+i*10, 60))
		for x := range img.Bounds().Dx() {
			img.Set(x, x%60, color.RGBA{255, uint8(i * 50), 0, 255})
		}
		if err := imageprocessing.Write(filepath.Join(imagesDir, fmt.Sprintf("photo%d.png", i)), img); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(imagesDir, "notes.txt"), []byte("not an image"), 0o644)
	return root
}

func TestDefaults(t *testing.T) {
	cfg, err := parseConfig([]string{"-runs", "3"}, fakeProject(t, 2), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Modes) != 3 || cfg.Runs != 3 || cfg.Options.LongSide != 500 || !cfg.Options.KeepAspect || cfg.Pause {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if len(cfg.Tasks) != 2 || len(cfg.Skipped) != 1 || cfg.Skipped[0] != "notes.txt" {
		t.Errorf("tasks %v, skipped %v; want 2 tasks and notes.txt skipped", cfg.Tasks, cfg.Skipped)
	}
}

func TestNoArgumentsTurnsOnPause(t *testing.T) {
	cfg, err := parseConfig(nil, fakeProject(t, 1), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Pause {
		t.Error("-pause should default on when started without arguments")
	}
}

func TestSingleMode(t *testing.T) {
	cfg, err := parseConfig([]string{"-mode", "pipeline"}, fakeProject(t, 1), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Modes) != 1 || cfg.Modes[0] != pipeline.Pipeline {
		t.Errorf("got modes %v, want [pipeline]", cfg.Modes)
	}
}

func TestInvalidInputsAreAllReported(t *testing.T) {
	root := fakeProject(t, 1)
	args := []string{"-mode", "turbo", "-runs", "0", "-warmup", "-1", "-size", "5", "-workers", "999"}
	_, err := parseConfig(args, root, io.Discard)
	if err == nil {
		t.Fatal("expected validation errors")
	}
	for _, want := range []string{"-mode", "-runs", "-warmup", "-size", "-workers"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %s:\n%v", want, err)
		}
	}
}

func TestFolderProblems(t *testing.T) {
	root := fakeProject(t, 1)
	empty := t.TempDir()
	cases := map[string][]string{
		"cannot read folder":      {"-in", filepath.Join(root, "missing")},
		"no .jpg, .jpeg, or .png": {"-in", empty},
		"different folder":        {"-in", filepath.Join(root, "images"), "-out", filepath.Join(root, "images")},
	}
	for want, args := range cases {
		if _, err := parseConfig(args, root, io.Discard); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("args %v: want error containing %q, got %v", args, want, err)
		}
	}
}

func TestBadFlagValues(t *testing.T) {
	root := fakeProject(t, 1)
	if _, err := parseConfig([]string{"-runs", "ten"}, root, io.Discard); err == nil {
		t.Error("non-numeric -runs should fail")
	}
	if _, err := parseConfig([]string{"20"}, root, io.Discard); err == nil || !strings.Contains(err.Error(), "unexpected argument") {
		t.Errorf("stray argument: got %v", err)
	}
	if _, err := parseConfig([]string{"-h"}, root, io.Discard); !errors.Is(err, flag.ErrHelp) {
		t.Errorf("-h: want flag.ErrHelp, got %v", err)
	}
}

// TestRunEndToEnd runs the whole program on generated images and checks every
// output file and the contents of the results.
func TestRunEndToEnd(t *testing.T) {
	root := fakeProject(t, 3)
	cfg, err := parseConfig([]string{"-runs", "2", "-warmup", "1", "-size", "40", "-workers", "2"}, root, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err := run(cfg); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	for i := range 3 {
		img, err := imageprocessing.Read(filepath.Join(cfg.OutputDir, fmt.Sprintf("photo%d.png", i)))
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != 40 {
			t.Errorf("photo%d: width %d, want 40", i, img.Bounds().Dx())
		}
	}

	var summary timingFile
	data, err := os.ReadFile(filepath.Join(cfg.ResultsDir, "timing.json"))
	if err != nil || json.Unmarshal(data, &summary) != nil {
		t.Fatalf("timing.json unreadable: %v", err)
	}
	if summary.Memory.SysMB <= 0 || summary.Memory.TotalAllocMB <= 0 {
		t.Errorf("program memory not recorded: %+v", summary.Memory)
	}
	if len(summary.Modes) != 3 || summary.Modes[0].Timing.Runs != 2 || summary.Modes[0].Speedup != 1 {
		t.Errorf("unexpected timing summary: %+v", summary.Modes)
	}

	log, _ := os.ReadFile(filepath.Join(cfg.ResultsDir, "run_log.txt"))
	for _, want := range []string{"identical output files", "| parallel |", "Whole-program memory", "finished successfully"} {
		if !strings.Contains(string(log), want) {
			t.Errorf("run log missing %q", want)
		}
	}
}

func TestRunReportsFailedImages(t *testing.T) {
	root := fakeProject(t, 1)
	os.WriteFile(filepath.Join(root, "images", "broken.jpg"), []byte("not a jpeg"), 0o644)
	cfg, err := parseConfig([]string{"-runs", "1", "-warmup", "0", "-size", "20"}, root, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err := run(cfg); !errors.Is(err, errImagesFailed) {
		t.Errorf("want errImagesFailed, got %v", err)
	}
	log, _ := os.ReadFile(filepath.Join(cfg.ResultsDir, "run_log.txt"))
	if !strings.Contains(string(log), "FAILED  broken.jpg") || !strings.Contains(string(log), "run failed") {
		t.Errorf("run log should name the failed image and the outcome:\n%s", log)
	}
}

func TestSpeedupsSkipZeroTimes(t *testing.T) {
	reports := []modeReport{
		{Mode: pipeline.Sequential, Timing: timing.Stats{MeanMs: 100}},
		{Mode: pipeline.Parallel, Timing: timing.Stats{MeanMs: 0}},
	}
	addSpeedups(reports)
	if reports[0].Speedup != 1 || reports[1].Speedup != 0 {
		t.Errorf("speedups %v and %v, want 1 and 0", reports[0].Speedup, reports[1].Speedup)
	}
	if _, err := json.Marshal(reports); err != nil {
		t.Errorf("reports must stay JSON-encodable: %v", err)
	}
}
