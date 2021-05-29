package main

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	maxJobsLimitReachedErr      = errors.New("Maximum number of running jobs has been reached")
	workerIndexIsOutOfBoundsErr = errors.New("Worker index is out of bounds")
)

type jobFunc func() result

type job struct {
	f          jobFunc
	resultChan chan result
}

type result struct {
	data interface{}
	err  error
}

type stats struct {
	ProcessedJobs    int64
	TotalElapsedTime int64 // NOTE: nanoseconds
}

type workerStats struct {
	stats
	mx sync.RWMutex
}

type WorkerPoolConfig struct {
	NWorkers      int
	MaxJobs       int64
	MaxJobsReload int64
}

type WorkerPool struct {
	config        WorkerPoolConfig
	workersChan   []chan job
	workersStats  []*workerStats
	terminateChan []chan bool
}

func worker(jobs chan job, terminate chan bool, s *workerStats) {
	for {
		select {
		case j := <-jobs:
			start := time.Now()
			res := j.f()
			elapsedTime := time.Since(start).Nanoseconds()
			s.mx.Lock()
			s.ProcessedJobs++
			s.TotalElapsedTime += elapsedTime
			s.mx.Unlock()
			j.resultChan <- res
			close(j.resultChan)
		case t := <-terminate:
			if t {
				return
			}
		}
	}
}

func NewWorkerPool(config WorkerPoolConfig) *WorkerPool {
	pool := &WorkerPool{
		config:        config,
		workersChan:   make([]chan job, config.NWorkers),
		workersStats:  make([]*workerStats, config.NWorkers),
		terminateChan: make([]chan bool, config.NWorkers),
	}
	for w := 0; w < config.NWorkers; w++ {
		pool.workersChan[w] = make(chan job, config.MaxJobs)
		pool.terminateChan[w] = make(chan bool)
		pool.workersStats[w] = &workerStats{}
		go worker(
			pool.workersChan[w],
			pool.terminateChan[w],
			pool.workersStats[w],
		)
	}
	return pool
}

func (wp WorkerPool) GetCurrentJobsNumber() int64 {
	var currentJobsN int64 = 0
	for _, ch := range wp.workersChan {
		currentJobsN += int64(len(ch))
	}
	return currentJobsN
}

func (wp *WorkerPool) GetWorkerStats(workerId int) (stats, error) {
	resStats := stats{}
	if workerId > len(wp.workersChan) || workerId < 0 {
		return resStats, workerIndexIsOutOfBoundsErr
	}
	s := wp.workersStats[workerId]
	s.mx.RLock()
	defer s.mx.RUnlock()
	resStats.ProcessedJobs = s.ProcessedJobs
	resStats.TotalElapsedTime = s.TotalElapsedTime
	return resStats, nil
}

func (wp *WorkerPool) TerminateWorker(workerId int) error {
	if workerId > len(wp.terminateChan) || workerId < 0 {
		return workerIndexIsOutOfBoundsErr
	}
	wp.terminateChan[workerId] <- true
	return nil
}

func (wp *WorkerPool) ReloadWorker(workerId int) error {
	if workerId > len(wp.workersChan) || workerId < 0 {
		return workerIndexIsOutOfBoundsErr
	}
	err := wp.TerminateWorker(workerId)
	if err != nil {
		return err
	}
	wp.workersStats[workerId] = &workerStats{}
	go worker(
		wp.workersChan[workerId],
		wp.terminateChan[workerId],
		wp.workersStats[workerId],
	)
	return nil
}

type WorkerPoolManager interface {
	ScheduleJob(f jobFunc) (chan result, error)
}

/*------------------------------------------------------------------------*/
// NOTE: default implmementation of the WorkerPoolManager

type RoundRobinWorkerPool struct {
	*WorkerPool
	mx           sync.RWMutex
	nextWorkerId int
}

func NewWorkerPoolRoundRobin(pool *WorkerPool) *RoundRobinWorkerPool {
	return &RoundRobinWorkerPool{
		WorkerPool:   pool,
		nextWorkerId: 0,
	}
}

func (rr *RoundRobinWorkerPool) ScheduleJob(f jobFunc) (chan result, error) {
	rr.mx.RLock()
	defer rr.mx.RUnlock()

	currentJobsN := rr.GetCurrentJobsNumber()
	if currentJobsN >= rr.config.MaxJobs {
		return nil, maxJobsLimitReachedErr
	}
	ch := make(chan result)
	rr.workersChan[rr.nextWorkerId] <- job{f, ch}
	rr.nextWorkerId = (rr.nextWorkerId + 1) % rr.config.NWorkers
	return ch, nil
}

/*------------------------------------------------------------------------*/

func ExampleJob(inp int) jobFunc {
	return func() result {
		res := inp * inp
		return result{
			data: res,
			err:  nil,
		}
	}
}

/*------------------------------------------------------------------------*/

/*
TODO: check worker termination and reload
*/

func main() {
	config := WorkerPoolConfig{
		NWorkers:      3,
		MaxJobs:       10,
		MaxJobsReload: 3, // TODO: not used yet
	}
	wp := NewWorkerPool(config)
	roundRobinPool := NewWorkerPoolRoundRobin(wp)

	nJobs := 50
	jobs := make([]chan result, 0)
	for i := 0; i < nJobs; i++ {
		ch, err := roundRobinPool.ScheduleJob(ExampleJob(i))
		if err != nil {
			fmt.Println("WARNING: ", err)
			break
		}
		jobs = append(jobs, ch)
	}

	fmt.Println("INFO: Getting the results...")
	for i, j := range jobs {
		fmt.Println(i, <-j)
	}

	for w := 0; w < config.NWorkers; w++ {
		s, err := roundRobinPool.GetWorkerStats(w)
		if err != nil {
			fmt.Println("WARNING: ", err)
		}
		fmt.Printf("Worker %v stats: %v\n", w, s)
	}
}
