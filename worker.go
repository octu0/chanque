package chanque

import(
  "context"
  "sync"
  "sync/atomic"
)
type Worker interface {
  Enqueue(interface{}) bool
  CloseEnqueue()       bool
  Shutdown()
  ShutdownAndWait()
  ForceStop()
}

type WorkerHandler    func(parameter interface{})

type WorkerHook       func()

func noopWorkerHook() {
  /* noop */
}

type WorkerOptionFunc func(*WorkerOption)

type WorkerOption struct {
  ctx           context.Context
  panicHandler  PanicHandler
  preHook       WorkerHook
  postHook      WorkerHook
}

func WorkerContext(ctx context.Context) WorkerOptionFunc {
  return func(opt *WorkerOption) {
    opt.ctx = ctx
  }
}

func WorkerPanicHandler(handler PanicHandler) WorkerOptionFunc {
  return func(opt *WorkerOption) {
    opt.panicHandler = handler
  }
}

func WorkerPreHook(hook WorkerHook) WorkerOptionFunc {
  return func(opt *WorkerOption) {
    opt.preHook = hook
  }
}

func WorkerPostHook(hook WorkerHook) WorkerOptionFunc {
  return func(opt *WorkerOption) {
    opt.postHook = hook
  }
}

// compile check
var(
  _ Worker = (*defaultWorker)(nil)
)

const(
  workerEnqueueInit    int32 = 0
  workerEnqueueClosed  int32 = 1
)

type defaultWorker struct {
  queue        *Queue
  handler      WorkerHandler
  executor     *Executor
  closed       int32
  wg           *sync.WaitGroup
  ctx          context.Context
  cancel       context.CancelFunc
  panicHandler PanicHandler
  preHook      WorkerHook
  postHook     WorkerHook
}

// run background
func NewDefaultWorker(handler WorkerHandler, funcs ...WorkerOptionFunc) *defaultWorker {
  opt := new(WorkerOption)
  for _, fn := range funcs {
    fn(opt)
  }
  if opt.ctx == nil {
    opt.ctx = context.Background()
  }
  if opt.panicHandler == nil {
    opt.panicHandler = defaultPanicHandler
  }
  if opt.preHook == nil {
    opt.preHook = noopWorkerHook
  }
  if opt.postHook == nil {
    opt.postHook = noopWorkerHook
  }

  ctx, cancel   := context.WithCancel(opt.ctx)
  w             := new(defaultWorker)
  w.queue        = NewQueue(0, QueuePanicHandler(opt.panicHandler))
  w.handler      = handler
  w.executor     = NewExecutor(1, 1)
  w.closed       = workerEnqueueInit
  w.wg           = new(sync.WaitGroup)
  w.ctx          = ctx
  w.cancel       = cancel
  w.panicHandler = opt.panicHandler
  w.preHook      = opt.preHook
  w.postHook     = opt.postHook

  w.initWorker()
  return w
}

func (w *defaultWorker) initWorker() {
  w.wg.Add(1)
  w.executor.Submit(w.runloop)
}

func (w *defaultWorker) ForceStop() {
  w.cancel()
}

// release channels and executor goroutine
func (w *defaultWorker) Shutdown() {
  w.CloseEnqueue()
  w.executor.Release()
}

func (w *defaultWorker) ShutdownAndWait() {
  w.CloseEnqueue()
  w.wg.Wait()
  w.executor.ReleaseAndWait()
}

func (w *defaultWorker) CloseEnqueue() bool {
  if w.tryQueueClose() {
    w.queue.Close()
    return true
  }
  return false
}

func (w *defaultWorker) tryQueueClose() bool {
  return atomic.CompareAndSwapInt32(&w.closed, workerEnqueueInit, workerEnqueueClosed)
}

// enqueue parameter w/ blocking until handler running
func (w *defaultWorker) Enqueue(param interface{}) bool {
  return w.queue.Enqueue(param)
}

