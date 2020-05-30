# `chanque`

[![MIT License](https://img.shields.io/github/license/octu0/chanque)](https://github.com/octu0/chanque/blob/master/LICENSE)
[![GoDoc](https://godoc.org/github.com/octu0/chanque?status.svg)](https://godoc.org/github.com/octu0/chanque)
[![Go Report Card](https://goreportcard.com/badge/github.com/octu0/chanque)](https://goreportcard.com/report/github.com/octu0/chanque)
[![Releases](https://img.shields.io/github/v/release/octu0/chanque)](https://github.com/octu0/chanque/releases)

`chanque` provides a simple workload for channel and goroutine.

## Documentation

https://godoc.org/github.com/octu0/chanque

## Installation

```
$ go get github.com/octu0/chanque
```

## Usage

### Queue

Queue implementation.  
It provides blocking and non-blocking methods, as well as panic handling of channels.

```go
import(
  "fmt"
  "github.com/octu0/chanque"
)

func main(){
  que1 := chanque.NewQueue(10)
  defer que1.Close()

  go func(){
    for {
      val := que1.Dequeue()
      fmt.Println(val.(string))
    }
  }()
  if ok := que1.Enqueue("hello"); ok {
    fmt.Println("enqueue")
  }

  que2 := chanque.NewQueue(10,
    QueuePanicHandler(func(pt chanque.PanicType, rcv interface{}) {
      fmt.Println("panic occurred", rcv.(error))
    }),
  )
  defer que2.Close()
  if ok := que2.EnqueueNB("world w/ non-blocking enqueue"); ok {
    fmt.Println("enqueue")
  }
}
```

### Executor

WorkerPool implementation,  
which limits the number of concurrent executions of goroutines and creates goroutines as needed,  
and can also be used as goroutine resource management.

```go
import(
  "fmt"
  "time"
  "github.com/octu0/chanque"
)

func main(){
  // minWorker 1 maxWorker 2 
  exec := chanque.NewExecutor(1, 2) 
  defer exec.Release()

  exec.Submit(func(){
    fmt.Println("job1")
    time.Sleep(1 * time.Second)
  })
  exec.Submit(func(){
    fmt.Println("job2")
    time.Sleep(1 * time.Second)
  })

  // Blocking because it became the maximum number of workers, 
  // executing when there are no more workers running
  exec.Submit(func(){
    fmt.Println("job3")
  })

  // Generate goroutines on demand up to the maximum number of workers.
  // Submit does not block up to the size of MaxCapacity
  // Workers that are not running are recycled to minWorker at the time of ReduceInterval.
  exec2 := chanque.NewExecutor(10, 50,
    chanque.ExecutorMaxCapacicy(1000),
    chanque.ExecutorReducderInterval(60 * time.Second),
  )
  defer exec2.Release()

  for i := 0; i < 100; i += 1 {
    exec2.Submit(func(i id) func() {
      return func() {
        fmt.Println("heavy process", id)
        time.Sleep(100 * time.Millisecond)
        fmt.Println("done process", id)
      }
    }(i))
  }
}
```

### Worker

Worker implementation for asynchronous execution, register WorkerHandler and execute it with Enqueue parameter.  
Enqueue of parameter is blocked while WorkerHandler is running.  
There is also a BufferWorker implementation that non-blocking enqueue during asynchronous execution.

```go
import(
  "fmt"
  "time"
  "context"
  "github.com/octu0/chanque"
)

func main(){
  handler := func(param interface{}) {
    if s, ok := param.(string); ok {
      fmt.Println(s)
    }
    time.Sleep(1 * time.Second)
  }
  w1 := chanque.NewDefaultWorker(handler)
  defer w1.Shutdown()

  go func(){
    w1.Enqueue("hello")
    w1.Enqueue("world") // blocking during 1 sec
  }()

  w2 := chanque.NewBufferWorker(handler)
  defer w2.Shutdown()

  go func(){
    w2.Enqueue("hello")
    w2.Enqueue("world") // non-blocking
  }()

  // BufferWorker provides helpers for performing sequential operations
  // by using PreHook and PostHook to perform the operations collectively.
  pre := func(){
    db.Begin()
  }
  post := func(){
    db.Commit()
  }
  hnd := func(param interface{}) {
    db.Insert(param.(string))
  }
  w3 := chanque.NewBufferWorker(hnd,
    WorkerPreHook(pre),
    WorkerPostHook(post),
  )
  for i := 0; i < 100; i += 1 {
    w3.Enqueue(strconv.Itoa(i))
  }
  w3.ShutdownAndWait()
}
```

### Pipeline

Pipeline provides sequential asynchronous input and output.
Execute func combination asynchronously

```go
import(
  "fmt"
  "time"
  "context"
  "github.com/octu0/chanque"
)

func main(){
  calcFn := func(parameter interface{}) (interface{}, error) {
    // heavy process
    time.Sleep(1 * time.Second)

    if val, ok := parameter.(int); ok {
      return val * 2, nil
    }
    return -1, fmt.Errorf("invalid parameter")
  }
  outFn := func(result interface{}, err error) {
    if err != nil {
       fmt.Fatal(err)
       return
    }

    fmt.Println("value =", parameter.(int))
  }

  pipe := chanque.NewPipeline(calcFn, outFn)
  pipe.Enqueue(10)
  pipe.Enqueue(20)
  pipe.Enqueue(30)
  pipe.ShutdownAndWait()
}
```

### Context / ContextTimeout

Context/ContextTimeout provides a method for the waiting process  
Both apply and use a function that will be called when all operations are complete  

```go
import(
  "fmt"
  "github.com/octu0/chanque"
)

func main(){
  e := NewExecutor(1, 10)
  defer e.Relase()

  heavyProcessA := func(done chanque.DoneFunc) {
    defer done()
    time.Sleep(1 * time.Second) // heavy process
  }
  heavyProcessB := func(done chanque.DoneFunc) {
    defer done()
    time.Sleep(5 * time.Second) // heavy process
  }

  ctx := chanque.NewContext(e, func(){
    fmt.Printf("all done")
  })
  go func(d chanque.DoneFunc) {
    heavyProcessA(d)
  }(ctx.Add())
  go func(d chanque.DoneFunc) {
    heavyProcessB(d)
  }(ctx.Add())
  ctx.Wait()

  // ContextTimeout is used to execute 
  // a process without waiting for each operation to complete.
  ctxTO := chanque.NewContextTimeout(e, func(){
    fmt.Printf("all done w/o sub process done")
  }, 1 * time.Second)
  go func(d chanque.DoneFunc) {
    heavyProcessA(d)
  }(ctxTO.Add())
  go func(d chanque.DoneFunc) {
    heavyProcessB(d)
  }(ctxTO.Add())
  ctxTO.Background()
}
```

## Functions

### type `Queue`

```
NewQueue(capacity int, fn ...QueueOptionFunc) *Queue

func(*Queue) Enqueue(value interface{}) (written bool)
func(*Queue) EnqueueNB(value interface{}) (written bool)
func(*Queue) EnqueueRetry(value interface{}, interval time.Duration, retry int) (written bool)

func(*Queue) Dequeue() (value interface{}, found bool)
func(*Queue) DequeueNB() (value interface{}, found bool)
func(*Queue) DequeueRetry(interval time.Duration, retry int) (value interface{}, found bool)

func(*Queue) Close() (closed bool)
```

### type `Executor`

```
type Job func()

CreateExecutor(minWorker, maxWorker int, fn ...ExecutorOptionFunc) *Executor

func(*Executor) Submit(Job)
func(*Executor) Release()
func(*Executor) ReleaseAndWait()
func(*Executor) Running() int32
func(*Executor) Workers() int32
func(*Executor) SubExecutor() *SubExecutor

func(*SubExecutor) Submit(Job)
func(*SubExecutor) Wait()
```

### type `Worker`

```
type WorkerHandler func(parameter interface{})

NewDefaultWorker(handler WorkerHandler, fn ...WorkerOptionFunc)
NewBufferWorker(handler WorkerHandler, fn ...WorkerOptionFunc)

func(Worker) Enqueue(parameter interface{}) bool
func(Worker) CloseEnqueue() bool
func(Worker) Shutdown()
func(Worker) ShutdownAndWait()
```

### type `Pipeline`

```
CreatePipeline(PipelineInputFunc, PipelineOutputFunc, fn ...PipelineOptionFunc) *Pipeline

func(*Pipeline) Enqueue(parameter interface{}) bool
func(*Pipeline) CloseEnqueue() bool
func(*Pipeline) Shutdown()
func(*Pipeline) ShutdownAndWait()
```

### type `Context` / `ContextTimeout`

```
NewContext(e *Executor, fn DoneFunc) *Context
NewContextTimeout(e *Executor, fn DoneFunc, timeout time.Duration) *ContextTimeout

func(*Context) Add() (cancel DoneFunc)
func(*Context) Wait()
func(*Context) Background()

func(*ContextTimeout) Add() (cancel DoneFunc)
func(*ContextTimeout) Wait()
func(*ContextTimeout) Background()
```

## License

MIT, see LICENSE file for details.
