package chanque

import (
	"math/rand"
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

			time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
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
