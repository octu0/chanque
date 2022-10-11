package chanque

import (
	"fmt"
	"math/rand"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWorkerSequence(t *testing.T) {
	checkSeq := func(tt *testing.T, f func(h WorkerHandler) Worker) {
		rand.Seed(time.Now().UnixNano())

		c := make([]int, 0)
		s := 30
		m := new(sync.Mutex)
		h := func(p interface{}) {
			m.Lock()
			defer m.Unlock()

			c = append(c, p.(int))

			time.Sleep(time.Duration(rand.Intn(10)+10) * time.Millisecond)
		}
		w := f(h)
		for i := 0; i < s; i += 1 {
			w.Enqueue(i)
		}
		w.ShutdownAndWait()

		tt.Logf("v = %v", c)
		if len(c) != s {
			tt.Errorf("c should be len = %d", s)
		}
		for i := 0; i < s; i += 1 {
			if c[i] != i {
				tt.Errorf("not sequencial c[%d] != %d", i, c[i])
			}
		}
	}
	t.Run("default/default", func(tt *testing.T) {
		checkSeq(tt, func(h WorkerHandler) Worker {
			return NewDefaultWorker(h)
		})
	})
	t.Run("buffer/default", func(tt *testing.T) {
		checkSeq(tt, func(h WorkerHandler) Worker {
			return NewBufferWorker(h)
		})
	})
	t.Run("default/executor10/10", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		checkSeq(tt, func(h WorkerHandler) Worker {
			return NewDefaultWorker(h, WorkerExecutor(e))
		})
	})
	t.Run("buffer/executor10/10", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		checkSeq(tt, func(h WorkerHandler) Worker {
			return NewBufferWorker(h, WorkerExecutor(e))
		})
	})
	t.Run("default/cap5/executor10/10", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		checkSeq(tt, func(h WorkerHandler) Worker {
			return NewDefaultWorker(h, WorkerExecutor(e), WorkerCapacity(5))
		})
	})
	t.Run("buffer/cap5/executor10/10", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		checkSeq(tt, func(h WorkerHandler) Worker {
			return NewBufferWorker(h, WorkerExecutor(e), WorkerCapacity(5))
		})
	})
}

func TestWorkerPrePostHook(t *testing.T) {
	t.Run("default", func(tt *testing.T) {
		logs := make([]string, 0)
		pre := func() {
			logs = append(logs, "begin")
		}
		post := func() {
			logs = append(logs, "commit")
		}
		handler := func(param interface{}) {
			time.Sleep(10 * time.Millisecond)
			logs = append(logs, "insert", param.(string))
		}
		w := NewDefaultWorker(handler,
			WorkerPreHook(pre),
			WorkerPostHook(post),
		)
		w.Enqueue("100")
		w.Enqueue("200")
		w.Enqueue("300")
		w.ShutdownAndWait()

		beginCount := 0
		insertCount := 0
		commitCount := 0
		for _, val := range logs {
			switch val {
			case "begin":
				beginCount += 1
			case "insert":
				insertCount += 1
			case "commit":
				commitCount += 1
			}
		}
		if (beginCount == insertCount && commitCount == insertCount) != true {
			t.Errorf("same times b:%d i:%d c:%d", beginCount, insertCount, commitCount)
		}

		expect := []string{
			"begin",
			"insert", "100",
			"commit",
			"begin",
			"insert", "200",
			"commit",
			"begin",
			"insert", "300",
			"commit",
		}
		expectStr := strings.Join(expect, ",")
		actualStr := strings.Join(logs, ",")
		if expectStr != actualStr {
			tt.Errorf("expect %s != actual %s", expectStr, actualStr)
		}
	})
	t.Run("buffer", func(tt *testing.T) {
		logs := make([]string, 0)
		pre := func() {
			logs = append(logs, "begin")
		}
		post := func() {
			logs = append(logs, "commit")
		}
		handler := func(param interface{}) {
			time.Sleep(10 * time.Millisecond)
			logs = append(logs, "insert", param.(string))
		}
		w := NewBufferWorker(handler,
			WorkerPreHook(pre),
			WorkerPostHook(post),
		)
		for i := 0; i < 10; i += 1 {
			w.Enqueue("1")
		}
		w.ShutdownAndWait()

		beginCount := 0
		insertCount := 0
		commitCount := 0
		for _, val := range logs {
			switch val {
			case "begin":
				beginCount += 1
			case "insert":
				insertCount += 1
			case "commit":
				commitCount += 1
			}
		}
		if (beginCount < insertCount && commitCount < insertCount) != true {
			t.Errorf("begin and commit less than insert  b:%d i:%d c:%d", beginCount, insertCount, commitCount)
		}
	})
}

