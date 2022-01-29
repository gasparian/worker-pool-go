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
(wp *workerPool) getCurrentJobsNumber() int64
(wp *workerPool) getWorkerStats(workerId int) (Stats, error)
(wp *workerPool) terminateWorker(workerId int) error
(wp *workerPool) reloadWorker(workerId int) error

// key public structs and methods
// Config holds worker pool params, like 
// it's size and max jobs per worker (without blocking)
type Config struct {
	NWorkers int
	MaxJobs  int64
}
// New creates new instance of workers pool manager
New(config Config, balancer BalancingStrategy) *Manager
// ScheduleJob puts new job in some worker queue
(m *Manager) ScheduleJob(f JobFunc) (chan Result, error)
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
    config := wp.Config{
        NWorkers: 3,  // NOTE: Number of workers to spawn
        MaxJobs:  10, // NOTE: Max length of the buffered 
                      //       channel with submitted jobs (per worker)
    }
    balancer := wp.NewRoundRobin()
    pool := wp.New(config, balancer)

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
