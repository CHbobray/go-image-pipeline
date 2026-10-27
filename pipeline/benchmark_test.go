package pipeline

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/CHbobray/go-image-pipeline/imageprocessing"
)

// Pipeline throughput benchmarks. Run them with:
//
//	go test -run XXX -bench . -benchmem ./pipeline
//
// Each benchmark reports ns/op (time for the whole batch), B/op and
// allocs/op (memory per batch), and images/s (throughput).

var benchOptions = Options{LongSide: 500, KeepAspect: true, Workers: runtime.NumCPU()}

// BenchmarkRepoImages times the four images in the repository's images/ folder.
func BenchmarkRepoImages(b *testing.B) {
	tasks := repoTasks(b)
	for _, mode := range Modes {
		b.Run(string(mode), func(b *testing.B) { benchmarkMode(b, mode, tasks) })
	}
}

// BenchmarkSyntheticBatch times 24 generated 1200-pixel images, a larger
// batch that gives the concurrent modes more work to overlap.
func BenchmarkSyntheticBatch(b *testing.B) {
	tasks := syntheticTasks(b, 24, 1200)
	for _, mode := range Modes {
		b.Run(string(mode), func(b *testing.B) { benchmarkMode(b, mode, tasks) })
	}
}

func benchmarkMode(b *testing.B, mode Mode, tasks []Task) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Run(mode, tasks, benchOptions); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(len(tasks)*b.N)/b.Elapsed().Seconds(), "images/s")
}

// repoTasks builds tasks for ../images, writing outputs to a temp folder.
func repoTasks(b *testing.B) []Task {
	b.Helper()
	inDir := filepath.Join("..", "images")
	entries, err := os.ReadDir(inDir)
	if err != nil {
		b.Skipf("no images folder: %v", err)
	}
	outDir := b.TempDir()
	var tasks []Task
	for _, entry := range entries {
		if imageprocessing.SupportedExtension(entry.Name()) {
			tasks = append(tasks, Task{
				InputPath:  filepath.Join(inDir, entry.Name()),
				OutputPath: filepath.Join(outDir, entry.Name()),
			})
		}
	}
	if len(tasks) == 0 {
		b.Skip("no images in ../images")
	}
	return tasks
}
