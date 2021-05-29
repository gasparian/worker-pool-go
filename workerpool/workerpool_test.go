package workerpool

import (
	"testing"
)

func exampleJob(inp int) JobFunc {
	return func() Result {
		res := inp * inp
		return Result{
			Data: res,
			Err:  nil,
		}
	}
}

func TestPool(t *testing.T) {
	config := WorkerPoolConfig{
		NWorkers:      3,
		MaxJobs:       10,
		MaxJobsReload: 3,
	}
	wp := NewWorkerPool(config)
	roundRobinPool := NewWorkerPoolRoundRobin(wp)

	t.Run("RunJobsSync", func(t *testing.T) {
		nJobs := 50
		jobs := make([]chan Result, 0)
		for i := 0; i < nJobs; i++ {
			ch, err := roundRobinPool.ScheduleJob(exampleJob(i))
			if err != nil {
				t.Log("WARNING: ", err)
				break
			}
			jobs = append(jobs, ch)
		}

		t.Log("INFO: Getting the results...")
		var processedJobsCount int = 0
		for _, j := range jobs {
			res := <-j
			if res.Err != nil {
				t.Fatal(res.Err)
			}
			processedJobsCount++
		}
		if processedJobsCount >= nJobs {
			t.Error()
		}
		t.Log("Number of processed jobs: ", processedJobsCount)
	})

	t.Run("CheckStats", func(t *testing.T) {
		for w := 0; w < config.NWorkers; w++ {
			s, err := roundRobinPool.GetWorkerStats(w)
			if err != nil {
				t.Fatal(err)
			}
			if s.ProcessedJobs <= 0 || s.TotalElapsedTime == 0 {
				t.Error()
			}
			t.Logf("Worker %v stats: %v\n", w, s) // Worker 0 stats: {4 1335} ...
		}
	})

	t.Run("ReloadWorker", func(t *testing.T) {
		err := roundRobinPool.ReloadWorker(0)
		if err != nil {
			t.Fatal(err)
		}
		s0, err := roundRobinPool.GetWorkerStats(0)
		if err != nil {
			t.Fatal(err)
		}
		s1, err := roundRobinPool.GetWorkerStats(1)
		if err != nil {
			t.Fatal(err)
		}
		if s0.ProcessedJobs != 0 || s0.TotalElapsedTime != 0 || s1.ProcessedJobs == 0 {
			t.Error()
		}
	})
}
