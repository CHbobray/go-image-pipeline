Run on linux amd64, 2 CPUs, go1.27.3; 4 images, 20 timed runs per mode after 2 warm-ups; parallel mode used 2 workers per stage.

| Mode | Mean (ms) | Std (ms) | Median (ms) | p95 (ms) | Images/s | Speedup vs sequential | Allocated per run (MB) |
|---|---:|---:|---:|---:|---:|---:|---:|
| sequential | 162.4 | 13.5 | 163.8 | 185.7 | 24.6 | 1.00x | 29.8 |
| pipeline | 116.4 | 10.7 | 116.0 | 138.6 | 34.4 | 1.40x | 29.8 |
| parallel | 112.0 | 9.9 | 115.1 | 128.0 | 35.7 | 1.45x | 29.8 |

Time per stage in one sequential pass:

| Stage | Time for all images (ms) | Share |
|---|---:|---:|
| Load (decode) | 58.2 | 31% |
| Resize | 92.5 | 49% |
| Grayscale | 19.0 | 10% |
| Save (encode) | 20.3 | 11% |
