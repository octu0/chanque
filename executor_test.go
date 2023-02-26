package chanque

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

func BenchmarkExecutor(b *testing.B) {
	run := func(name string, fn func(*testing.B)) {
		m1 := new(runtime.MemStats)
		runtime.ReadMemStats(m1)

		b.Run(name, fn)

		m2 := new(runtime.MemStats)
		runtime.ReadMemStats(m2)
		b.Logf(
			"%-20s\tTotalAlloc=%5d\tStackInUse=%5d",
			name,
			int64(m2.TotalAlloc)-int64(m1.TotalAlloc),
			int64(m2.StackInuse)-int64(m1.StackInuse),
			//int64(m2.HeapSys)  - int64(m1.HeapSys),
			//int64(m2.HeapIdle)   - int64(m1.HeapIdle),
		)
	}
	goroutine := func(size int) {
		wg := new(sync.WaitGroup)
		wg.Add(size)
		for i := 0; i < size; i += 1 {
			go func(w *sync.WaitGroup) {
				defer w.Done()
				time.Sleep(1 * time.Millisecond)
			}(wg)
		}
		wg.Wait()
	}
	executor := func(size int, e *Executor) {
		wg := new(sync.WaitGroup)
		wg.Add(size)
		for i := 0; i < size; i += 1 {
			e.Submit(func(w *sync.WaitGroup) Job {
				return func() {
					defer w.Done()
					time.Sleep(1 * time.Millisecond)
				}
			}(wg))
		}
		wg.Wait()
	}
	run("goroutine", func(tb *testing.B) {
		goroutine(tb.N)
	})
	run("executor/100-1000", func(tb *testing.B) {
		tb.StopTimer()
		e := NewExecutor(100, 1000)
		tb.StartTimer()

		executor(tb.N, e)

		tb.StopTimer()
		e.Release()
		tb.StartTimer()
	})
	run("executor/1000-5000", func(tb *testing.B) {
		tb.StopTimer()
		e := NewExecutor(1000, 5000)
		tb.StartTimer()

		executor(tb.N, e)

		tb.StopTimer()
		e.Release()
		tb.StartTimer()
	})
}

func TestExecutorDefault(t *testing.T) {
	chkDefault := func(e *Executor) {
		defer e.Release()

		if e.MinWorker() != 0 {
			t.Errorf("default min 0 actual:%d", e.minWorker)
		}
		if e.MaxWorker() != 1 {
			t.Errorf("default max 1 actual:%d", e.maxWorker)
		}
		if e.jobs.Cap() != 0 {
			t.Errorf("default capacity 0 actual:%d", e.jobs.Cap())
		}
		if e.reducerInterval != defaultReducerInterval {
			t.Errorf("default 10s actual:%s", e.reducerInterval)
		}
		if e.panicHandler == nil {
			t.Errorf("default panic handler:%v", e.panicHandler)
		}
	}
	chkDefault(NewExecutor(0, 0))
	chkDefault(NewExecutor(-1, -1))
	chkDefault(NewExecutor(-1, -2))

	t.Run("100-50", func(tt *testing.T) {
		e := NewExecutor(100, 50)
		defer e.Release()

		if e.MinWorker() != 100 {
			tt.Errorf("init 100: %d", e.minWorker)
		}
		if e.MaxWorker() != 100 {
			tt.Errorf("init max < min then max eq min: %d", e.maxWorker)
		}
		if e.jobs.Cap() != 0 {
			tt.Errorf("init maxCapacity default 0: %d", e.jobs.Cap())
		}
	})

	t.Run("50-150", func(tt *testing.T) {
		e := NewExecutor(50, 150)
		defer e.Release()

		if e.MinWorker() != 50 {
			tt.Errorf("init 50: %d", e.minWorker)
		}
		if e.MaxWorker() != 150 {
			tt.Errorf("init specified max: %d", e.maxWorker)
		}
		if e.jobs.Cap() != 0 {
			tt.Errorf("init maxCapacity default 0: %d", e.jobs.Cap())
		}
	})
}

