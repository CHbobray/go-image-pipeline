package pipeline

import "sync"

// runStage starts `workers` goroutines that each read jobs from in, apply
// process, and send the result on the returned channel. The output channel is
// closed once every worker has finished, which tells the next stage to stop.
//
// With workers == 1 this is the classic pipeline stage from the original
// program; with more workers it fans the stage out across CPU cores.
func runStage(in <-chan job, workers int, process func(job) job) <-chan job {
	out := make(chan job)
	var running sync.WaitGroup
	for range workers {
		running.Add(1)
		go func() {
			defer running.Done()
			for j := range in {
				out <- process(j)
			}
		}()
	}
	go func() {
		running.Wait()
		close(out)
	}()
	return out
}
