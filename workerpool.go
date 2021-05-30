package workerpool

import (
	"errors"
	"sync"
	"time"
)

var (
	maxJobsLimitReachedErr      = errors.New("Maximum number of running jobs has been reached")
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
	ProcessedJobs    int64
	TotalElapsedTime int64 // NOTE: nanoseconds
}

type workerStats struct {
	Stats
	mx sync.RWMutex
}

// Config holds needed data to start worker pool
type Config struct {
	NWorkers int
	MaxJobs  int64
}

// WorkerPool holds data needed for pool operation
type WorkerPool struct {
	config        Config
	workersChan   []chan Job
	workersStats  []*workerStats
	terminateChan []chan bool
}

func worker(jobs chan Job, terminate chan bool, s *workerStats) {
	for {
		select {
		case j := <-jobs:
			start := time.Now()
			res := j.F()
			elapsedTime := time.Since(start).Nanoseconds()
			s.mx.Lock()
			s.ProcessedJobs++
			s.TotalElapsedTime += elapsedTime
			s.mx.Unlock()
			j.ResultChan <- res
			close(j.ResultChan)
		case t := <-terminate:
			if t {
				return
			}
		}
	}
}

// New created new worker pool
func New(config Config) *WorkerPool {
	pool := &WorkerPool{
		config:        config,
		workersChan:   make([]chan Job, config.NWorkers),
		workersStats:  make([]*workerStats, config.NWorkers),
		terminateChan: make([]chan bool, config.NWorkers),
	}
	for w := 0; w < config.NWorkers; w++ {
		pool.workersChan[w] = make(chan Job, config.MaxJobs)
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

// GetCurrentJobsNumber calculates amount of jobs across all workers queues
func (wp WorkerPool) GetCurrentJobsNumber() int64 {
	var currentJobsN int64 = 0
	for _, ch := range wp.workersChan {
		currentJobsN += int64(len(ch))
	}
	return currentJobsN
}

// GetWorkerStats  by specified worker id
func (wp *WorkerPool) GetWorkerStats(workerId int) (Stats, error) {
	resStats := Stats{}
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

// TerminateWorker sends termination signal to the specified worker
func (wp *WorkerPool) TerminateWorker(workerId int) error {
	if workerId > len(wp.terminateChan) || workerId < 0 {
		return workerIndexIsOutOfBoundsErr
	}
	wp.terminateChan[workerId] <- true
	return nil
}

// ReloadWorker terminates worker by id, and spawns new one
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

// Manafer interface that holds implementation of balancing strategy
type Manager interface {
	ScheduleJob(f JobFunc) (chan Result, error)
}

/*------------------------------------------------------------------------*/
// NOTE: default implmementation of the Manager

// RoundRobin evenely distribute jobs across workers
type RoundRobin struct {
	*WorkerPool
	mx           sync.RWMutex
	nextWorkerId int
}

// NewRoundRobin ...
func NewRoundRobin(pool *WorkerPool) *RoundRobin {
	return &RoundRobin{
		WorkerPool:   pool,
		nextWorkerId: 0,
	}
}

// ScheduleJob puts job in a queue
func (rr *RoundRobin) ScheduleJob(f JobFunc) (chan Result, error) {
	rr.mx.RLock()
	defer rr.mx.RUnlock()

	currentJobsN := rr.GetCurrentJobsNumber()
	if currentJobsN >= rr.config.MaxJobs {
		return nil, maxJobsLimitReachedErr
	}
	ch := make(chan Result)
	rr.workersChan[rr.nextWorkerId] <- Job{f, ch}
	rr.nextWorkerId = (rr.nextWorkerId + 1) % rr.config.NWorkers
	return ch, nil
}
