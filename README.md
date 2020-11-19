# `chanque`

[![MIT License](https://img.shields.io/github/license/octu0/chanque)](https://github.com/octu0/chanque/blob/master/LICENSE)
[![GoDoc](https://godoc.org/github.com/octu0/chanque?status.svg)](https://godoc.org/github.com/octu0/chanque)
[![Go Report Card](https://goreportcard.com/badge/github.com/octu0/chanque)](https://goreportcard.com/report/github.com/octu0/chanque)
[![Releases](https://img.shields.io/github/v/release/octu0/chanque)](https://github.com/octu0/chanque/releases)

`chanque` provides simple framework for asynchronous programming and goroutine management and safe use of channel.

## Installation

```
$ go get github.com/octu0/chanque
```

## Usage

### Queue

Queue implementation.  
It provides blocking and non-blocking methods, as well as panic handling of channels.

```go
func main() {
	que1 := chanque.NewQueue(10)
	defer que1.Close()

	go func() {
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
func main() {
	// minWorker 1 maxWorker 2
	exec := chanque.NewExecutor(1, 2)
	defer exec.Release()

	exec.Submit(func() {
		fmt.Println("job1")
		time.Sleep(1 * time.Second)
	})
	exec.Submit(func() {
		fmt.Println("job2")
		time.Sleep(1 * time.Second)
	})

	// Blocking because it became the maximum number of workers,
	// executing when there are no more workers running
	exec.Submit(func() {
		fmt.Println("job3")
	})

	// Generate goroutines on demand up to the maximum number of workers.
	// Submit does not block up to the size of MaxCapacity
	// Workers that are not running are recycled to minWorker at the time of ReduceInterval.
	exec2 := chanque.NewExecutor(10, 50,
		chanque.ExecutorMaxCapacicy(1000),
		chanque.ExecutorReducderInterval(60*time.Second),
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

	// On-demand tune min/max worker size
	exec.TuneMaxWorker(10)
	exec.TuneMinWorker(5)
}
```

### Worker

Worker implementation for asynchronous execution, register WorkerHandler and execute it with Enqueue parameter.  
Enqueue of parameter is blocked while WorkerHandler is running.  
There is also a BufferWorker implementation that non-blocking enqueue during asynchronous execution.

```go
func main() {
	handler := func(param interface{}) {
		if s, ok := param.(string); ok {
			fmt.Println(s)
		}
		time.Sleep(1 * time.Second)
	}

	// DefaultWorker executes in order, waiting for the previous one
	w1 := chanque.NewDefaultWorker(handler)
	defer w1.Shutdown()

	go func() {
		w1.Enqueue("hello")
		w1.Enqueue("world") // blocking during 1 sec
	}()

	w2 := chanque.NewBufferWorker(handler)
	defer w2.Shutdown()

	go func() {
		w2.Enqueue("hello")
		w2.Enqueue("world") // non-blocking
	}()

	// BufferWorker provides helpers for performing sequential operations
	// by using PreHook and PostHook to perform the operations collectively.
	pre := func() {
		db.Begin()
	}
	post := func() {
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

### Parallel

Parallel provides for executing in parallel and acquiring the execution result.  
extended implementation of Worker.

```go
func main() {
	executor := chanque.NewExecutor(10, 100)
	defer executor.Release()

	para := chanque.NewParallel(
		executor,
		chanque.Parallelism(2),
	)
	para.Queue(func() (interface{}, error) {
		return "#1 result", nil
	})
	para.Queue(func() (interface{}, error) {
		return "#2 result", nil
	})
	para.Queue(func() (interface{}, error) {
		return nil, errors.New("#3 error")
	})

	future := para.Submit()
	for _, r := range future.Result() {
		if r.Value() != nil {
			println("result:", r.Value().(string))
		}
		if r.Err() != nil {
			println("error:", r.Err().Error())
		}
	}
}
```

### Retry

Retry provides function retry based on the exponential backoff algorithm.

```go
func main() {
	retry := chanque.NewRetry(
		chanque.RetryMax(10),
		chanque.RetryBackoffIntervalMin(100*time.Millisecond),
		chanque.RetryBackoffIntervalMax(30*time.Second),
	)
	future := retry.Retry(func(ctx context.Context) (interface{}, error) {
		req, _ := http.NewRequest("GET", url, nil)
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		...
		return ioutil.ReadAll(resp.Body), nil
	})
	r := future.Result()
	if err := r.Err(); err != nil {
		panic(err.Error())
	}
	fmt.Printf("GET resp = %s", r.Value().([]byte))
}
```

### Pipeline

Pipeline provides sequential asynchronous input and output.
Execute func combination asynchronously

```go
func main() {
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
func main() {
	e := chanque.NewExecutor(1, 10)
	defer e.Relase()

	heavyProcessA := func(done chanque.DoneFunc) {
		defer done()
		time.Sleep(1 * time.Second) // heavy process
	}
	heavyProcessB := func(done chanque.DoneFunc) {
		defer done()
		time.Sleep(5 * time.Second) // heavy process
	}

	ctx := chanque.NewContext(e, func() {
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
	ctxTO := chanque.NewContextTimeout(e, func() {
		fmt.Printf("all done w/o sub process done")
	}, 1*time.Second)

	go func(d chanque.DoneFunc) {
		heavyProcessA(d)
	}(ctxTO.Add())

	go func(d chanque.DoneFunc) {
		heavyProcessB(d)
	}(ctxTO.Add())

	ctxTO.Background()
}
```

### Loop

Loop provides safe termination of an infinite loop by goroutine.  
You can use callbacks with Queue and time.Ticker.  

```go
func newloop() {
	e := NewExecutor(1, 10)

	queue := NewQueue(0)

	loop := NewLoop(e)
	loop.SetDequeue(func(val interface{}, ok bool) chanque.LoopNext {
		if ok != true {
			// queue closed
			return chanque.LoopNextBreak
		}
		println("queue=", val.(string))
		return chanque.LoopNextContinue
	}, queue)

	loop.ExecuteTimeout(10 * time.Second)

	go func() {
		queue.Enqueue("hello1")
		queue.Enqueue("hello2")
		time.Sleep(1 * time.Second)
		queue.EnqueueNB("world") // Enqueue / EnqueueNB / EnqueueRetry
	}()
	go func() {
		time.Sleep(1 * time.Second)
		loop.Stop() // done for loop
	}()
}
```

## Documentation

https://godoc.org/github.com/octu0/chanque

# Benchmark

## `go func()` vs `Executor`

```bash
$ go test -v -run=BenchmarkExecutor -bench=BenchmarkExecutor -benchmem  ./
goos: darwin
goarch: amd64
pkg: github.com/octu0/chanque
BenchmarkExecutor/goroutine-8         	 1000000	      2306 ns/op	     544 B/op	       2 allocs/op
BenchmarkExecutor/executor/100-1000-8 	  952410	      1252 ns/op	      16 B/op	       1 allocs/op
BenchmarkExecutor/executor/1000-5000-8    795402	      1327 ns/op	      18 B/op	       1 allocs/op
--- BENCH: BenchmarkExecutor
    executor_test.go:19: goroutine           	TotalAlloc=546437344	StackInUse=1996357632
    executor_test.go:19: executor/100-1000   	TotalAlloc=25966144	StackInUse=-1993277440
    executor_test.go:19: executor/1000-5000  	TotalAlloc=16092752	StackInUse=7012352
PASS
ok  	github.com/octu0/chanque	6.935s
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

NewExecutor(minWorker, maxWorker int, fn ...ExecutorOptionFunc) *Executor

func(*Executor) Submit(Job)
func(*Executor) Release()
func(*Executor) ReleaseAndWait()
func(*Executor) Running() int32
func(*Executor) Workers() int32
func(*Executor) SubExecutor() *SubExecutor
func(*Executor) MinWorker() int
func(*Executor) MaxWorker() int
func(*Executor) TuneMinWorker(nextMinWorker int)
func(*Executor) TuneMaxWorker(nextMaxWorker int)

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

### type `Parallel`

```
type ParallelJob func() (result interface{}, err error)
NewParallel(*Executor) *Parallel

func (*Parallel) Queue(ParallelJob)
func (*Parallel) Submit() *ParallelFuture

func (*ParallelFuture) Result() []ValueError

func (ValueError) Value() interface{}
func (ValueError) Err()   error
```

### type `Retry`

```
type RetryFunc func(context.Context) (result interface{}, err error)
type RetryErrorHandler func(err error, b *Backoff) RetryNext

NewBackoff(min,max time.Duration) *Backoff
NewBackoffNoJitter(min,max time.Duration) *Backoff

NewRetry(*Executor) *Retry
NewRetryWithBackoff(*Executor, *Backoff)

func (*Retry) Retry(RetryFunc) *RetryFuture
func (*Retry) RetryWithErrorHandler(RetryFunc, RetryErrorHandler) *RetryFuture

func (*RetryFuture) Result() ValueError

func (ValueError) Value() interface{}
func (ValueError) Err()   error
```

### type `Pipeline`

```
NewPipeline(PipelineInputFunc, PipelineOutputFunc, fn ...PipelineOptionFunc) *Pipeline

type PipelineInputFunc  func(parameter interface{}) (result interface{}, err error)
type PipelineOutputFunc func(result interface{}, err error)

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

### type `Loop`

```
NewLoop(e *Executor, fn ...LoopOptionFunc) *Loop

const(
  LoopNextContinue LoopNext = 1
  LoopNextBreak    LoopNext = 2
)

type LoopDefaultHandler  func() LoopNext
type LoopDequeueHandler  func(val interface{}, ok bool) LoopNext
type LoopTickerHandler   func() LoopNext

func(*Loop) SetDefeult(LoopDefaultHandler) // select { case ...; default: }
func(*Loop) SetTicker(LoopTickerHandler, time.Duration) // select { case <-tick.C: }
func(*Loop) SetDequeue(LoopDequeueHandler, *Queue)  // select { case val, ok <-queue: }
func(*Loop) Execute()
func(*Loop) ExecuteTimeout(time.Duration)
func(*Loop) Stop()
func(*Loop) StopAndWait()
```

## License

MIT, see LICENSE file for details.
