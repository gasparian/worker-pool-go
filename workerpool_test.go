package workerpool

import (
	"errors"
	"math"
	"sync"
	"testing"
)

var (
	jobsShouldBeBelowLimitErr  = errors.New("Number of processed jobs should be below the limit")
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
	wp := New(config)
	roundRobinPool := NewRoundRobin(wp)

	var actualProcessedJobs float64 = 0.0
	nJobs := 50

	t.Run("RunJobsSync", func(t *testing.T) {
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
			t.Error(jobsShouldBeBelowLimitErr)

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
			t.Error(workerStatsNotNullErr)
		}
	})

	t.Run("RunJobsConcurrent", func(t *testing.T) {
		for w := 0; w < config.NWorkers; w++ {
			err := roundRobinPool.ReloadWorker(w)
			if err != nil {
				t.Fatal(err)
			}
		}

		jobs := make(chan chan Result, nJobs)
		wg := sync.WaitGroup{}
		for i := 0; i < nJobs; i++ {
			wg.Add(1)
			go func(idx int, wg *sync.WaitGroup) {
				defer wg.Done()
				ch, err := roundRobinPool.ScheduleJob(exampleJob(idx))
				if err == nil {
					jobs <- ch
				}
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
			s, err := roundRobinPool.GetWorkerStats(w)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("Worker %v stats: %v\n", w, s)
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
				s, err := roundRobinPool.GetWorkerStats(w)
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
				err := roundRobinPool.ReloadWorker(w)
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
			s, err := roundRobinPool.GetWorkerStats(w)
			if err != nil {
				t.Fatal(err)
			}
			if s.ProcessedJobs != 0 || s.TotalElapsedTime != 0 {
				t.Fatal(workerStatsNotNullErr)
			}
			t.Logf("Worker %v stats: %v\n", w, s)
		}
	})
}