func TestExecutorOption(t *testing.T) {
	t.Run("cap2", func(tt *testing.T) {
		e := NewExecutor(1, 5,
			ExecutorMaxCapacity(2),
		)
		defer e.Release()

		if e.MinWorker() != 1 {
			tt.Errorf("init 1: %d", e.minWorker)
		}
		if e.MaxWorker() != 5 {
			tt.Errorf("init specified max: %d", e.maxWorker)
		}
		if e.jobs.Cap() != 2 {
			tt.Errorf("init specified capacity: %d", e.jobs.Cap())
		}
	})

	t.Run("cap0", func(tt *testing.T) {
		e := NewExecutor(1, 5,
			ExecutorMaxCapacity(0),
		)
		defer e.Release()

		if e.MinWorker() != 1 {
			tt.Errorf("init 1: %d", e.minWorker)
		}
		if e.MaxWorker() != 5 {
			tt.Errorf("init specified max: %d", e.maxWorker)
		}
		if e.jobs.Cap() != 0 {
			tt.Errorf("init specified capacity: %d", e.jobs.Cap())
		}
	})

	t.Run("cap10/intval30", func(tt *testing.T) {
		e := NewExecutor(1, 5,
			ExecutorMaxCapacity(10),
			ExecutorReducderInterval(30*time.Millisecond),
		)
		defer e.Release()

		if e.jobs.Cap() != 10 {
			tt.Errorf("init specified capacity: %d", e.jobs.Cap())
		}
		if e.reducerInterval != (time.Duration(30) * time.Millisecond) {
			tt.Errorf("init specified interval: %s", e.reducerInterval)
		}
	})

	t.Run("intval0", func(tt *testing.T) {
		e := NewExecutor(1, 5,
			ExecutorReducderInterval(0),
		)
		defer e.Release()

		if e.reducerInterval != defaultReducerInterval {
			tt.Errorf("init specified interval: %s", e.reducerInterval)
		}
	})

	t.Run("panichandler", func(tt *testing.T) {
		val := "default"
		ph := func(pt PanicType, rcv interface{}) {
			val = "hello world"
		}

		e := NewExecutor(0, 0,
			ExecutorPanicHandler(ph),
		)
		defer e.Release()

		e.panicHandler(PanicType(0), nil)
		if val != "hello world" {
			t.Errorf("init specified panic handler: %v", e.panicHandler)
		}
	})
}

func TestExecutorRunningAndWorker(t *testing.T) {
	t.Run("10-10", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()
		time.Sleep(10 * time.Millisecond)

		if e.Running() != 0 {
			tt.Errorf("running should be 0")
		}
		if e.Workers() != 10 {
			tt.Errorf("worker startup 10")
		}
	})
	t.Run("0-10", func(tt *testing.T) {
		e := NewExecutor(0, 10)
		defer e.Release()
		time.Sleep(10 * time.Millisecond)

		if e.Running() != 0 {
			tt.Errorf("running should be 0")
		}
		if e.Workers() != 0 {
			tt.Errorf("worker startup 0")
		}
	})
	t.Run("-1-15", func(tt *testing.T) {
		e := NewExecutor(-1, 15)
		defer e.Release()
		time.Sleep(10 * time.Millisecond)

		if e.Running() != 0 {
			tt.Errorf("running should be 0")
		}
		if e.Workers() != 0 {
			tt.Errorf("worker startup 0")
		}
	})
	t.Run("10-15-0", func(tt *testing.T) {
		e := NewExecutor(10, 15, ExecutorMaxCapacity(0))
		defer e.Release()
		time.Sleep(10 * time.Millisecond)

		if e.Running() != 0 {
			tt.Errorf("running should be 0")
		}
		if e.Workers() != 10 {
			tt.Errorf("worker startup 10")
		}
	})
	t.Run("10-15-5", func(tt *testing.T) {
		e := NewExecutor(10, 15, ExecutorMaxCapacity(5))
		defer e.Release()
		time.Sleep(10 * time.Millisecond)

		if e.Running() != 0 {
			tt.Errorf("running should be 0")
		}
		if e.Workers() != 10 {
			tt.Errorf("worker startup 10")
		}
	})
}

