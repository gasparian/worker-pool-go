![tests](https://github.com/gasparian/worker-pool-go/actions/workflows/test.yml/badge.svg?branch=main)

# worker-pool-go  

Implementation of worker pool with extendable functionality.  
By default, I've implemented "round-robin" balancing to evenly distribute jobs across workers.  
You need to implement the `Manager` interface in order to use other balancing strategies:  
```go
type Manager interface {
	ScheduleJob(f JobFunc) (chan Result, error)
}
```  

API:  
```go
New(config Config) *WorkerPool
(wp WorkerPool) GetCurrentJobsNumber() int64
(wp *WorkerPool) GetWorkerStats(workerId int) (Stats, error)
(wp *WorkerPool) TerminateWorker(workerId int) error
(wp *WorkerPool) ReloadWorker(workerId int) error

NewRoundRobin(pool *WorkerPool) *RoundRobin
(rr *RoundRobin) ScheduleJob(f JobFunc) (chan Result, error)
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
        MaxJobs:  10, // Max jobs in a queue, new jobs will be rejected with error
    }
    pool := wp.New(config)
    roundRobinPool := wp.NewRoundRobin(pool)

    nJobs := 50
    jobs := make([]chan wp.Result, 0)
    for i := 0; i < nJobs; i++ {
        ch, err := roundRobinPool.ScheduleJob(exampleJob(i))
        if err != nil {
            break
        }
        jobs = append(jobs, ch)
    }
    
    for _, j := range jobs {
        res := <-j
        fmt.Println(res)
    }

    for w := 0; w < config.NWorkers; w++ {
        s, err := roundRobinPool.GetWorkerStats(w)
        if err != nil {
            panic(err)
        }
        fmt.Printf("Worker %v stats: %v\n", w, s)
    }
}
```  

Run tests:  
```
make test
```  
