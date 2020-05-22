package chanque

import(
  "time"
  "sync"
  "sync/atomic"
)

type Job func()

type ExecutorOption func(*Executor)

var(
  defaultReducerInterval = 10 * time.Second
)

func ExecutorPanicHandler(handler PanicHandler) ExecutorOption {
  return func(e *Executor) {
    e.panicHandler = handler
  }
}
func ExecutorReducderInterval(interval time.Duration) ExecutorOption {
  return func(e *Executor) {
    e.reducerInterval = interval
  }
}
func ExecutorMaxCapacity(capacity int) ExecutorOption {
  return func(e *Executor) {
    e.maxCapacity = capacity
  }
}

type Executor struct {
  mutex           *sync.Mutex
  wg              *sync.WaitGroup
  jobs            *Queue
  done            *Queue
  minWorker       int
  maxWorker       int
  maxCapacity     int
  panicHandler    PanicHandler
  reducerInterval time.Duration
  runningNum      int32
  workerNum       int32
}

func CreateExecutor(minWorker, maxWorker int, opts ...ExecutorOption) *Executor {
  e := new(Executor)
  for _, opt := range opts {
    opt(e)
  }

  if minWorker < 1 {
    minWorker = 0
  }
  if maxWorker < 1 {
    maxWorker = 1
  }
  if maxWorker < minWorker {
    maxWorker = minWorker
  }
  e.minWorker = minWorker
  e.maxWorker = maxWorker

  if e.maxCapacity < 1 {
    e.maxCapacity = 0
  }
  if e.panicHandler == nil {
    e.panicHandler = defaultPanicHandler
  }
  if e.reducerInterval < 1 {
    e.reducerInterval = defaultReducerInterval
  }

  e.mutex        = new(sync.Mutex)
  e.wg           = new(sync.WaitGroup)
  e.jobs         = NewQueue(e.maxCapacity)
  e.done         = NewQueue(0)
  e.runningNum   = int32(0)
  e.workerNum    = int32(0)
  e.initWorker()
  return e
}
func (e *Executor) initWorker() {
  for i := 0; i < e.minWorker; i += 1 {
    e.increWorker()
    e.wg.Add(1)
    go e.execloop(i, e.jobs)
  }
  e.wg.Add(1)
  go e.healthloop(e.done, e.jobs)
}
func (e *Executor) PanicHandler(handler PanicHandler) {
  e.mutex.Lock()
  defer e.mutex.Unlock()

  e.panicHandler = handler
  e.jobs.PanicHandler(handler)
  e.done.PanicHandler(handler)
}
func (e *Executor) callPanicHandler(pt PanicType, rcv interface{}) {
  e.mutex.Lock()
  defer e.mutex.Unlock()

  e.panicHandler(pt, rcv)
}
func (e *Executor) increRunning() {
  atomic.AddInt32(&e.runningNum, 1)
}
func (e *Executor) decreRunning() {
  atomic.AddInt32(&e.runningNum, -1)
}
// return num of running workers
func (e *Executor) Running() int32 {
  return atomic.LoadInt32(&e.runningNum)
}
func (e *Executor) increWorker() int32 {
  return atomic.AddInt32(&e.workerNum, 1)
}
func (e *Executor) decreWorker() int32 {
  return atomic.AddInt32(&e.workerNum, -1)
}
// return num of goroutines
func (e *Executor) Workers() int32 {
  return atomic.LoadInt32(&e.workerNum)
}
func (e *Executor) startOndemand() {
  next := int(e.increWorker())
  if e.minWorker < next {
    if next <= e.maxWorker {
      e.wg.Add(1)
      go e.execloop(next, e.jobs)
      return
    }
  }
  e.decreWorker()
}
// enqueue job
func (e *Executor) Submit(fn Job) {
  defer func(){
    if rcv := recover(); rcv != nil {
      e.callPanicHandler(PanicTypeEnqueue, rcv)
    }
  }()

  if fn == nil {
    return
  }

  e.startOndemand()
  e.jobs.Enqueue(fn)
}
// release goroutines
func (e *Executor) Release() {
  defer func(){
    if rcv := recover(); rcv != nil {
      e.callPanicHandler(PanicTypeClose, rcv)
    }
  }()

  e.done.Close()
  e.jobs.Close()
}
// release goroutines and wait goroutine done
func (e *Executor) ReleaseAndWait() {
  e.Release()
  e.wg.Wait()
}
func (e *Executor) healthloop(done *Queue, jobs *Queue) {
  defer e.wg.Done()
  defer func(){
    if rcv := recover(); rcv != nil {
      e.callPanicHandler(PanicTypeEnqueue, rcv)
    }
  }()

  ticker := time.NewTicker(e.reducerInterval)
  defer ticker.Stop()

  for {
    select {
    case <-done.Chan():
      return

    case <-ticker.C:
      currentWorkerNum := e.Workers()
      runningWorkerNum := e.Running()
      idleWorkers      := int(currentWorkerNum - runningWorkerNum)
      if e.minWorker < idleWorkers {
        reduceSize := int(idleWorkers - e.minWorker)
        for i := 0; i < reduceSize; i += 1 {
          jobs.EnqueueNB(nil)
        }
      }
    }
  }
}
func (e *Executor) execloop(id int, jobs *Queue) {
  defer e.wg.Done()
  defer func(){
    if rcv := recover(); rcv != nil {
      e.callPanicHandler(PanicTypeDequeue, rcv)
    }
  }()
  defer e.decreWorker()

  for {
    select {
    case job, ok := <-jobs.Chan():
      if ok != true {
        return
      }
      if job == nil {
        return
      }

      e.increRunning()
      fn := job.(Job)
      fn()
      e.decreRunning()
    }
  }
}