func TestExecutorOndemandStart(t *testing.T) {
	e := NewExecutor(0, 1,
		ExecutorPanicHandler(func(pt PanicType, rcv interface{}) {
			/* nop */
		}),
	)
	defer e.Release()

	r1 := e.Running()
	w1 := e.Workers()
	if r1 != 0 {
		t.Errorf("no task submmit")
	}
	if w1 != 0 {
		t.Errorf("minimum zero worker")
	}

	latchFirst := make(chan struct{})
	lockFirst := make(chan struct{})
	e.Submit(func() {
		latchFirst <- struct{}{}
		t.Logf("first worker running")
		<-lockFirst
		t.Logf("first worker done")
	})

	<-latchFirst

	r2 := e.Running()
	w2 := e.Workers()
	if r2 != 1 {
		t.Errorf("running worker 1 actual:%d", r2)
	}
	if w2 != 2 {
		t.Errorf("generated worker 2(run+1) actual:%d", w2)
	}

	latchSecond := make(chan struct{})
	lockSecond := make(chan struct{})
	e.Submit(func() {
		latchSecond <- struct{}{}
		t.Logf("second worker running")
		<-lockSecond
		t.Logf("second worker done")
	})

	<-latchSecond

	r3 := e.Running()
	w3 := e.Workers()
	if r3 != 2 {
		t.Errorf("running worker 2 actual:%d", r3)
	}
	if w3 != 3 {
		t.Errorf("generated worker 2(run+1) actual:%d", w3)
	}

	close(lockFirst)
	time.Sleep(50 * time.Millisecond)

	r4 := e.Running()
	w4 := e.Workers()
	if r4 != 1 {
		t.Errorf("running worker 1 actual:%d", r4)
	}
	if w4 != 3 {
		t.Errorf("generated worker 3(not reducer run) actual:%d", w4)
	}
	close(lockSecond)
}

func TestExecutorOndemandStartUpto100(t *testing.T) {
	e := NewExecutor(0, 100,
		ExecutorPanicHandler(func(pt PanicType, rcv interface{}) {
			/* nop */
		}),
	)
	defer e.Release()

	r1 := e.Running()
	w1 := e.Workers()
	if r1 != 0 {
		t.Errorf("no task submmit")
	}
	if w1 != 0 {
		t.Errorf("minimum zero worker")
	}

	enqueued := make(chan struct{})
	for i := 0; i < 50; i += 1 {
		e.Submit(func(ch chan struct{}) func() {
			return func() {
				ch <- struct{}{}
			}
		}(enqueued))
	}
	time.Sleep(10 * time.Millisecond)

	r2 := e.Running()
	w2 := e.Workers()
	if r2 < 50 {
		t.Errorf("reader waiting worker1: %d", r2)
	}
	if w2 < 50 {
		t.Errorf("max worker running1: %d", w2)
	}

	for i := 50; i < 100; i += 1 {
		e.Submit(func(ch chan struct{}) func() {
			return func() {
				ch <- struct{}{}
			}
		}(enqueued))
	}
	time.Sleep(10 * time.Millisecond)

	r3 := e.Running()
	w3 := e.Workers()
	if r3 < 100 {
		t.Errorf("reader waiting worker2: %d", r3)
	}
	if w3 < 100 {
		t.Errorf("max worker running2: %d", w3)
	}

	r4 := e.Running()
	w4 := e.Workers()
	if r4 < 100 {
		t.Errorf("still max cap = reader waiting worker3: %d", r4)
	}
	if w4 < 100 {
		t.Errorf("still max worker = max worker running3 around 100: %d", w4)
	}

	for i := 0; i < 50; i += 1 {
		<-enqueued
	}
	time.Sleep(10 * time.Millisecond)

	r5 := e.Running()
	w5 := e.Workers()
	if r5 < 50 {
		t.Errorf("reader waiting worker4: %d", r5)
	}
	if w5 < 100 {
		t.Errorf("max worker running4: %d", w5)
	}

	for i := 50; i < 100; i += 1 {
		<-enqueued
	}

	time.Sleep(10 * time.Millisecond)

	r6 := e.Running()
	w6 := e.Workers()
	if r6 != 0 {
		t.Errorf("worker dummy still running: %d", r6)
	}
	if w6 < 100 {
		t.Errorf("max worker running6: %d", w6)
	}
}