func TestDefaultWorkerShutdownAndWait(t *testing.T) {
	t.Run("1-1", func(tt *testing.T) {
		h := func(p interface{}) {
			time.Sleep(10 * time.Millisecond)
			tt.Logf("val = %v", p)
		}
		w := NewDefaultWorker(h)
		w.Enqueue(10)
		w.Enqueue(20)
		w.Enqueue(30)
		w.ShutdownAndWait()
	})
	t.Run("1-5", func(tt *testing.T) {
		h := func(p interface{}) {
			time.Sleep(10 * time.Millisecond)
		}
		w := NewDefaultWorker(h)
		for i := 0; i < 10; i += 1 {
			w.Enqueue(i)
		}
		w.ShutdownAndWait()
	})
}

func TestBufferWorkerShutdownAndWait(t *testing.T) {
	t.Run("3", func(tt *testing.T) {
		h := func(p interface{}) {
			time.Sleep(10 * time.Millisecond)
			tt.Logf("val = %v", p)
		}
		w := NewBufferWorker(h, WorkerPanicHandler(noopPanicHandler))
		w.Enqueue(10)
		w.Enqueue(20)
		w.Enqueue(30)
		w.ShutdownAndWait()
	})
	t.Run("10", func(tt *testing.T) {
		h := func(p interface{}) {
			time.Sleep(10 * time.Millisecond)
		}
		w := NewBufferWorker(h, WorkerPanicHandler(noopPanicHandler))
		for i := 0; i < 10; i += 1 {
			w.Enqueue(i)
		}
		w.ShutdownAndWait()
	})
}

func TestWorkerOptionExecutor(t *testing.T) {
	t.Run("default", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		done := make(chan struct{})
		e.Submit(func() {
			// my task
			<-done
		})
		time.Sleep(1 * time.Millisecond)

		r1 := e.Running()
		if r1 != 1 {
			tt.Errorf("running my task 1 != %d", r1)
		}

		w := NewDefaultWorker(func(interface{}) {}, WorkerExecutor(e))
		w.Enqueue(1)
		w.Enqueue(2)
		w.Enqueue(3)

		r2 := e.Running()
		if r2 < 2 {
			tt.Errorf("worker + my task running: %d", r2)
		}

		w.ShutdownAndWait()

		r3 := e.Running()
		if r3 != 1 {
			tt.Errorf("running my task 1 != %d", r1)
		}

		done <- struct{}{}

		time.Sleep(1 * time.Millisecond)

		r4 := e.Running()
		if r4 != 0 {
			tt.Errorf("all task done %d", r1)
		}
	})
	t.Run("buffer", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		done := make(chan struct{})
		e.Submit(func() {
			// my task
			<-done
		})
		time.Sleep(1 * time.Millisecond)

		r1 := e.Running()
		if r1 != 1 {
			tt.Errorf("running my task 1 != %d", r1)
		}

		w := NewBufferWorker(func(interface{}) {}, WorkerExecutor(e))
		w.Enqueue(1)
		w.Enqueue(2)
		w.Enqueue(3)

		r2 := e.Running()
		if r2 < 2 {
			tt.Errorf("worker + my task running: %d", r2)
		}

		w.ShutdownAndWait()

		r3 := e.Running()
		if r3 != 1 {
			tt.Errorf("running my task 1 != %d", r1)
		}

		done <- struct{}{}

		time.Sleep(1 * time.Millisecond)

		r4 := e.Running()
		if r4 != 0 {
			tt.Errorf("all task done %d", r1)
		}
	})
}

