package workerpool

import (
	"errors"
	"math"
	"sync"
	"testing"
)

var (
	workerStatsNullErr         = errors.New("Worker stats should not be null")
	workerStatsNotNullErr      = errors.New("Worker stats should be null after the reload")
	jobsDistributionNotEvenErr = errors.New("Jobs should be evenly distributet across workers")
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
	balancer := NewRoundRobin()
	roundRobinPool := New(config, balancer)
	wp := roundRobinPool.wp

	var actualProcessedJobs float64 = 0.0
	nJobs := 50

	t.Run("RunJobsSync", func(t *testing.T) {
		results := make([]chan Result, 0)
		for i := 0; i < nJobs; i++ {
			ch := roundRobinPool.ScheduleJob(exampleJob(i))
			results = append(results, ch)
		}

		t.Logf("Number of processing jobs: %v (requested: %v)", len(results), nJobs)
		actualProcessedJobs = float64(len(results))

		t.Log("INFO: Getting the results...")
		for _, r := range results {
			res := <-r
			if res.Err != nil {
				t.Fatal(res.Err)
			}
			_ = float64(res.Data.(int))
		}
	})

	t.Run("CheckStats", func(t *testing.T) {
		var averageProcessedJobs float64 = 0.0
		for w := 0; w < config.NWorkers; w++ {
			s, err := wp.getWorkerStats(w)
			if err != nil {
				t.Fatal(err)
			}
			if s.ProcessedJobs <= 0 || s.TotalJobsExecutionTime == 0 {
				t.Error(workerStatsNullErr)
			}
			averageProcessedJobs += float64(s.ProcessedJobs)
			t.Logf("Worker %v stats: %v\n", w, s) // Worker 0 stats: {4 1335} ...
		}
		averageProcessedJobs /= float64(config.NWorkers)
		idealAverageProcessedJobs := actualProcessedJobs / float64(config.NWorkers)
		t.Log("Average processed jobs per worker:", averageProcessedJobs, idealAverageProcessedJobs)
		if math.Abs(averageProcessedJobs-idealAverageProcessedJobs) > 1 {
			t.Error(jobsDistributionNotEvenErr)
		}
	})

	t.Run("ReloadWorker", func(t *testing.T) {
		err := wp.reloadWorker(0)
		if err != nil {
			t.Fatal(err)
		}
		s0, err := wp.getWorkerStats(0)
		if err != nil {
			t.Fatal(err)
		}
		s1, err := wp.getWorkerStats(1)
		if err != nil {
			t.Fatal(err)
		}
		if s0.ProcessedJobs != 0 || s0.TotalJobsExecutionTime != 0 || s1.ProcessedJobs == 0 {
			t.Error(workerStatsNotNullErr)
		}
	})

	t.Run("RunJobsConcurrent", func(t *testing.T) {
		for w := 0; w < config.NWorkers; w++ {
			err := wp.reloadWorker(w)
			if err != nil {
				t.Fatal(err)
			}
		}

		jobs := make(chan chan Result, nJobs)
		wg := sync.WaitGroup{}
		for i := 1; i < nJobs; i++ {
			wg.Add(1)
			go func(idx int, wg *sync.WaitGroup) {
				defer wg.Done()
				ch := roundRobinPool.ScheduleJob(exampleJob(idx))
				jobs <- ch
			}(i, &wg)
		}
		wg.Wait()
		close(jobs)

		for j := range jobs {
			res := <-j
			if res.Err != nil {
				t.Fatal(res.Err)
			}
		}

		for w := 0; w < config.NWorkers; w++ {
			s, err := wp.getWorkerStats(w)
			if err != nil {
				t.Fatal(err)
			}
			if s.ProcessedJobs == 0 || s.TotalJobsExecutionTime == 0 {
				t.Error(workerStatsNullErr)
			}
		}
	})

	t.Run("CheckStatsConcurrent", func(t *testing.T) {
		wg := sync.WaitGroup{}
		stats := make(chan Stats, config.NWorkers)
		errs := make(chan error, config.NWorkers)
		for w := 0; w < config.NWorkers; w++ {
			wg.Add(1)
			go func(w int, wg *sync.WaitGroup) {
				defer wg.Done()
				s, err := wp.getWorkerStats(w)
				stats <- s
				errs <- err
			}(w, &wg)
		}
		wg.Wait()
		close(stats)
		close(errs)

		for {
			select {
			case err := <-errs:
				if err != nil {
					t.Fatal(err)
				}
			case s, opened := <-stats:
				if !opened {
					return
				}
				t.Logf("Worker stats: %v\n", s)
			}
		}
	})

	t.Run("ReloadWorkerConcurrent", func(t *testing.T) {
		wg := sync.WaitGroup{}
		errs := make(chan error, config.NWorkers)
		for w := 0; w < config.NWorkers; w++ {
			wg.Add(1)
			go func(w int, wg *sync.WaitGroup) {
				defer wg.Done()
				err := wp.reloadWorker(w)
				errs <- err
			}(w, &wg)
		}
		wg.Wait()
		close(errs)

		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}

		for w := 0; w < config.NWorkers; w++ {
			s, err := wp.getWorkerStats(w)
			if err != nil {
				t.Fatal(err)
			}
			if s.ProcessedJobs != 0 || s.TotalJobsExecutionTime != 0 {
				t.Fatal(workerStatsNotNullErr)
			}
			t.Logf("Worker %v stats: %v\n", w, s)
		}
	})
}
