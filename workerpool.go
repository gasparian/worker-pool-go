package workerpool

import (
	"errors"
	"sync"
	"time"
)

var (
	workerIndexIsOutOfBoundsErr = errors.New("Worker index is out of bounds")
)

// Result holds output data and error
type Result struct {
	Data interface{}
	Err  error
}

// JobFunc is function that contains inside code for the job
type JobFunc func() Result

// Job holds function with the task and channel where job result will be submitted
type Job struct {
	F          JobFunc
	ResultChan chan Result
}

// Stats holds basic stats for the worker
type Stats struct {
	ProcessedJobs          uint
	TotalJobsExecutionTime int64 // NOTE: nanoseconds
}

// Strategy interface that holds implementation of balancing strategy
type BalancingStrategy interface {
	NextWorkerId(workersStats []Stats) int
}

type workerStats struct {
	Stats
	mx sync.RWMutex
}

// workerPool holds data needed for pool operation
type WorkerPool struct {
	mx            sync.RWMutex
	nWorkers      uint
	workersChan   []chan Job
	workersStats  []*workerStats
	terminateChan []chan bool
	balancer      BalancingStrategy
}

// NewWorkerPool creates new worker pool
func NewWorkerPool(nWorkers, maxJobs uint, balancer BalancingStrategy) *WorkerPool {
	pool := &WorkerPool{
		nWorkers:      nWorkers,
		workersChan:   make([]chan Job, nWorkers),
		workersStats:  make([]*workerStats, nWorkers),
		terminateChan: make([]chan bool, nWorkers),
		balancer:      balancer,
	}
	var w uint
	for w = 0; w < nWorkers; w++ {
		pool.workersChan[w] = make(chan Job, maxJobs)
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

func worker(jobs chan Job, terminate chan bool, s *workerStats) {
	var job Job
	for {
		select {
		case job = <-jobs:
			start := time.Now()
			res := job.F()
			elapsedTime := time.Since(start).Nanoseconds()
			s.mx.Lock()
			s.ProcessedJobs++
			s.TotalJobsExecutionTime += elapsedTime
			s.mx.Unlock()
			job.ResultChan <- res
			close(job.ResultChan)
		case t := <-terminate:
			if t {
				return
			}
		}
	}
}

// getWorkerStats by specified worker id
func (wp *WorkerPool) getWorkerStats(workerId int) (Stats, error) {
	resStats := Stats{}
	if workerId > len(wp.workersChan) || workerId < 0 {
		return resStats, workerIndexIsOutOfBoundsErr
	}
	wp.mx.RLock()
	s := wp.workersStats[workerId]
	wp.mx.RUnlock()

	s.mx.RLock()
	defer s.mx.RUnlock()
	resStats.ProcessedJobs = s.ProcessedJobs
	resStats.TotalJobsExecutionTime = s.TotalJobsExecutionTime
	return resStats, nil
}

// terminateWorker sends termination signal to the specified worker
func (wp *WorkerPool) terminateWorker(workerId int) error {
	if workerId > len(wp.terminateChan) || workerId < 0 {
		return workerIndexIsOutOfBoundsErr
	}
	wp.mx.RLock()
	wp.terminateChan[workerId] <- true
	wp.mx.RUnlock()
	return nil
}

// reloadWorker terminates worker by id, and spawns new one
func (wp *WorkerPool) reloadWorker(workerId int) error {
	if workerId > len(wp.workersChan) || workerId < 0 {
		return workerIndexIsOutOfBoundsErr
	}
	err := wp.terminateWorker(workerId)
	if err != nil {
		return err
	}
	wp.mx.Lock()
	wp.workersStats[workerId] = &workerStats{}
	wp.mx.Unlock()
	go worker(
		wp.workersChan[workerId],
		wp.terminateChan[workerId],
		wp.workersStats[workerId],
	)
	return nil
}

// ScheduleJob puts job in a queue
func (wp *WorkerPool) ScheduleJob(f JobFunc) chan Result {
	wp.mx.RLock()
	nWorkers := wp.nWorkers
	workersStats := make([]Stats, nWorkers)
	for i, s := range wp.workersStats {
		s.mx.RLock()
		workersStats[i] = s.Stats
		s.mx.RUnlock()
	}
	wp.mx.RUnlock()
	ch := make(chan Result, 1)
	nextWorkerId := wp.balancer.NextWorkerId(workersStats)
	wp.workersChan[nextWorkerId] <- Job{f, ch}
	return ch
}

/*------------------------------------------------------------------------*/

// NOTE: default balancing strategy

// RoundRobin evenly distributes jobs across workers
type RoundRobin struct {
	mx           sync.RWMutex
	nextWorkerId int
}

// NewRoundRobin ...
func NewRoundRobin() *RoundRobin {
	return &RoundRobin{nextWorkerId: 0}
}

// NextWorkerId generates worker id to schedule the job
func (rr *RoundRobin) NextWorkerId(workerStats []Stats) int {
	rr.mx.Lock()
	defer rr.mx.Unlock()

	workerId := rr.nextWorkerId
	rr.nextWorkerId = (workerId + 1) % len(workerStats)
	return workerId
}