func TestWorkerAbortQueueHandler(t *testing.T) {
	t.Run("default", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		count := 0
		dequeue := func(p interface{}) {
			v := p.(int)
			tt.Logf("dequeue val[%d] = %d", count, v)
			if v != 123 {
				tt.Errorf("first value is 123")
			}
			count += 1
		}
		abortHandler := func(p interface{}) {
			v := p.(int)
			tt.Logf("abort val[%d] = %d", count, v)

			if count == 1 {
				if v != 456 {
					tt.Errorf("second value is 456")
				}
				count += 1
			}
			if count == 1 {
				if v != 789 {
					tt.Errorf("third value is 789")
				}
				count += 1
			}
		}

		w := NewDefaultWorker(dequeue, WorkerExecutor(e), WorkerAbortQueueHandler(abortHandler))

		w.Enqueue(123)
		w.Shutdown() // close enqueue

		w.Enqueue(456)
		w.Enqueue(789)
	})
	t.Run("buffer", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		count := 0
		dequeue := func(p interface{}) {
			v := p.(int)
			tt.Logf("dequeue val[%d] = %d", count, v)
			if v != 123 {
				tt.Errorf("first value is 123")
			}
			count += 1
		}
		abortHandler := func(p interface{}) {
			v := p.(int)
			tt.Logf("abort val[%d] = %d", count, v)

			if count == 1 {
				if v != 456 {
					tt.Errorf("second value is 456")
				}
				count += 1
			}
			if count == 1 {
				if v != 789 {
					tt.Errorf("third value is 789")
				}
				count += 1
			}
		}

		w := NewBufferWorker(dequeue, WorkerExecutor(e), WorkerAbortQueueHandler(abortHandler))

		w.Enqueue(123)
		w.ShutdownAndWait() // close enqueue

		w.Enqueue(456)
		w.Enqueue(789)
	})
}

func TestWorkerCapacity(t *testing.T) {
	t.Run("default/cap0", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		times := 5
		delay := 100 * time.Millisecond
		dequeue := func(p interface{}) {
			<-time.After(delay)
			tt.Logf("dequeue %d", p.(int))
		}
		w := NewDefaultWorker(dequeue, WorkerExecutor(e))

		s := time.Now()
		for i := 0; i <= times; i += 1 {
			k := time.Now()
			w.Enqueue(i)
			tt.Logf("%d: e = %s", i, time.Since(k))
		}

		elapse := time.Since(s)
		if (time.Duration(times-1) * delay) <= elapse {
			tt.Logf("capacity 0 is blocking >=400ms: %s", elapse)
		} else {
			tt.Errorf("must block queue: %s", elapse)
		}
	})

	t.Run("default/cap5", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		times := 5
		delay := 100 * time.Millisecond
		dequeue := func(p interface{}) {
			<-time.After(delay)
			tt.Logf("dequeue %d", p.(int))
		}
		w := NewDefaultWorker(dequeue, WorkerExecutor(e), WorkerCapacity(5))

		s := time.Now()
		for i := 0; i <= times; i += 1 {
			k := time.Now()
			w.Enqueue(i)
			tt.Logf("%d: e = %s", i, time.Since(k))
		}

		elapse := time.Since(s)
		if elapse < delay {
			tt.Logf("non blocking until 5 times %s", elapse)
		} else {
			tt.Errorf("must non blocking queue: %s", elapse)
		}

		s2 := time.Now()
		w.Enqueue(struct{}{})
		elapse2 := time.Since(s2)

		if delay <= elapse2 {
			tt.Logf("must blocking over 5 times: %s", elapse2)
		} else {
			tt.Errorf("must blocking greater than eq 5 cap: %s", elapse2)
		}
	})

	t.Run("buffer/cap0", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		times := 5
		delay := 100 * time.Millisecond
		dequeue := func(p interface{}) {
			<-time.After(delay)
			tt.Logf("dequeue %d", p.(int))
		}
		w := NewBufferWorker(dequeue, WorkerExecutor(e))

		s := time.Now()
		for i := 0; i <= times; i += 1 {
			k := time.Now()
			w.Enqueue(i)
			tt.Logf("%d: e = %s", i, time.Since(k))
		}

		elapse := time.Since(s)
		if elapse < delay {
			tt.Logf("buffer worker non blocking worker execution: %s", elapse)
		} else {
			tt.Errorf("buffer worker blocking(maybe small goroutine size: %s", elapse)
		}
	})

	t.Run("buffer/cap5", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		times := 5
		delay := 100 * time.Millisecond
		dequeue := func(p interface{}) {
			<-time.After(delay)
			tt.Logf("dequeue %d", p.(int))
		}
		w := NewBufferWorker(dequeue, WorkerExecutor(e), WorkerCapacity(5))

		s := time.Now()
		for i := 0; i <= times; i += 1 {
			k := time.Now()
			w.Enqueue(i)
			tt.Logf("%d: e = %s", i, time.Since(k))
		}

		elapse := time.Since(s)
		if elapse < delay {
			tt.Logf("non blocking buffer worker %s", elapse)
		} else {
			tt.Errorf("must non blocking queue: %s", elapse)
		}

		s2 := time.Now()
		w.Enqueue(struct{}{})
		elapse2 := time.Since(s2)

		if elapse2 < delay {
			tt.Logf("buffer worker non blocking worker execution: %s", elapse)
		} else {
			tt.Errorf("buffer worker blocking(maybe small goroutine size: %s", elapse)
		}
	})
}

