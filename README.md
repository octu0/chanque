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

### Wait

Wait provides wait handling like sync.WaitGroup and context.Done.  
Provides implementations for patterns that run concurrently, wait for multiple processes, wait for responses, and many other use cases.

```go
func one() {
	w := WaitOne()
	defer w.Cancel()

	go func(w *Wait) {
		defer w.Done()

		fmt.Println("heavy process")
	}(w)

	w.Wait()
}

func any() {
	w := WaitN(10)
	defer w.Cancel()

	for i := 0; i < 10; i += 1 {
		go func(w *Wait) {
			defer w.Done()

			fmt.Println("N proc")
		}(w)
	}
	w.Wait()
}

func sequencial() {
	w1, := WaitOne()
	defer w1.Cancel()
	go Preprocess(w1)

	w2 := WaitOne()
	defer w2.Cancel()
	go Preprocess(w2)

	ws := WaitSeq(w1, w2)
	defer ws.Cancel()

	// Wait for A.Done() -> B.Done() -> ... N.Done() ordered
	ws.Wait()
}

func rendezvous() {
	wr := WaitRendez(2)
	defer wr.Cancel()

	go func() {
		if err := wr.Wait(); err != nil {
			fmt.Println("timeout or cancel")
			return
		}
		fmt.Println("run sync")
	}()
	go func() {
		if err := wr.Wait(); err != nil {
			fmt.Println("timeout or cancel")
			return
		}
		fmt.Println("run sync")
	}()
}

func req() {
	wreq := WaitReq()
	defer wreq.Cancel()

	go func() {
		if err := wreq.Req("hello world"); err != nil {
			fmt.Println("timeout or cancel")
		}
		fmt.Println("send req")
	}()

	v, err := wreq.Wait()
	if err != nil {
		fmt.Println("timeout or cancel")
	}
	fmt.Println(v.(string)) // => "hello world"
}

func reqreply() {
	wrr := WaitReqReply()
	go func() {
		v, err := wrr.Req("hello")
		if err != nil {
			fmt.Println("timeout or cancel")
		}
		fmt.Println(v.(string)) // => "hello world2"
	}()
	go func() {
		err := wrr.Reply(func(v interface{}) (interface{}, err) {
			s := v.(string)
			return s + " world2", nil
		})
		if err != nil {
			fmt.Println("timeout or cancel")
		}
	}()
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

## License

MIT, see LICENSE file for details.