func TestExecutorSubmitBlocking(t *testing.T) {
	e1 := NewExecutor(0, 100,
		ExecutorPanicHandler(func(pt PanicType, rcv interface{}) {
			/* nop */
		}),
	)
	defer e1.Release()

	enqueue := make(chan struct{})
	for i := 0; i < 100; i += 1 {
		e1.Submit(func(ch chan struct{}) func() {
			return func() {
				ch <- struct{}{}
				t.Logf("hoge")
			}
		}(enqueue))
	}
	time.Sleep(10 * time.Millisecond)

	r1 := e1.Running()
	w1 := e1.Workers()
	if r1 < 100 {
		t.Errorf("blocking waiting reader running: %d", r1)
	}
	if w1 < 100 {
		t.Errorf("blocking waiting reader workers: %d", w1)
	}

	latch := make(chan struct{})
	done := make(chan struct{})
	go func(la chan struct{}, d chan struct{}, ch chan struct{}) {
		<-la
		e1.Submit(func() {
			ch <- struct{}{}
		})
		d <- struct{}{}
	}(latch, done, enqueue)

	latch <- struct{}{}

	select {
	case <-time.After(10 * time.Millisecond):
		t.Logf("max capacity exceeded. submit blocking ok1")
	case <-done:
		t.Logf("submit should not be blocked")
	}

	r2 := e1.Running()
	w2 := e1.Workers()
	if r2 < 100 {
		t.Errorf("blocking waiting reader running: %d", r2)
	}
	if w2 < 100 {
		t.Errorf("blocking waiting reader workers: %d", w2)
	}
}

func TestExecutorSubmitNonBlocking(t *testing.T) {
	e := NewExecutor(0, 100,
		ExecutorMaxCapacity(50),
		ExecutorPanicHandler(func(pt PanicType, rcv interface{}) {
			/* nop */
		}),
	)
	defer e.Release()

	enqueue := make(chan struct{})
	for i := 0; i < 100; i += 1 {
		e.Submit(func(ch chan struct{}) func() {
			return func() {
				ch <- struct{}{}
			}
		}(enqueue))
	}
	time.Sleep(10 * time.Millisecond)

	r1 := e.Running()
	w1 := e.Workers()
	if r1 < 100 {
		t.Errorf("blocking waiting reader running: %d", r1)
	}
	if w1 < 100 {
		t.Errorf("blocking waiting reader workers: %d", r1)
	}

	latch := make(chan struct{})
	done := make(chan struct{})
	go func(la chan struct{}, d chan struct{}, ch chan struct{}) {
		<-la
		e.Submit(func() {
			ch <- struct{}{}
		})
		d <- struct{}{}
	}(latch, done, enqueue)

	latch <- struct{}{}

	select {
	case <-time.After(10 * time.Millisecond):
		t.Errorf("free capacity")
	case <-done:
		t.Log("free capacity. submit non-blocking ok")
	}

	for i := 0; i < 49; i += 1 {
		e.Submit(func(ch chan struct{}) func() {
			return func() {
				ch <- struct{}{}
			}
		}(enqueue))
	}
	time.Sleep(10 * time.Millisecond)

	r2 := e.Running()
	w2 := e.Workers()
	if r2 < 100 {
		t.Errorf("blocking waiting reader running: %d", r2)
	}
	if w2 < 100 {
		t.Errorf("blocking waiting reader workers: %d", r2)
	}

	latch2 := make(chan struct{})
	done2 := make(chan struct{})
	go func(la chan struct{}, d chan struct{}, ch chan struct{}) {
		<-la
		e.Submit(func() {
			ch <- struct{}{}
		})
		d <- struct{}{}
	}(latch2, done2, enqueue)

	latch2 <- struct{}{}

	select {
	case <-time.After(10 * time.Millisecond):
		t.Log("max capacity exceeded. submit blocking ok2")
	case <-done2:
		t.Log("submit should not be blocked")
	}
}

