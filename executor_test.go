package chanque

import(
  "testing"
  "time"
)

func TestExecutorDefault(t *testing.T) {
  chkDefault := func(e *Executor) {
    if e.minWorker != 0 {
      t.Errorf("default min 0 actual:%d", e.minWorker)
    }
    if e.maxWorker != 1 {
      t.Errorf("default max 1 actual:%d", e.maxWorker)
    }
    if e.maxCapacity != 0 {
      t.Errorf("default capacity 0 actual:%d", e.maxCapacity)
    }
    if e.reducerInterval != defaultReducerInterval {
      t.Errorf("default 10s actual:%s", e.reducerInterval)
    }
    if e.panicHandler == nil {
      t.Errorf("default panic handler:%v", e.panicHandler)
    }
  }
  chkDefault(CreateExecutor(0, 0))
  chkDefault(CreateExecutor(-1, -1))
  chkDefault(CreateExecutor(-1, -2))

  e1 := CreateExecutor(100, 50)
  if e1.minWorker != 100 {
    t.Errorf("init 100: %d", e1.minWorker)
  }
  if e1.maxWorker != 100 {
    t.Errorf("init max < min then max eq min: %d", e1.maxWorker)
  }
  if e1.maxCapacity != 0 {
    t.Errorf("init maxCapacity default 0: %d", e1.maxCapacity)
  }

  e2 := CreateExecutor(50, 150)
  if e2.minWorker != 50 {
    t.Errorf("init 50: %d", e2.minWorker)
  }
  if e2.maxWorker != 150 {
    t.Errorf("init specified max: %d", e2.maxWorker)
  }
  if e2.maxCapacity != 0 {
    t.Errorf("init maxCapacity default 0: %d", e2.maxCapacity)
  }
}

func TestExecutorOption(t *testing.T) {
  e1 := CreateExecutor(1, 5,
    ExecutorMaxCapacity(2),
  )
  if e1.minWorker != 1 {
    t.Errorf("init 1: %d", e1.minWorker)
  }
  if e1.maxWorker != 5 {
    t.Errorf("init specified max: %d", e1.maxWorker)
  }
  if e1.maxCapacity != 2 {
    t.Errorf("init specified capacity: %d", e1.maxCapacity)
  }

  e2 := CreateExecutor(1, 5,
    ExecutorMaxCapacity(0),
  )
  if e2.minWorker != 1 {
    t.Errorf("init 1: %d", e2.minWorker)
  }
  if e2.maxWorker != 5 {
    t.Errorf("init specified max: %d", e2.maxWorker)
  }
  if e2.maxCapacity != 0 {
    t.Errorf("init specified capacity: %d", e2.maxCapacity)
  }

  e3 := CreateExecutor(1, 5,
    ExecutorMaxCapacity(10),
    ExecutorReducderInterval(30 * time.Millisecond),
  )
  if e3.maxCapacity != 10 {
    t.Errorf("init specified capacity: %d", e3.maxCapacity)
  }
  if e3.reducerInterval != (time.Duration(30) * time.Millisecond) {
    t.Errorf("init specified interval: %s", e3.reducerInterval)
  }

  e4 := CreateExecutor(1, 5,
    ExecutorReducderInterval(0),
  )
  if e4.reducerInterval != defaultReducerInterval {
    t.Errorf("init specified interval: %s", e4.reducerInterval)
  }

  val := "default"
  ph := func(pt PanicType, rcv interface{}) {
    val = "hello world"
  }
  e5 := CreateExecutor(0, 0,
    ExecutorPanicHandler(ph),
  )
  e5.panicHandler(PanicType(0), nil)
  if val != "hello world" {
    t.Errorf("init specified panic handler: %v", e5.panicHandler)
  }
}