func TestWorkerDequeue(t *testing.T) {
	t.Run("default/cap10/deqSize3", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		expect := []string{
			"pre", "0", "1", "2", "post",
		}
		values := make([]string, 0, 10)

		done := make(chan struct{})
		w := NewDefaultWorker(func(param interface{}) {
			if param.(string) == "stop" {
				close(done)
				return
			}
			values = append(values, param.(string))
		}, WorkerPreHook(func() {
			values = append(values, "pre")
		}), WorkerPostHook(func() {
			values = append(values, "post")
		}), WorkerCapacity(10), WorkerMaxDequeueSize(3))

		for i := 0; i < 13; i += 1 {
			w.Enqueue(fmt.Sprintf("%d", i))
		}
		w.Enqueue("stop")
		<-done

		tt.Logf("max < cap")
		for i, _ := range expect {
			if values[i] != expect[i] {
				tt.Errorf("v[%d] expect %s actual %s", i, expect[i], values[i])
			}
		}
	})
	t.Run("default/cap5/deqSize10", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		expect := []string{
			"pre", "0", "1", "2", "3", "4", "post",
		}
		values := make([]string, 0, 10)

		done := make(chan struct{})
		w := NewDefaultWorker(func(param interface{}) {
			if param.(string) == "stop" {
				close(done)
				return
			}
			values = append(values, param.(string))
		}, WorkerPreHook(func() {
			values = append(values, "pre")
		}), WorkerPostHook(func() {
			values = append(values, "post")
		}), WorkerCapacity(5), WorkerMaxDequeueSize(10))

		for i := 0; i < 10; i += 1 {
			w.Enqueue(fmt.Sprintf("%d", i))
		}
		w.Enqueue("stop")
		<-done

		tt.Logf("max = cap")
		for i, _ := range expect {
			if values[i] != expect[i] {
				tt.Errorf("v[%d] expect %s actual %s", i, expect[i], values[i])
			}
		}
	})
	t.Run("buffer/cap10/deqSize3", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		expect := []string{
			"pre", "0", "1", "2", "post",
		}
		values := make([]string, 0, 10)

		done := make(chan struct{})
		w := NewBufferWorker(func(param interface{}) {
			if param.(string) == "stop" {
				close(done)
				return
			}
			values = append(values, param.(string))
		}, WorkerPreHook(func() {
			values = append(values, "pre")
		}), WorkerPostHook(func() {
			values = append(values, "post")
		}), WorkerCapacity(10), WorkerMaxDequeueSize(3))

		for i := 0; i < 13; i += 1 {
			w.Enqueue(fmt.Sprintf("%d", i))
		}
		w.Enqueue("stop")
		<-done

		tt.Logf("max < cap")
		tt.Logf("%v", values)
		for i, _ := range expect {
			if values[i] != expect[i] {
				tt.Errorf("v[%d] expect %s actual %s", i, expect[i], values[i])
			}
		}
	})
}

