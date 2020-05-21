package chanque

import(
  "context"
)

type Worker interface {
  PanicHandler(PanicHandler)
  Run(context.Context)
  Shutdown()
  Enqueue(interface{})
}

type WorkerHandler func(parameter interface{})

// cople check
var(
  _ Worker = (*defaultWorker)(nil)
)

type defaultWorker struct {
  queue    *Queue
  handler  WorkerHandler
  cancel   context.CancelFunc
  executor *Executor
}
func NewDefaultWorker(handler WorkerHandler) *defaultWorker {
  w         := new(defaultWorker)
  w.queue    = NewQueue(0)
  w.handler  = handler
  w.executor = CreateExecutor(1, 1)
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

  w.executor.Submit(func(c context.Context) Job {
    return func() {
      w.runloop(c)
    }
  }(ctx))
}
func (w *defaultWorker) Shutdown() {
  w.cancel()
  w.queue.Close()
  w.executor.Release()
}
func (w *defaultWorker) Enqueue(param interface{}) {
  w.queue.Enqueue(param)
}
func (w *defaultWorker) runloop(ctx context.Context) {
  for {
    select {
    case <-ctx.Done():
      return

    case param := <-w.queue.Chan():
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
func NewBufferWorker(handler WorkerHandler, workerSize int) *bufferWorker {
  if workerSize < 2 {
    workerSize = 2
  }

  w         := new(bufferWorker)
  w.queue    = NewQueue(0)
  w.handler  = handler
  w.executor = CreateExecutor(workerSize, workerSize * 2)
  return w
}
func (w *bufferWorker) PanicHandler(handler PanicHandler) {
  w.queue.PanicHandler(handler)
}
func (w *bufferWorker) Run(parent context.Context) {
  if parent == nil {
    parent = context.Background()
  }

  ctx, cancel := context.WithCancel(parent)
  if w.cancel != nil {
    w.cancel()
  }
  w.cancel = cancel

  w.executor.Submit(func(c context.Context) Job {
    return func(){
      w.runloop(c)
    }
  }(ctx))
}
func (w *bufferWorker) Shutdown() {
  w.cancel()
  w.queue.Close()
  w.executor.Release()
}
func (w *bufferWorker) Enqueue(param interface{}) {
  w.queue.Enqueue(param)
}
func (w *bufferWorker) exec(parameters []interface{}, done func()) {
  defer done()

  for _, param := range parameters {
    w.handler(param)
  }
}
func (w *bufferWorker) runloop(ctx context.Context) {
  chkqueue := NewQueue(1)
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

    case param :=<-w.queue.Chan():
      buffer = append(buffer, param)
      w.executor.Submit(check)
    }
  }
}
