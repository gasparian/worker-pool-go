package workerpool

import (
	"math"
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
	config := Config{
		NWorkers: 3,
		MaxJobs:  10,
	}
	wp := New(config)
	roundRobinPool := NewRoundRobin(wp)
	var actualProcessedJobs float64 = 0.0

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

		if len(jobs) >= nJobs {
			t.Error()
		}
		t.Logf("Number of processing jobs: %v (requested: %v)", len(jobs), nJobs)
		actualProcessedJobs = float64(len(jobs))

		t.Log("INFO: Getting the results...")
		for _, j := range jobs {
			res := <-j
			if res.Err != nil {
				t.Fatal(res.Err)
			}
		}
	})

	t.Run("CheckStats", func(t *testing.T) {
		var averageProcessedJobs float64 = 0.0
		for w := 0; w < config.NWorkers; w++ {
			s, err := roundRobinPool.GetWorkerStats(w)
			if err != nil {
				t.Fatal(err)
			}
			if s.ProcessedJobs <= 0 || s.TotalElapsedTime == 0 {
				t.Error()
			}
			averageProcessedJobs += float64(s.ProcessedJobs)
			t.Logf("Worker %v stats: %v\n", w, s) // Worker 0 stats: {4 1335} ...
		}
		averageProcessedJobs /= float64(config.NWorkers)
		idealAverageProcessedJobs := actualProcessedJobs / float64(config.NWorkers)
		t.Log("Average processed jobs per worker:", averageProcessedJobs, idealAverageProcessedJobs)
		if math.Abs(averageProcessedJobs-idealAverageProcessedJobs) > 1 {
			t.Error()
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

	// TODO: add concurrency test
	t.Run("RunJobsConcurrent", func(t *testing.T) {

	})

	t.Run("CheckStatsConcurrent", func(t *testing.T) {

	})

	t.Run("ReloadWorkerConcurrent", func(t *testing.T) {

	})
}
