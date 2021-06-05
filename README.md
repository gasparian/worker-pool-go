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
New(config Config, balancer BalancingStrategy) *Manager
(m *Manager) ScheduleJob(f JobFunc) (chan Result, error)

NewRoundRobin() *RoundRobin
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
        NWorkers: 3,  // Number of workers to spawn
        MaxJobs:  10, // Max jobs in a queue, new jobs will be rejected with an error
    }
    balancer := wp.NewRoundRobin()
    pool := wp.New(config, balancer)

    nJobs := 50
    results := make([]chan wp.Result, 0)
    for i := 0; i < nJobs; i++ {
        ch, err := pool.ScheduleJob(exampleJob(i))
        if err != nil {
            break
        }
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
