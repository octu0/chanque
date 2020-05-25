package chanque

import(
  "context"
  "sync"
  "sync/atomic"
)

type Worker interface {
  PanicHandler(PanicHandler)
  Run(context.Context)
  Shutdown()
  ShutdownAndWait()
  Enqueue(interface{}) bool
  CloseEnqueue()       bool
  ForceCancel()
}

type WorkerHandler func(parameter interface{})

// cople check
var(
  _ Worker = (*defaultWorker)(nil)
)

const(
  workerEnqueueInit    int32 = 0
  workerEnqueueClosed  int32 = 1
)

type defaultWorker struct {
  queue    *Queue
  handler  WorkerHandler
  cancel   context.CancelFunc
  executor *Executor
  closed   int32
  wg       *sync.WaitGroup
}

// run background
func NewDefaultWorker(handler WorkerHandler) *defaultWorker {
  w         := new(defaultWorker)
  w.queue    = NewQueue(0)
  w.handler  = handler
  w.executor = CreateExecutor(1, 1)
  w.closed   = workerEnqueueInit
  w.wg       = new(sync.WaitGroup)
  return w
}

func (w *defaultWorker) PanicHandler(handler PanicHandler) {
  w.queue.PanicHandler(handler)
}

func (w *defaultWorker) Run(parent context.Context) {
  if parent == nil {
    parent = context.Background()
  }

  ctx, cancel := context.WithCancel(parent)
  if w.cancel != nil {
    w.cancel()
  }
  w.cancel = cancel

  w.wg.Add(1)
  w.executor.Submit(func(c context.Context) Job {
    return func() {
      w.runloop(c)
    }
  }(ctx))
}

func (w *defaultWorker) ForceCancel() {
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

func (w *defaultWorker) runloop(ctx context.Context) {
  defer w.wg.Done()
  for {
    select {
    case <-ctx.Done():
      return

    case param, ok := <-w.queue.Chan():
      if ok != true {
        return
      }

      w.handler(param)
    }
  }
}

// cople check
var(
  _ Worker = (*bufferWorker)(nil)
)

type bufferWorker struct {
  defaultWorker
}

func NewBufferWorker(handler WorkerHandler) *bufferWorker {
  w         := new(bufferWorker)
  w.queue    = NewQueue(0)
  w.handler  = handler
  w.executor = CreateExecutor(2, 2) // checker + dequeue
  w.closed   = workerEnqueueInit
  w.wg       = new(sync.WaitGroup)
  return w
}

func (w *bufferWorker) PanicHandler(handler PanicHandler) {
  w.queue.PanicHandler(handler)
}

// run background
func (w *bufferWorker) Run(parent context.Context) {
  if parent == nil {
    parent = context.Background()
  }

  ctx, cancel := context.WithCancel(parent)
  if w.cancel != nil {
    w.cancel()
  }
  w.cancel = cancel

  boot := make(chan struct{})
  w.wg.Add(1)
  w.executor.Submit(func(c context.Context, b chan struct{}) Job {
    return func(){
      b <-struct{}{}
      w.runloop(c)
    }
  }(ctx, boot))
  <-boot
}

func (w *bufferWorker) ForceCancel() {
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

func (w *bufferWorker) exec(parameters []interface{}, done func()) {
  defer done()

  for _, param := range parameters {
    w.handler(param)
  }
}

func (w *bufferWorker) runloop(ctx context.Context) {
  defer w.wg.Done()

  chkqueue := NewQueue(1)
  chkqueue.PanicHandler(noopPanicHandler)
  defer chkqueue.Close()

  check := func(c *Queue) Job {
    return func(){
      c.EnqueueNB(struct{}{})
    }
  }(chkqueue)

  genExec := func(q []interface{}, done func()) Job {
    return func(){
      w.exec(q, done)
    }
  }

  buffer := make([]interface{}, 0)
  for {
    select {
    case <-ctx.Done():
      return

    case <-chkqueue.Chan():
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
          w.executor.Submit(genExec(buffer, func(){
            /* nop done */
          }))
        }
        return
      }

      buffer = append(buffer, param)
      w.executor.Submit(check)
    }
  }
}
