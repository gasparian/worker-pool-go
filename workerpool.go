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
	ProcessedJobs          int64
	TotalJobsExecutionTime int64 // NOTE: nanoseconds
}

// Config holds needed data to start worker pool
type Config struct {
	NWorkers int
	MaxJobs  int64
}

type workerStats struct {
	Stats
	mx sync.RWMutex
}

// workerPool holds data needed for pool operation
type workerPool struct {
	mx            sync.RWMutex
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
			s.TotalJobsExecutionTime += elapsedTime
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

// getCurrentJobsAmount calculates amount of jobs across all workers queues
func (wp *workerPool) getCurrentJobsAmount() int64 {
	wp.mx.RLock()
	defer wp.mx.RUnlock()

	var currentJobsAmount int64 = 0
	for _, ch := range wp.workersChan {
		currentJobsAmount += int64(len(ch))
	}
	return currentJobsAmount
}

// getWorkerStats by specified worker id
func (wp *workerPool) getWorkerStats(workerId int) (Stats, error) {
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
func (wp *workerPool) terminateWorker(workerId int) error {
	if workerId > len(wp.terminateChan) || workerId < 0 {
		return workerIndexIsOutOfBoundsErr
	}
	wp.mx.RLock()
	wp.terminateChan[workerId] <- true
	wp.mx.RUnlock()
	return nil
}

// reloadWorker terminates worker by id, and spawns new one
func (wp *workerPool) reloadWorker(workerId int) error {
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

// Strategy interface that holds implementation of balancing strategy
type BalancingStrategy interface {
	NextWorkerId(workersStats []Stats) int
}

// Manager spawns workers and schedule jobs
type Manager struct {
	wp       *workerPool
	balancer BalancingStrategy
}

// New creates new worker pool
func New(config Config, balancer BalancingStrategy) *Manager {
	pool := &workerPool{
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
	manager := &Manager{
		wp:       pool,
		balancer: balancer,
	}
	// TODO: add routine that reloads workers
	return manager
}

// ScheduleJob puts job in a queue
func (m *Manager) ScheduleJob(f JobFunc) (chan Result, error) {
	m.wp.mx.RLock()
	config := m.wp.config
	workersStats := make([]Stats, config.NWorkers)
	for i, s := range m.wp.workersStats {
		s.mx.RLock()
		workersStats[i] = s.Stats
		s.mx.RUnlock()
	}
	m.wp.mx.RUnlock()

	currentJobsAmount := m.wp.getCurrentJobsAmount()
	if currentJobsAmount >= config.MaxJobs {
		return nil, maxJobsLimitReachedErr
	}
	ch := make(chan Result)

	nextWorkerId := m.balancer.NextWorkerId(workersStats)
	m.wp.workersChan[nextWorkerId] <- Job{f, ch}
	return ch, nil
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