func TestExecutorWorkerShrink(t *testing.T) {
	t.Run("min0/max10/job10", func(tt *testing.T) {
		e := NewExecutor(0, 10,
			ExecutorReducderInterval(50*time.Millisecond),
			ExecutorPanicHandler(func(pt PanicType, rcv interface{}) {
				/* nop */
			}),
		)
		defer e.Release()

		r1 := e.Running()
		w1 := e.Workers()

		if r1 != 0 {
			tt.Errorf("initial run is zero %v", r1)
		}
		if w1 != 0 {
			tt.Errorf("initial worker is zero %v", w1)
		}
		for i := 0; i < 10; i += 1 {
			e.Submit(func() {
				time.Sleep(50 * time.Millisecond)
			})
		}
		time.Sleep(10 * time.Millisecond) // waiting submitted

		r2 := e.Running()
		w2 := e.Workers()
		if r2 != 10 {
			tt.Errorf("running worker 10 != %v", r2)
		}
		if w2 < 10 {
			tt.Errorf("generated workers around 10 != %v", w2)
		}

		time.Sleep(100 * time.Millisecond)

		r3 := e.Running()
		w3 := e.Workers()
		if r3 != 0 {
			tt.Errorf("done for all worker %v", r3)
		}
		if w3 != 0 {
			tt.Errorf("worker shrink to min size0 %v", w3)
		}
	})
	t.Run("min10/max30/job30", func(tt *testing.T) {
		e := NewExecutor(10, 30,
			ExecutorReducderInterval(50*time.Millisecond),
			ExecutorPanicHandler(func(pt PanicType, rcv interface{}) {
				/* nop */
			}),
		)
		defer e.Release()
		time.Sleep(10 * time.Millisecond) // todo worker startup WaitGroup

		r1 := e.Running()
		w1 := e.Workers()

		if r1 != 0 {
			tt.Errorf("initial run is zero %v", r1)
		}
		if w1 != 10 {
			tt.Errorf("initial worker is 10 %v", w1)
		}

		lock := make(chan struct{})
		for i := 0; i < 30; i += 1 {
			e.Submit(func() {
				<-lock
			})
		}
		time.Sleep(100 * time.Millisecond) // waiting submitted todo: SubmitAndWait

		r2 := e.Running()
		w2 := e.Workers()
		if r2 < 30 {
			tt.Errorf("running worker around 30 != %v", r2)
		}
		if w2 < 30 {
			tt.Errorf("generated workers around 30 != %v", w2)
		}

		close(lock) // release running

		time.Sleep(100 * time.Millisecond)

		r3 := e.Running()
		w3 := e.Workers()
		if r3 != 0 {
			tt.Errorf("done for all worker %v", r3)
		}
		if w3 != 10 {
			tt.Errorf("worker shrink to min size10 %v", w3)
		}
	})
	t.Run("min10/max30/job5", func(tt *testing.T) {
		e := NewExecutor(10, 30,
			ExecutorReducderInterval(50*time.Millisecond),
			ExecutorPanicHandler(func(pt PanicType, rcv interface{}) {
				/* nop */
			}),
		)
		defer e.Release()

		time.Sleep(10 * time.Millisecond) // wait init worker run

		r1 := e.Running()
		w1 := e.Workers()

		if r1 != 0 {
			tt.Errorf("initial run is zero %v", r1)
		}
		if w1 != 10 {
			tt.Errorf("initial worker is 10 actual=%d", w1)
		}
		for i := 0; i < 5; i += 1 {
			e.Submit(func() {
				time.Sleep(50 * time.Millisecond)
			})
		}
		time.Sleep(10 * time.Millisecond) // waiting submitted

		r2 := e.Running()
		w2 := e.Workers()
		if r2 != 5 {
			tt.Errorf("running worker 5 != %v", r2)
		}
		if w2 != 10 {
			tt.Errorf("generated workers 10 != %v", w2)
		}

		time.Sleep(100 * time.Millisecond)

		r3 := e.Running()
		w3 := e.Workers()
		if r3 != 0 {
			tt.Errorf("done for all worker %v", r3)
		}
		if w3 != 10 {
			tt.Errorf("worker shrink to min size10 %v", w3)
		}
	})
}

