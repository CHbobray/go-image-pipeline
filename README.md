# Go Image Pipeline: With and Without Goroutines

**Question from management:** can Go concurrency reduce the time a data pipeline takes?

This repository replicates and extends Amrit Singh's (CODEHEIM) Go image-processing pipeline, [Episode #21: Concurrency in Go: Pipeline Pattern](https://www.codeheim.io/courses/Episode-21-Concurrency-in-Go-Pipeline-Pattern-65c3ca14e4b0628a4e002201) ([original repository](https://github.com/code-heim/go_21_goroutines_pipeline)). It processes a folder of images (load → resize → grayscale → save) in three ways and times each one:

| Mode | How it runs | Worker goroutines |
|---|---|---|
| `sequential` | each image goes through all four stages before the next one starts | none |
| `pipeline` | one goroutine per stage, connected by channels (the original program's design); stages overlap | 1 per stage |
| `parallel` | the same pipeline with several worker goroutines per stage (fan-out), so several images can be in one stage at once | `-workers` per stage (default: one per CPU) |

**Short answer:** yes. On the four sample images, the goroutine versions were **1.40x (pipeline)** and **1.45x (parallel)** faster than sequential on a 2-core machine. All three modes wrote byte-identical files and allocated the same memory. A larger generated batch showed a similar gain (up to 1.55x). More CPU cores should raise the parallel mode's advantage; see [Results](#results).

---

## Contents

- [Repository layout](#repository-layout)
- [What changed from the original](#what-changed-from-the-original)
- [Requirements](#requirements)
- [Quick start on Windows](#quick-start-on-windows)
- [Building the executable](#building-the-executable)
- [Using the program](#using-the-program)
- [Testing and benchmarks](#testing-and-benchmarks)
- [Results](#results)
- [Preserving the aspect ratio (optional item)](#preserving-the-aspect-ratio-optional-item)
- [Design notes](#design-notes)
- [GenAI Tools](#genai-tools)
- [Credits and license](#credits-and-license)

---

## Repository layout

```
.
├── README.md                this file
├── build.ps1                Windows: run tests, build bin\imagepipeline.exe, optionally run it
├── go.mod, go.sum           Go module (github.com/CHbobray/go-image-pipeline) and dependency checksums
├── LICENSE                  Apache 2.0, from the original repository
│
├── main.go                  program flow: validate input, check run, benchmark, report
├── config.go                command-line flags and input validation
├── benchmark.go             identical-output check and Monte Carlo benchmark of the modes
├── report.go                console tables, timing.json, timing_comparison.md
├── main_test.go             tests of validation and of complete runs
│
├── imageprocessing/         reusable image steps: Read, Write, Resize, Grayscale, ScaledSize
├── pipeline/                the three modes; stage.go holds the goroutine/channel stage helper
│   └── benchmark_test.go    Go benchmark methods for pipeline throughput
├── timing/                  reusable statistics for repeated timings (mean, std, median, p95)
│
├── images/                  the four input images (see docs/IMAGE_CREDITS.md)
│   └── output/              processed images (created by the program; not committed)
├── results/                 output of the runs reported below
│   ├── timing_comparison.md     mode comparison and per-stage times
│   ├── timing.json              the same, with every statistic, as JSON
│   ├── run_log.txt              log written by the program: start time, command, all output, outcome
│   ├── go_benchmarks.txt        output of go test -bench -benchmem
│   └── cpu_profile_summary.txt  CPU profile of a full run
└── docs/
    ├── IMAGE_CREDITS.md         sources and licenses of the images
    └── genai-conversation-log.md  the AI-assistant conversation for this project
```

---

## What changed from the original

The Git history keeps the three original CODEHEIM commits, followed by one commit per change below.

| Original | Now |
|---|---|
| Four 1536 × 1536 sample JPEGs | Four public-domain/CC0 photos of different sizes and shapes, one of them PNG ([credits](docs/IMAGE_CREDITS.md)) |
| Any problem calls `panic`, crashing the whole run | Functions return errors. A bad image is reported with its reason, the other images are still processed, and the exit code is 1 |
| Only JPEG could be read; the PNG sample crashed it with `image: unknown format` | JPEG and PNG are read; output format follows the file extension |
| Concurrent pipeline only | `sequential`, `pipeline`, and `parallel` modes sharing the same stage functions |
| Every image squashed to 500 × 500 | Aspect ratio kept by default; `-keep-aspect=false` restores the original behavior |
| File list hard-coded in `main.go` | Processes every image in a folder given by `-in` |
| No tests or timing | Unit tests, Go benchmark methods, a Monte Carlo benchmark in the program, a run log, and CPU/heap profiles |
| Prints `Success!` per image | Per-image results with output size, a check that all modes wrote identical files, and timing tables |

---

## Requirements

- [Go](https://go.dev/dl/) **1.24 or newer** (developed with Go 1.27). The only dependency, [`github.com/nfnt/resize`](https://github.com/nfnt/resize), downloads automatically.
- [Git](https://git-scm.com/download/win), to clone the repository.

Nothing else: the program is pure Go and builds into a single executable.

---

## Quick start on Windows

In PowerShell:

```powershell
git clone https://github.com/CHbobray/go-image-pipeline.git
cd go-image-pipeline
.\build.ps1 -Run
```

That runs the tests, builds `bin\imagepipeline.exe`, and compares the three modes on the images in `images\`. It takes well under a minute. If PowerShell says running scripts is disabled, run `Set-ExecutionPolicy -Scope CurrentUser RemoteSigned` once and try again.

---

## Building the executable

**Windows (.exe)**, with the script:

```powershell
.\build.ps1              # test, then build bin\imagepipeline.exe
.\build.ps1 -SkipTests   # build only
```

or directly:

```powershell
$env:CGO_ENABLED="0"; go build -trimpath -ldflags "-s -w" -o bin\imagepipeline.exe .
```

**macOS or Linux**, or cross-compiling the Windows `.exe` from them:

```bash
go build -o bin/imagepipeline .
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o bin/imagepipeline.exe .
```

The `.exe` is about 4 MB and needs nothing installed. Double-clicking it runs the default comparison in a console window, which stays open until you press Enter. It finds the `images` folder whether started from the repository or from `bin\`.

---

## Using the program

```
Usage: imagepipeline [flags]

  -mode string        sequential, pipeline, parallel, or compare (all three) (default "compare")
  -in string          folder of input images (.jpg, .jpeg, .png) (default "images")
  -out string         folder for processed images, created if missing (default "images/output")
  -results string     folder for the run log and timing files (default "results")
  -size int           output pixels on the longer side (16-10000) (default 500)
  -keep-aspect        keep each image's shape; -keep-aspect=false makes every output size x size (default true)
  -workers int        goroutines per stage in parallel mode (1-256) (default: number of CPUs)
  -runs int           timed runs per mode (1-1000) (default 20)
  -warmup int         untimed warm-up runs per mode (0-100) (default 2)
  -seed uint          seed for shuffling the order of modes in each timed round (default 1)
  -cpuprofile string  write a CPU profile to this file
  -memprofile string  write a heap profile to this file
  -pause              wait for Enter before exiting (default on when started with no arguments)
```

Examples:

```powershell
.\bin\imagepipeline.exe                          # compare all three modes on images\
.\bin\imagepipeline.exe -mode sequential -runs 5 # time only the version without goroutines
.\bin\imagepipeline.exe -workers 4               # parallel mode with 4 workers per stage
.\bin\imagepipeline.exe -keep-aspect=false       # original behavior: every output 500 x 500
.\bin\imagepipeline.exe -in C:\Photos -out C:\Photos\gray -size 800
```

**What a run does:**

1. Checks every flag and the input folder. Files that are not `.jpg`, `.jpeg`, or `.png` are listed and skipped.
2. Runs each selected mode once, lists every image's result and output size, and confirms that all modes wrote **byte-identical** files.
3. Shows how long each stage took in the sequential pass.
4. Runs the Monte Carlo benchmark: `-warmup` untimed rounds, then `-runs` timed rounds. Each round runs every mode once in a freshly shuffled order, so slow drift such as a laptop heating up affects all modes alike.
5. Prints and saves the timing table (`results\timing_comparison.md`, `results\timing.json`). Everything shown on screen is also written to `results\run_log.txt`, with the start time, the exact command, and whether the run finished or why it failed.

**Input checking.** All problems are reported together, before any work starts:

```
> .\bin\imagepipeline.exe -runs 0 -mode turbo -size 5 -in nowhere
invalid input:
  - -mode "turbo" is not one of sequential, pipeline, parallel, compare
  - -runs must be between 1 and 1000 (got 0)
  - -size must be between 16 and 10000 (got 5)
  - -in: cannot read folder nowhere (open nowhere: no such file or directory)

Run with -h to see all options.
```

The program also refuses an output folder that is the same as the input folder, since that would overwrite the originals, and an input folder with no images. A corrupt or unreadable image does not stop the run; it shows up as `FAILED  name  reason`.

**Exit codes:** `0` success · `1` runtime error or an image failed · `2` invalid input · `3` modes produced different files.

---

## Testing and benchmarks

**Unit tests:**

```powershell
go test ./...            # all tests
go test -cover ./...     # with coverage
go test -race ./...      # also check for data races between goroutines (needs a C compiler for cgo)
```

| Package | What the tests check | Coverage |
|---|---|---|
| `imageprocessing` | grayscale values for known colors (white, black, red); resize to the requested size; aspect-ratio sizes for landscape, **tall**, square, very thin, and upscaled images; JPEG and PNG round trips; a PNG with a `.jpg` name; errors for missing, corrupt, and unsupported files; partial files removed when encoding fails | 98% |
| `pipeline` | all three modes write **byte-identical files** in input order; outputs are grayscale with the right shape; a mix of good, corrupt, missing, and unsupported files finishes in every mode **without deadlock**, failing only the bad ones; empty batches; invalid options | 100% |
| `timing` | mean, median, sample standard deviation, p95, and rate on known values; one-sample and empty inputs | 100% |
| main | flag defaults; every validation rule; bad and stray arguments; `-h`; complete runs on generated images, checking output files, `timing.json`, and the run log; a run with a corrupt image | 78% |

Test images are generated by the tests themselves, in several shapes including portrait, so the tests need no downloads. The suite also passes with the race detector (`-race`).

**Benchmark methods** (time and memory for each component and for whole batches):

```powershell
go test -run XXX -bench . -benchmem ./...
```

| Benchmark | Measures |
|---|---|
| `BenchmarkGrayscale`, `BenchmarkResize` | one 1000 × 750 image through one step |
| `BenchmarkRepoImages/{sequential,pipeline,parallel}` | the four images in `images\`, in each mode |
| `BenchmarkSyntheticBatch/{sequential,pipeline,parallel}` | 24 generated 1200-pixel images, in each mode |

Each reports time per batch, bytes and allocations per batch, and images per second. Output from the development machine is in `results/go_benchmarks.txt`.

**Profiles:**

```powershell
.\bin\imagepipeline.exe -cpuprofile cpu.prof -memprofile mem.prof
go tool pprof -top bin\imagepipeline.exe cpu.prof
go tool pprof -http=:8080 bin\imagepipeline.exe cpu.prof    # interactive view in the browser
```

---

## Results

Measured on the development machine: **Linux x86-64, 2 CPU cores**, Go 1.27.3. Parallel mode used 2 workers per stage (one per core). Run the program on your own computer to get your numbers; with more cores, use the default `-workers` (one per CPU).

### Monte Carlo benchmark: the four sample images (20 timed rounds)

| Mode | Mean (ms) | Std (ms) | Median (ms) | p95 (ms) | Images/s | Speedup vs sequential | Allocated per run (MB) |
|---|---:|---:|---:|---:|---:|---:|---:|
| sequential | 162.4 | 13.5 | 163.8 | 185.7 | 24.6 | 1.00x | 29.8 |
| pipeline | 116.4 | 10.7 | 116.0 | 138.6 | 34.4 | **1.40x** | 29.8 |
| parallel | 112.0 | 9.9 | 115.1 | 128.0 | 35.7 | **1.45x** | 29.8 |

All three modes wrote byte-identical output files.

### Where the time goes (one sequential pass)

| Stage | Time for all images (ms) | Share |
|---|---:|---:|
| Load (decode) | 58.2 | 31% |
| Resize | 92.5 | 49% |
| Grayscale | 19.0 | 10% |
| Save (encode) | 20.3 | 11% |

The CPU profile agrees: most time is in `nfnt/resize`'s resampling and in the standard library's JPEG decoder.

### Go benchmark methods (`go test -bench`)

| Batch | Mode | Time per batch (ms) | Images/s | Memory per batch (MB) | Allocations per batch |
|---|---|---:|---:|---:|---:|
| 4 sample images | sequential | 155.3 | 25.8 | 31.3 | 802,204 |
| | pipeline | 112.2 | 35.6 | 31.3 | 802,222 |
| | parallel | 109.5 | 36.5 | 31.3 | 802,227 |
| 24 generated 1200-px images | sequential | 1,094.1 | 21.9 | 220.7 | 4,594,726 |
| | pipeline | 782.4 | 30.7 | 220.7 | 4,594,745 |
| | parallel | 706.1 | 34.0 | 220.7 | 4,594,751 |

### What the results mean

- **Goroutines cut the time by about 30%** (a 1.4–1.55x speedup), with no change in output and essentially no extra memory: the concurrent modes add only about 20 allocations per batch, for the goroutines and channels themselves.
- **Why the pipeline helps even with one goroutine per stage.** Stages overlap: while one image is being resized, the previous one can be converted to grayscale and the next one decoded. But a pipeline is only as fast as its slowest stage, here resizing at about half the work, so its gain levels off.
- **Why parallel helps more, and more on bigger batches.** Fan-out runs the slow stage on several images at once. On 2 cores the extra gain over the pipeline is small, because the pipeline already keeps both cores fairly busy. The gap widens with more images (1.55x vs 1.40x on the 24-image batch) and should widen further on a machine with more cores.
- **The limit is the hardware, not the code.** Concurrency can only use cores that exist. On 2 cores the ideal speedup is 2x, and Go reached about 1.5x after coordination costs and the parts that cannot overlap. That matches what management would see on a small server; larger servers should see more.
- **A finding for future optimization.** The memory benchmarks show `Grayscale` making one allocation per pixel (750,002 for a 1000 × 750 image), because the original code reads every pixel through the generic `img.At` interface. Reading the pixel buffer directly would remove that cost. It was left unchanged so that all three modes run the original algorithm and the comparison isolates the effect of concurrency.

---

## Preserving the aspect ratio (optional item)

The original `Resize` forced every image to 500 × 500, which distorts any picture that is not square. Now `imageprocessing.ScaledSize` reads each image's width and height and scales the **longer side** to `-size` (500 by default), scaling the shorter side by the same factor:

| Input | Original output | New output |
|---|---|---|
| `retina.jpg`, 1411 × 1411 | 500 × 500 | 500 × 500 (square stays square) |
| `hubble_deep_field.jpg`, 1000 × 872 | 500 × 500 (squashed) | 500 × 436 |
| `spacex_rocket.jpg`, 640 × 427 | 500 × 500 (stretched) | 500 × 334 |
| `coffee.png`, 600 × 400 | 500 × 500 (stretched) | 500 × 333 |
| a tall 300 × 900 image (unit test) | 500 × 500 | 167 × 500 |

The short side is rounded to the nearest pixel and never drops below 1. Use `-keep-aspect=false` to get the original square output.

---

## Design notes

- **One set of stage functions, three schedulers.** `load`, `resize`, `grayscale`, and `save` in `pipeline/pipeline.go` are plain functions. Sequential mode calls them in a loop; the concurrent modes wrap them with `runStage` (`pipeline/stage.go`), which starts a number of worker goroutines reading from one channel and writing to the next, and closes its output when all workers finish. So any timing difference comes from scheduling alone, and the modes provably produce the same files.
- **Errors travel with the work.** Each job carries an `err` field. The first stage that fails sets it, and later stages pass the job along untouched. No goroutine is ever left blocked, so a bad file cannot deadlock the pipeline. A test checks this with a timeout.
- **Order is preserved.** Parallel workers can finish out of order, so each job remembers its position and results are returned in input order.
- **"Share memory by communicating."** Goroutines never touch shared variables: each image is owned by exactly one stage at a time and handed on through a channel. The only synchronization besides channels is a `sync.WaitGroup` that closes each stage's output.
- **Reusable packages.** `imageprocessing` and `timing` have no ties to this program and can be used elsewhere; `pipeline` works for any list of input/output file pairs.
- **Fair benchmarking.** Warm-up rounds, shuffled mode order in each round, and memory statistics read outside the timed region.

---

## GenAI Tools

This project was built with help from **Claude** (Anthropic), used through the Claude app with its ability to run code in a cloud workspace.

**How it was used:**

- **Planning and setup.** I gave Claude the assignment text. It located and cloned the CODEHEIM repository, built and ran the original unchanged, and kept the original Git history so my changes show as separate commits on top.
- **Choosing images.** I asked Claude to pick the four images. It chose public-domain and CC0 photos with different sizes, shapes, and formats and recorded their sources in `docs/IMAGE_CREDITS.md`. Running the original program on them exposed its PNG crash.
- **Writing code.** Claude wrote the error handling, the `pipeline` package with the three modes, the aspect-ratio resizing, the command-line program with input validation and logging, the unit tests, the benchmark methods, `build.ps1`, and this README. It committed each step separately with a description.
- **Testing and measuring.** Claude ran the tests (including the race detector), the Go benchmarks, the Monte Carlo comparison, and a CPU profile on its 2-core Linux workspace, and cross-compiled the Windows `.exe`. The results above come from those runs.
- **Mistakes caught along the way.** Claude's first per-stage timing code used `defer` in a way that lost the measurement (the job was copied before the deferred update ran). It noticed when the stage table came out empty and fixed it. One test assumed the JPEG encoder would reject an empty image; it doesn't, so the test was changed to use a too-wide image instead.

**My role:** I supplied the assignment, created the GitHub repository, chose to let Claude pick the images, and directed and reviewed the work.

**Conversation log:** the full conversation for this assignment is in [`docs/genai-conversation-log.md`](docs/genai-conversation-log.md).

---

## Credits and license

- Original program: Amrit Singh, CODEHEIM, [go_21_goroutines_pipeline](https://github.com/code-heim/go_21_goroutines_pipeline), Apache License 2.0. This repository is a modified version; the changes are listed in [What changed from the original](#what-changed-from-the-original) and in the Git history. The original license is kept in `LICENSE`.
- Image resizing: [`github.com/nfnt/resize`](https://github.com/nfnt/resize) (ISC license).
- Images: see [`docs/IMAGE_CREDITS.md`](docs/IMAGE_CREDITS.md).
- McConnell, S. (2004). *Code Complete* (2nd ed.). Microsoft Press.
