Run on linux amd64, 2 CPUs, go1.27.3; 4 images, 20 timed runs per mode after 2 warm-ups; parallel mode used 2 workers per stage.

| Mode | Mean (ms) | Std (ms) | Median (ms) | p95 (ms) | Images/s | Speedup vs sequential | Allocated per run (MB) |
|---|---:|---:|---:|---:|---:|---:|---:|
| sequential | 158.3 | 15.5 | 155.7 | 202.9 | 25.3 | 1.00x | 29.8 |
| pipeline | 117.2 | 18.2 | 111.8 | 164.5 | 34.1 | 1.35x | 29.8 |
| parallel | 104.8 | 11.5 | 103.5 | 134.6 | 38.2 | 1.51x | 29.8 |

Time per stage in one sequential pass:

| Stage | Time for all images (ms) | Share |
|---|---:|---:|
| Load (decode) | 77.4 | 33% |
| Resize | 98.3 | 42% |
| Grayscale | 31.9 | 14% |
| Save (encode) | 24.3 | 10% |
