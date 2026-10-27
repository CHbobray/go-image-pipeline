Run on windows amd64, 12 CPUs, go1.27.1-X:simd; 4 images, 20 timed runs per mode after 2 warm-ups; parallel mode used 12 workers per stage.

| Mode | Mean (ms) | Std (ms) | Median (ms) | p95 (ms) | Images/s | Speedup vs sequential | Allocated per run (MB) |
|---|---:|---:|---:|---:|---:|---:|---:|
| sequential | 138.0 | 42.6 | 126.6 | 278.1 | 29.0 | 1.00x | 29.8 |
| pipeline | 91.9 | 12.4 | 93.8 | 108.3 | 43.5 | 1.50x | 29.8 |
| parallel | 56.4 | 6.9 | 58.0 | 66.1 | 71.0 | 2.45x | 29.8 |

Time per stage in one sequential pass:

| Stage | Time for all images (ms) | Share |
|---|---:|---:|
| Load (decode) | 49.0 | 40% |
| Resize | 36.8 | 30% |
| Grayscale | 18.9 | 15% |
| Save (encode) | 19.2 | 16% |