func TestWorkerFinalizer(t *testing.T) {
	t.Run("default", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		if e.Running() != 0 {
			tt.Errorf("initial")
		}

		w := NewDefaultWorker(func(param interface{}) {
			tt.Logf("param = %+v", param)
		}, WorkerExecutor(e))

		ch := make(chan struct{})
		runtime.SetFinalizer(w, nil)
		runtime.SetFinalizer(w, func(me Worker) {
			ch <- struct{}{}
		})

		w.Enqueue("test") // wait start worker

		if e.Running() != 1 {
			tt.Errorf("default worker running: %d", e.Running())
		}

		tt.Logf("call finalizer")
		w = nil
		runtime.GC()

		select {
		case <-ch:
			tt.Logf("called worker finalize")
		case <-time.After(1 * time.Second):
			tt.Errorf("finalizer timeout")
		}

		if e.Running() != 0 {
			tt.Logf("[ok] worker still running %s", debug.Stack())
		}
	})
	t.Run("buffer", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		if e.Running() != 0 {
			tt.Errorf("initial")
		}

		w := NewBufferWorker(func(param interface{}) {
			tt.Logf("param = %+v", param)
		}, WorkerExecutor(e))

		ch := make(chan struct{})
		runtime.SetFinalizer(w, nil)
		runtime.SetFinalizer(w, func(me Worker) {
			ch <- struct{}{}
		})

		w.Enqueue("test") // wait start worker

		if e.Running() < 1 {
			tt.Errorf("buffer worker running: %d", e.Running())
		}

		tt.Logf("call finalizer")
		w = nil
		runtime.GC()

		select {
		case <-ch:
			tt.Logf("called worker finalize")
		case <-time.After(1 * time.Second):
			tt.Errorf("finalizer timeout")
		}

		if e.Running() != 0 {
			tt.Logf("[ok] worker still running %s", debug.Stack())
		}
	})
	t.Run("default/autoshutdown", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		if e.Running() != 0 {
			tt.Errorf("initial")
		}

		w := NewDefaultWorker(func(param interface{}) {
			tt.Logf("param = %+v", param)
		}, WorkerExecutor(e), WorkerAutoShutdown(true))

		w.Enqueue("test") // wait start worker

		if e.Running() != 1 {
			tt.Errorf("default worker running: %d", e.Running())
		}

		tt.Logf("call finalizer")
		w = nil
		runtime.GC()

		time.Sleep(time.Second)

		if e.Running() != 0 {
			tt.Errorf("worker still running %s", debug.Stack())
		}
	})
	t.Run("buffer/autoshutdown", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		if e.Running() != 0 {
			tt.Errorf("initial")
		}

		w := NewBufferWorker(func(param interface{}) {
			tt.Logf("param = %+v", param)
		}, WorkerExecutor(e), WorkerAutoShutdown(true))

		w.Enqueue("test") // wait start worker

		if e.Running() < 1 {
			tt.Errorf("buffer worker running: %d", e.Running())
		}

		tt.Logf("call finalizer")
		w = nil
		runtime.GC()

		time.Sleep(time.Second)

		if e.Running() != 0 {
			tt.Errorf("worker still running %s", debug.Stack())
		}
	})
}

func TestWorkerTimeout(t *testing.T) {
	t.Run("default", func(tt *testing.T) {
	})
	t.Run("buffer", func(tt *testing.T) {
	})
}
