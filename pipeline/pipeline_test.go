package pipeline

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CHbobray/go-image-pipeline/imageprocessing"
)

// syntheticTasks writes count generated images of varied shapes, landscape,
// portrait, and square, to a temp folder and returns tasks for them.
func syntheticTasks(t testing.TB, count, longSide int) []Task {
	t.Helper()
	inDir, outDir := t.TempDir(), t.TempDir()
	shapes := [][2]int{{4, 3}, {3, 4}, {1, 1}, {16, 9}}
	tasks := make([]Task, count)
	for i := range count {
		shape := shapes[i%len(shapes)]
		width := longSide * shape[0] / max(shape[0], shape[1])
		height := longSide * shape[1] / max(shape[0], shape[1])
		name := fmt.Sprintf("img%02d.png", i)
		if err := imageprocessing.Write(filepath.Join(inDir, name), pattern(width, height, i)); err != nil {
			t.Fatal(err)
		}
		tasks[i] = Task{InputPath: filepath.Join(inDir, name), OutputPath: filepath.Join(outDir, name)}
	}
	return tasks
}

// pattern makes a colorful image that differs for each seed.
func pattern(width, height, seed int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{uint8(x + seed*40), uint8(y * 2), uint8((x ^ y) + seed), 255})
		}
	}
	return img
}

var testOptions = Options{LongSide: 64, KeepAspect: true, Workers: 3}

func TestAllModesProduceIdenticalFiles(t *testing.T) {
	tasks := syntheticTasks(t, 7, 200)
	reference := map[string][]byte{}
	for _, mode := range Modes {
		results, err := Run(mode, tasks, testOptions)
		if err != nil {
			t.Fatal(err)
		}
		for i, result := range results {
			if result.Err != nil {
				t.Fatalf("%s: %v", mode, result.Err)
			}
			if result.InputPath != tasks[i].InputPath {
				t.Fatalf("%s: result %d is for %s, want input order kept", mode, i, result.InputPath)
			}
			data, _ := os.ReadFile(result.OutputPath)
			if mode == Sequential {
				reference[result.OutputPath] = data
			} else if !bytes.Equal(data, reference[result.OutputPath]) {
				t.Errorf("%s: %s differs from sequential output", mode, filepath.Base(result.OutputPath))
			}
		}
	}
}

func TestOutputsAreGrayscaleAndKeepShape(t *testing.T) {
	tasks := syntheticTasks(t, 4, 200) // shapes 4:3, 3:4, 1:1, 16:9
	results, err := Run(Parallel, tasks, testOptions)
	if err != nil {
		t.Fatal(err)
	}
	want := [][2]int{{64, 48}, {48, 64}, {64, 64}, {64, 36}}
	for i, result := range results {
		if result.Width != want[i][0] || result.Height != want[i][1] {
			t.Errorf("image %d: got %dx%d, want %dx%d", i, result.Width, result.Height, want[i][0], want[i][1])
		}
		written, err := imageprocessing.Read(result.OutputPath)
		if err != nil {
			t.Fatal(err)
		}
		if _, isGray := written.(*image.Gray); !isGray {
			t.Errorf("image %d: decoded as %T, want *image.Gray", i, written)
		}
	}
}

func TestStretchToSquareWhenAspectNotKept(t *testing.T) {
	tasks := syntheticTasks(t, 2, 200)
	results, err := Run(Pipeline, tasks, Options{LongSide: 50, KeepAspect: false})
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range results {
		if result.Width != 50 || result.Height != 50 {
			t.Errorf("got %dx%d, want 50x50", result.Width, result.Height)
		}
	}
}

// TestBadFilesDoNotStopTheBatch mixes good, corrupt, and missing inputs and
// checks every mode finishes (no deadlock) with errors only on the bad ones.
func TestBadFilesDoNotStopTheBatch(t *testing.T) {
	tasks := syntheticTasks(t, 3, 100)
	corrupt := filepath.Join(t.TempDir(), "corrupt.jpg")
	os.WriteFile(corrupt, []byte("not an image"), 0o644)
	outDir := filepath.Dir(tasks[0].OutputPath)
	tasks = append(tasks,
		Task{InputPath: corrupt, OutputPath: filepath.Join(outDir, "corrupt.jpg")},
		Task{InputPath: filepath.Join(t.TempDir(), "missing.png"), OutputPath: filepath.Join(outDir, "missing.png")},
		Task{InputPath: tasks[0].InputPath, OutputPath: filepath.Join(outDir, "bad-extension.gif")},
	)

	for _, mode := range Modes {
		results := runWithTimeout(t, mode, tasks)
		for i, result := range results {
			wantFailure := i >= 3
			if (result.Err != nil) != wantFailure {
				t.Errorf("%s: task %d (%s) error = %v, want failure %v", mode, i, filepath.Base(result.OutputPath), result.Err, wantFailure)
			}
		}
	}
}

func runWithTimeout(t *testing.T, mode Mode, tasks []Task) []Result {
	t.Helper()
	done := make(chan []Result, 1)
	go func() {
		results, err := Run(mode, tasks, testOptions)
		if err != nil {
			t.Error(err)
		}
		done <- results
	}()
	select {
	case results := <-done:
		return results
	case <-time.After(10 * time.Second):
		t.Fatalf("%s mode did not finish: possible deadlock", mode)
		return nil
	}
}

func TestEmptyBatch(t *testing.T) {
	for _, mode := range Modes {
		results, err := Run(mode, nil, testOptions)
		if err != nil || len(results) != 0 {
			t.Errorf("%s: got %d results, err %v; want none", mode, len(results), err)
		}
	}
}

func TestRunRejectsBadOptions(t *testing.T) {
	if _, err := Run(Parallel, nil, Options{LongSide: 10, Workers: 0}); err == nil {
		t.Error("parallel with 0 workers should fail")
	}
	if _, err := Run(Sequential, nil, Options{LongSide: 0}); err == nil {
		t.Error("size 0 should fail")
	}
	if _, err := Run("fast", nil, testOptions); err == nil {
		t.Error("unknown mode should fail")
	}
}

func TestParseMode(t *testing.T) {
	for _, mode := range Modes {
		if parsed, err := ParseMode(string(mode)); err != nil || parsed != mode {
			t.Errorf("ParseMode(%q) = %q, %v", mode, parsed, err)
		}
	}
	if _, err := ParseMode("compare"); err == nil {
		t.Error(`ParseMode("compare") should fail; compare is a CLI option, not a mode`)
	}
}

// TestStageTimesAreRecorded shrinks a 2000-pixel image to 1000 pixels so every
// stage does real work taking milliseconds. Go's clock on Windows advances in
// steps of up to about a millisecond, so a stage on a tiny image can measure
// as exactly zero there. The output size must differ from the input size,
// because the resize library returns a same-size image unchanged, instantly.
func TestStageTimesAreRecorded(t *testing.T) {
	results, err := Run(Sequential, syntheticTasks(t, 1, 2000), Options{LongSide: 1000, KeepAspect: true})
	if err != nil {
		t.Fatal(err)
	}
	times := results[0].Times
	for name, took := range map[string]time.Duration{"load": times.Load, "resize": times.Resize, "grayscale": times.Grayscale, "save": times.Save} {
		if took <= 0 {
			t.Errorf("%s time not recorded", name)
		}
	}
}