func TestExecutorRunningAndWorker(t *testing.T) {
  e1 := CreateExecutor(10, 10)
  e2 := CreateExecutor(0, 10)
  e3 := CreateExecutor(-1, 15)
  e4 := CreateExecutor(10, 15, ExecutorMaxCapacity(0))
  e5 := CreateExecutor(10, 15, ExecutorMaxCapacity(5))

  time.Sleep(10 * time.Millisecond)

  if e1.Running() != 0 {
    t.Errorf("running should be 0")
  }
  if e1.Workers() != 10 {
    t.Errorf("worker startup 10")
  }

  if e2.Running() != 0 {
    t.Errorf("running should be 0")
  }
  if e2.Workers() != 0 {
    t.Errorf("worker startup 0")
  }

  if e3.Running() != 0 {
    t.Errorf("running should be 0")
  }
  if e3.Workers() != 0 {
    t.Errorf("worker startup 0")
  }

  if e4.Running() != 0 {
    t.Errorf("running should be 0")
  }
  if e4.Workers() != 10 {
    t.Errorf("worker startup 10")
  }

  if e5.Running() != 0 {
    t.Errorf("running should be 0")
  }
  if e5.Workers() != 10 {
    t.Errorf("worker startup 10")
  }
}

func TestExecutorOndemandStart(t *testing.T) {
  e := CreateExecutor(0, 1,
    ExecutorPanicHandler(func(pt PanicType, rcv interface{}){
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
    t.Errorf("minimun zero worker")
  }

  latch := make(chan struct{})
  e.Submit(func(){
    latch <-struct{}{}
    println("fist worker running")
    <-time.After(50 * time.Millisecond)
    println("fist worker done")
  })

  <-latch

  r2 := e.Running()
  w2 := e.Workers()
  if r2 != 1 {
    t.Errorf("running worker 1 actual:%d", r2)
  }
  if w2 != 1 {
    t.Errorf("generated worker 1 actual:%d", w2)
  }

  enqueued := make(chan struct{})
  go func(q chan struct{}){
    e.Submit(func() {
      enqueued <-struct{}{}
    })
  }(enqueued)

  select {
  case <-enqueued:
    t.Errorf("maxWorker is 1 = should be not goroutine generated")
  default:
    println("no submit reader ok")
    r3 := e.Running()
    w3 := e.Workers()
    if r3 != 1 {
      t.Errorf("still running first worker actual:%d", r3)
    }
    if w3 != 1 {
      t.Errorf("not yet worker generated actual:%d", w3)
    }
  }

  time.Sleep(100 * time.Millisecond)

  select {
  case <-enqueued:
    println("submit reader ok")
    r4 := e.Running()
    w4 := e.Workers()
    if r4 != 1 {
      t.Errorf("still running first worker actual:%d", r4)
    }
    if w4 != 1 {
      t.Errorf("not yet worker generated actual:%d", w4)
    }
  default:
    t.Errorf("should be second worker submitted")
  }
}
func TestExecutorOndemandStartUpto100(t *testing.T) {
  e := CreateExecutor(0, 100,
    ExecutorPanicHandler(func(pt PanicType, rcv interface{}){
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
    t.Errorf("minimun zero worker")
  }

  enqueued := make(chan struct{})

  for i := 0; i < 50; i += 1 {
    e.Submit(func (ch chan struct{}) func() {
      return func(){
        ch <-struct{}{}
      }
    }(enqueued))
  }
  time.Sleep(10 * time.Millisecond)

  r2 := e.Running()
  w2 := e.Workers()
  if r2 != 50 {
    t.Errorf("reader waiting worker1: %d", r2)
  }
  if w2 != 50 {
    t.Errorf("max worker running1: %d", w2)
  }

  for i := 50; i < 100; i += 1 {
    e.Submit(func (ch chan struct{}) func() {
      return func(){
        ch <-struct{}{}
      }
    }(enqueued))
  }
  time.Sleep(10 * time.Millisecond)

  r3 := e.Running()
  w3 := e.Workers()
  if r3 != 100 {
    t.Errorf("reader waiting worker2: %d", r3)
  }
  if w3 != 100 {
    t.Errorf("max worker running2: %d", w3)
  }

  r4 := e.Running()
  w4 := e.Workers()
  if r4 != 100 {
    t.Errorf("still max cap = reader waiting worker3: %d", r4)
  }
  if w4 != 100 {
    t.Errorf("still max worker = max worker running3: %d", w4)
  }

  for i := 0; i < 50; i += 1 {
    <-enqueued
  }
  time.Sleep(10 * time.Millisecond)

  r5 := e.Running()
  w5 := e.Workers()
  if r5 != 50 {
    t.Errorf("reader waiting worker4: %d", r5)
  }
  if w5 != 100 {
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
  if w6 != 100 {
    t.Errorf("max worker running6: %d", w6)
  }
}

func TestExecutorSubmitBlocking(t *testing.T) {
  e1 := CreateExecutor(0, 100,
    ExecutorPanicHandler(func(pt PanicType, rcv interface{}){
      /* nop */
    }),
  )
  defer e1.Release()

  enqueue := make(chan struct{})
  for i := 0; i < 100; i += 1 {
    e1.Submit(func (ch chan struct{}) func(){
      return func(){
        ch <-struct{}{}
        println("hoge")
      }
    }(enqueue))
  }
  time.Sleep(10 * time.Millisecond)

  r1 := e1.Running()
  w1 := e1.Workers()
  if r1 != 100 {
    t.Errorf("blocking waiting reader running: %d", r1)
  }
  if w1 != 100 {
    t.Errorf("blocking waiting reader workers: %d", w1)
  }

  latch := make(chan struct{})
  done  := make(chan struct{})
  go func(la chan struct{}, d chan struct{}, ch chan struct{}){
    <-la
    e1.Submit(func(){
      ch <-struct{}{}
    })
    d <-struct{}{}
  }(latch, done, enqueue)

  latch <-struct{}{}

  select {
  case <-time.After(10 * time.Millisecond):
    println("max capacity exceeded. submit blocking ok1")
  case <-done:
    t.Errorf("submit should be blocked")
  }

  r2 := e1.Running()
  w2 := e1.Workers()
  if r2 != 100 {
    t.Errorf("blocking waiting reader running: %d", r2)
  }
  if w2 != 100 {
    t.Errorf("blocking waiting reader workers: %d", w2)
  }
}

func TestExecutorSubmitNonBlocking(t *testing.T) {
  e1 := CreateExecutor(0, 100,
    ExecutorMaxCapacity(50),
    ExecutorPanicHandler(func(pt PanicType, rcv interface{}){
      /* nop */
    }),
  )
  defer e1.Release()

  enqueue := make(chan struct{})
  for i := 0; i < 100; i += 1 {
    e1.Submit(func (ch chan struct{}) func(){
      return func(){
        ch <-struct{}{}
      }
    }(enqueue))
  }
  time.Sleep(10 * time.Millisecond)

  r1 := e1.Running()
  w1 := e1.Workers()
  if r1 != 100 {
    t.Errorf("blocking waiting reader running: %d", r1)
  }
  if w1 != 100 {
    t.Errorf("blocking waiting reader workers: %d", r1)
  }

  latch := make(chan struct{})
  done  := make(chan struct{})
  go func(la chan struct{}, d chan struct{}, ch chan struct{}){
    <-la
    e1.Submit(func(){
      ch <-struct{}{}
    })
    d <-struct{}{}
  }(latch, done, enqueue)

  latch <-struct{}{}

  select {
  case <-time.After(10 * time.Millisecond):
    t.Errorf("free capacity")
  case <-done:
    println("free capacity. submit non-blocking ok")
  }

  for i := 0; i < 49; i += 1 {
    e1.Submit(func (ch chan struct{}) func(){
      return func(){
        ch <-struct{}{}
      }
    }(enqueue))
  }
  time.Sleep(10 * time.Millisecond)

  r2 := e1.Running()
  w2 := e1.Workers()
  if r2 != 100 {
    t.Errorf("blocking waiting reader running: %d", r2)
  }
  if w2 != 100 {
    t.Errorf("blocking waiting reader workers: %d", r2)
  }

  latch2 := make(chan struct{})
  done2  := make(chan struct{})
  go func(la chan struct{}, d chan struct{}, ch chan struct{}){
    <-la
    e1.Submit(func(){
      ch <-struct{}{}
    })
    d <-struct{}{}
  }(latch2, done2, enqueue)

  latch2 <-struct{}{}

  select {
  case <-time.After(10 * time.Millisecond):
    println("max capacity exceeded. submit blocking ok2")
  case <-done2:
    t.Errorf("submit should be blocked")
  }
}