func TestSubExecutor(t *testing.T) {
	t.Run("parent max cap", func(tt *testing.T) {
		max := 10
		e := NewExecutor(1, max,
			ExecutorPanicHandler(func(pt PanicType, rcv interface{}) {
				/* nop */
			}),
		)
		defer e.Release()
		sub := e.SubExecutor()

		lock := make(chan struct{})
		for i := 0; i < max; i += 1 {
			e.Submit(func() {
				<-lock
			})
		}

		tt.Logf("[log]reached cap of parent")

		latch := make(chan struct{})
		done := make(chan struct{})
		sub.Submit(func() {
			latch <- struct{}{}
			tt.Logf("[log]sub worker hello")
			done <- struct{}{}
		})

		tt.Logf("[log]submitted sub")

		<-latch
		select {
		case <-time.After(10 * time.Millisecond):
			tt.Errorf("ondemand worker not run?")
		case <-done:
			// pass
		}

		tt.Logf("[log]release parent")
		close(lock)

		latch2 := make(chan struct{})
		done2 := make(chan struct{})
		sub.Submit(func() {
			latch2 <- struct{}{}
			tt.Logf("[log]sub worker world")
			done2 <- struct{}{}
		})

		tt.Logf("[log]submitted sub 2")

		<-latch2
		select {
		case <-time.After(10 * time.Millisecond):
			tt.Errorf("ondemand worker not run?")
		case <-done2:
			// pass
		}
	})
	t.Run("wait", func(tt *testing.T) {
		max := 10
		e := NewExecutor(1, max,
			ExecutorPanicHandler(func(pt PanicType, rcv interface{}) {
				/* nop */
			}),
		)
		defer e.Release()
		sub := e.SubExecutor()

		parent := make(chan struct{})
		e.Submit(func() {
			time.Sleep(50 * time.Millisecond)
			tt.Logf("parent worker done")
			parent <- struct{}{}
		})

		for i := 0; i < 5; i += 1 {
			sub.Submit(func() {
				time.Sleep(150 * time.Millisecond)
			})
		}

		select {
		case <-parent:
			tt.Errorf("parent worker still run")
		case <-time.After(1 * time.Millisecond):
			tt.Logf("non blocking ok")
		}

		sub.Wait()

		r1 := e.Running()
		if r1 != 1 {
			tt.Errorf("parent worker running: %v", r1)
		}

		select {
		case <-parent:
			tt.Logf("parent job done")
		case <-time.After(10 * time.Millisecond):
			tt.Errorf("parent job still run")
		}

		r2 := e.Running()
		if r2 != 0 {
			tt.Errorf("all jobs done: %v", r2)
		}
	})
}

