![tests](https://github.com/gasparian/worker-pool-go/actions/workflows/test.yml/badge.svg?branch=main)

# worker-pool-go  

Implementation of worker pool with extendable functionality.  
By default, I've implemented "round-robin" balancing to evenly distribute jobs across workers.  
You need to implement the `BalancingStrategy` interface in order to use other balancing strategies:  
```go
type BalancingStrategy interface {
	NextWorkerId(workersStats []Stats) int
}
```  

API:  
```go
// worker pool struct private api
(wp *WorkerPool) getCurrentJobsNumber() int64
(wp *WorkerPool) getWorkerStats(workerId int) (Stats, error)
(wp *WorkerPool) terminateWorker(workerId int) error
(wp *WorkerPool) reloadWorker(workerId int) error

// worker pool public methods
// New creates new instance of workers pool
// params defining it's size and max jobs per worker (without blocking)
NewWorkerPool(nWorkers, maxJobs uint, balancer BalancingStrategy) *WorkerPool
// ScheduleJob puts new job in some worker queue
(wp *WorkerPool) ScheduleJob(f JobFunc) (chan Result, error)

// NewRoundRobin creates new instance of the "load balancer"
NewRoundRobin() *RoundRobin
// NextWorkerId returns worker id selected by some
// predefined policy
(rr *RoundRobin) NextWorkerId(workerStats []Stats) int
```  

Install:  
```
go get -u github.com/gasparian/worker-pool-go
```  

Usage example:  
```go
package main

import (
    "fmt"
    wp "github.com/gasparian/worker-pool-go"
)

// Use closure to create job function
func exampleJob(inp int) wp.JobFunc {
    return func() wp.Result {
        res := inp * inp
        return wp.Result{
            Data: res,
            Err:  nil,
        }
    }
}

func main() {
    pool := wp.New(config, balancer)
	pool := NewWorkerPool(
		3,
		10,
		NewRoundRobin(),
	)

    // NOTE: if number of jobs per woker will be > MaxJobs, 
    //       ScheduleJob func will block
    nJobs := 50
    results := make([]chan wp.Result, 0)
    for i := 0; i < nJobs; i++ {
        ch := pool.ScheduleJob(exampleJob(i))
        results = append(results, ch)
    }
    
    for _, r := range results {
        res := <-r
        fmt.Println(res)
    }
}
```  

Run tests:  
```
make test
```  