func (w *defaultWorker) runloop() {
  defer w.wg.Done()
  for {
    select {
    case <-w.ctx.Done():
      return

    case param, ok := <-w.queue.Chan():
      if ok != true {
        return
      }

      w.preHook()
      w.handler(param)
      w.postHook()
    }
  }
}

// compile check
var(
  _ Worker = (*bufferWorker)(nil)
)

func bufferExecNoopDone() {
  /* noop */
}

type bufferWorker struct {
  defaultWorker
  chkqueue  *Queue
}

func NewBufferWorker(handler WorkerHandler, funcs ...WorkerOptionFunc) *bufferWorker {
  opt := new(WorkerOption)
  for _, fn := range funcs {
    fn(opt)
  }
  if opt.ctx == nil {
    opt.ctx = context.Background()
  }
  if opt.panicHandler == nil {
    opt.panicHandler = defaultPanicHandler
  }
  if opt.preHook == nil {
    opt.preHook = noopWorkerHook
  }
  if opt.postHook == nil {
    opt.postHook = noopWorkerHook
  }

  ctx, cancel   := context.WithCancel(opt.ctx)
  w             := new(bufferWorker)
  w.queue        = NewQueue(0, QueuePanicHandler(opt.panicHandler))
  w.handler      = handler
  w.executor     = NewExecutor(2, 2) // checker + dequeue
  w.closed       = workerEnqueueInit
  w.wg           = new(sync.WaitGroup)
  w.ctx          = ctx
  w.cancel       = cancel
  w.panicHandler = opt.panicHandler
  w.preHook      = opt.preHook
  w.postHook     = opt.postHook
  w.chkqueue     = NewQueue(1, QueuePanicHandler(noopPanicHandler))

  w.initWorker()
  return w
}

// run background
func (w *bufferWorker) initWorker() {
  w.wg.Add(1)
  w.executor.Submit(w.runloop)
}

func (w *bufferWorker) ForceStop() {
  w.cancel()
}

// release channels and executor goroutine
func (w *bufferWorker) Shutdown() {
  w.CloseEnqueue()
  w.executor.Release()
}

func (w *bufferWorker) ShutdownAndWait() {
  w.CloseEnqueue()
  w.wg.Wait()
  w.executor.ReleaseAndWait()
}

func (w *bufferWorker) CloseEnqueue() bool {
  if w.tryQueueClose() {
    w.queue.Close()
    return true
  }
  return false
}

func (w *bufferWorker) tryQueueClose() bool {
  return atomic.CompareAndSwapInt32(&w.closed, workerEnqueueInit, workerEnqueueClosed)
}

// enqueue parameter w/ non-blocking until capacity
func (w *bufferWorker) Enqueue(param interface{}) bool {
  return w.queue.Enqueue(param)
}

// execute handler from queue
func (w *bufferWorker) exec(parameters []interface{}, done func()) {
  defer done()

  w.preHook()
  for _, param := range parameters {
    w.handler(param)
  }
  w.postHook()
}

func (w *bufferWorker) runloop() {
  defer w.wg.Done()
  defer w.chkqueue.Close()

  check := func(c *Queue) Job {
    return func(){
      c.EnqueueNB(struct{}{})
    }
  }(w.chkqueue)

  genExec := func(q []interface{}, done func()) Job {
    return func(){
      w.exec(q, done)
    }
  }

  buffer := make([]interface{}, 0)
  for {
    select {
    case <-w.ctx.Done():
      return

    case <-w.chkqueue.Chan():
      if len(buffer) < 1 {
        continue
      }

      queue := make([]interface{}, len(buffer))
      copy(queue, buffer)
      buffer = buffer[len(buffer):]
      w.executor.Submit(genExec(queue, check))

    case param, ok :=<-w.queue.Chan():
      if ok != true {
        if 0 < len(buffer) {
          w.executor.Submit(genExec(buffer, bufferExecNoopDone))
        }
        return
      }

      buffer = append(buffer, param)
      w.executor.Submit(check)
    }
  }
}