func TestExecutorTune(t *testing.T) {
	minimum := func(tt *testing.T, min, max int) {
		e := NewExecutor(1, 1)
		defer e.Release()

		time.Sleep(10 * time.Millisecond) // wait init worker run

		if e.MinWorker() != 1 {
			tt.Errorf("minWorker initial 1 != %d", e.MinWorker())
		}
		if e.MaxWorker() != 1 {
			tt.Errorf("maxWorker initial 1 != %d", e.MaxWorker())
		}
		if e.Workers() != 1 {
			tt.Errorf("initial run 1 != %d", e.Workers())
		}

		e.TuneMinWorker(min)

		if e.MinWorker() != 1 {
			tt.Errorf("minWorker initial 1 != %d", e.MinWorker())
		}
		if e.MaxWorker() != 1 {
			tt.Errorf("maxWorker initial 1 != %d", e.MaxWorker())
		}
		if e.Workers() != 1 {
			tt.Errorf("initial run 1 != %d", e.Workers())
		}

		e.TuneMaxWorker(max)

		if e.MinWorker() != 1 {
			tt.Errorf("minWorker initial 1 != %d", e.MinWorker())
		}
		if e.MaxWorker() != 1 {
			tt.Errorf("maxWorker initial 1 != %d", e.MaxWorker())
		}
		if e.Workers() != 1 {
			tt.Errorf("initial run 1 != %d", e.Workers())
		}
	}
	t.Run("-1", func(tt *testing.T) {
		minimum(tt, -1, -1)
		minimum(tt, 0, 0)
		minimum(tt, 1, 0)
		minimum(tt, 0, 1)
	})
	t.Run("max_gt_min", func(tt *testing.T) {
		e := NewExecutor(1, 10)
		defer e.Release()

		time.Sleep(10 * time.Millisecond) // wait init worker run

		if e.Workers() != 1 {
			tt.Errorf("initial run 1 != %d", e.Workers())
		}

		e.TuneMinWorker(50)
		time.Sleep(10 * time.Millisecond) // wait start up worker

		if e.MinWorker() != 10 {
			tt.Errorf("round max size: %d", e.MinWorker())
		}
		if e.Workers() != 10 {
			tt.Errorf("up to 10: %d", e.Workers())
		}

		e.TuneMaxWorker(50)
		time.Sleep(10 * time.Millisecond) // wait start up worker

		if e.MinWorker() != 10 {
			tt.Errorf("still min size 10: %d", e.MinWorker())
		}
		if e.MaxWorker() != 50 {
			tt.Errorf("max worker 50 != %d", e.MaxWorker())
		}
		if e.Workers() != 10 {
			tt.Errorf("still worker 10: %d", e.Workers())
		}

		e.TuneMinWorker(50)
		time.Sleep(10 * time.Millisecond) // wait start up worker

		if e.MinWorker() != 50 {
			tt.Errorf("up to 50: %d", e.MinWorker())
		}
		if e.MaxWorker() != 50 {
			tt.Errorf("max worker 50 != %d", e.MaxWorker())
		}
		if e.Workers() != 50 {
			tt.Errorf("up to 50: %d", e.Workers())
		}
	})
	t.Run("with_ondemand", func(tt *testing.T) {
		e := NewExecutor(1, 10,
			ExecutorReducderInterval(50*time.Millisecond),
		)
		defer e.Release()

		waitStartup := new(sync.WaitGroup)
		doneJob := new(sync.WaitGroup)
		ch := make(chan struct{}, 0)
		for i := 0; i < 5; i += 1 {
			waitStartup.Add(1)
			doneJob.Add(1)
			e.Submit(func() {
				waitStartup.Done()
				<-ch
				doneJob.Done()
			})
		}
		waitStartup.Wait()

		if e.Workers() < 5 {
			tt.Errorf("ondemand up: %d > 5", e.Workers())
		}

		if e.MinWorker() != 1 {
			tt.Errorf("default min worker 1 != %d", e.MinWorker())
		}

		e.TuneMinWorker(2)
		time.Sleep(50 * time.Millisecond) // wait reducer

		if e.MinWorker() != 2 {
			tt.Errorf("default min worker 2 != %d", e.MinWorker())
		}
		if e.Workers() < 5 {
			tt.Errorf("already running min < curr: %d", e.Workers())
		}

		close(ch)
		doneJob.Wait()
		time.Sleep(100 * time.Millisecond) // wait reducer

		if e.MinWorker() != 2 {
			tt.Errorf("default min worker 2 != %d", e.MinWorker())
		}
		if e.Workers() != 2 {
			tt.Errorf("reduced size 2 != %d", e.Workers())
		}

		e.TuneMinWorker(3)
		time.Sleep(50 * time.Millisecond) // wait reducer

		if e.MinWorker() != 3 {
			tt.Errorf("default min worker 3 != %d", e.MinWorker())
		}
		if e.Workers() != 3 {
			tt.Errorf("up to 3 != %d", e.Workers())
		}
	})
}
