package chanque

import(
  "time"
  "sync"
  "context"
)

type LoopNext uint8

const(
  LoopNextContinue LoopNext = iota + 1
  LoopNextBreak
)

type LoopDefaultHandler  func() LoopNext

type LoopDequeueHandler  func(val interface{}, ok bool) LoopNext

type LoopTickerHandler   func() LoopNext

type LoopOptionFunc      func(*optLoop)

type optLoop struct {
  ctx context.Context
}

func LoopContext(ctx context.Context) LoopOptionFunc {
  return func(opt *optLoop) {
    opt.ctx = ctx
  }
}

type loopMux struct {
  ticker         *time.Ticker
  tickerHandler  LoopTickerHandler
  queue          *Queue
  dequeueHandler LoopDequeueHandler
  defaultHandler LoopDefaultHandler
}

func (m *loopMux) loopJob(ctx context.Context) Job {
  return func() {
    defer m.stopTicker()

    for {
      next := m.sel(ctx)
      if next != LoopNextContinue {
        return
      }
    }
  }
}

func (m *loopMux) stopTicker() {
  if m.ticker != nil {
    m.ticker.Stop()
  }
}

func (m *loopMux) selectTickQueueDefault(ctx context.Context) LoopNext {
  select {
  case <-ctx.Done():
    return LoopNextBreak

  case <-m.ticker.C:
    return m.tickerHandler()

  case val, ok := <-m.queue.Chan():
    return m.dequeueHandler(val, ok)

  default:
    return m.defaultHandler()
  }

}
func (m *loopMux) selectTickQueue(ctx context.Context) LoopNext {
  select {
  case <-ctx.Done():
    return LoopNextBreak

  case <-m.ticker.C:
    return m.tickerHandler()

  case val, ok := <-m.queue.Chan():
    return m.dequeueHandler(val, ok)
  }
}

func (m *loopMux) selectTickerDefault(ctx context.Context) LoopNext {
  select {
  case <-ctx.Done():
    return LoopNextBreak

  case <-m.ticker.C:
    return m.tickerHandler()

  default:
    return m.defaultHandler()
  }
}
func (m *loopMux) selectTicker(ctx context.Context) LoopNext {
  select {
  case <-ctx.Done():
    return LoopNextBreak

  case <-m.ticker.C:
    return m.tickerHandler()
  }
}

func (m *loopMux) selectQueueDefault(ctx context.Context) LoopNext {
  select {
  case <-ctx.Done():
    return LoopNextBreak

  case val, ok := <-m.queue.Chan():
    return m.dequeueHandler(val, ok)

  default:
    return m.defaultHandler()
  }
}

func (m *loopMux) selectQueue(ctx context.Context) LoopNext {
  select {
  case <-ctx.Done():
    return LoopNextBreak

  case val, ok := <-m.queue.Chan():
    return m.dequeueHandler(val, ok)
  }
}

func (m *loopMux) selectDefault(ctx context.Context) LoopNext {
  select {
  case <-ctx.Done():
    return LoopNextBreak

  default:
    return m.defaultHandler()
  }
}

func (m *loopMux) withDefault(ctx context.Context) LoopNext {
  if m.ticker != nil && m.queue != nil {
    // default && ticker && queue
    return m.selectTickQueueDefault(ctx)
  }
  if m.ticker != nil && m.queue == nil {
    // default && ticker
    return m.selectTickerDefault(ctx)
  }
  if m.ticker == nil && m.queue != nil {
    // default && queue
    return m.selectQueueDefault(ctx)
  }
  // default
  return m.selectDefault(ctx)
}

func (m *loopMux) sel(ctx context.Context) LoopNext {
  if m.defaultHandler != nil {
    return m.withDefault(ctx)
  }

  if m.ticker != nil && m.queue != nil {
    // ticker && queue
    return m.selectTickQueue(ctx)
  }
  if m.ticker != nil && m.queue == nil {
    // ticker
    return m.selectTicker(ctx)
  }
  if m.ticker == nil && m.queue != nil {
    // queue
    return m.selectQueue(ctx)
  }
  // all nil
  return LoopNextBreak
}

type Loop struct {
  mutex    *sync.Mutex
  ctx      context.Context
  cancel   context.CancelFunc
  exec     *SubExecutor
  mux      *loopMux
}

func NewLoop(e *Executor, funcs ...LoopOptionFunc) *Loop {
  opt := new(optLoop)
  for _, fn := range funcs {
    fn(opt)
  }
  if opt.ctx == nil {
    opt.ctx = context.Background()
  }

  lo        := new(Loop)
  lo.mutex   = new(sync.Mutex)
  lo.ctx     = opt.ctx
  lo.cancel  = nil
  lo.exec    = e.SubExecutor()
  lo.mux     = new(loopMux)
  return lo
}

func (lo *Loop) Stop() {
  lo.mutex.Lock()
  defer lo.mutex.Unlock()

  if lo.cancel != nil {
    lo.cancel()
  }
}
func (lo *Loop) StopAndWait() {
  lo.Stop()
  lo.exec.Wait()
}

func (lo *Loop) SetDefault(h LoopDefaultHandler) {
  lo.mutex.Lock()
  defer lo.mutex.Unlock()

  lo.mux.defaultHandler = h
}

func (lo *Loop) SetDequeue(h LoopDequeueHandler, q *Queue) {
  lo.mutex.Lock()
  defer lo.mutex.Unlock()

  lo.mux.queue = q
  lo.mux.dequeueHandler = h
}

func (lo *Loop) SetTicker(h LoopTickerHandler, dur time.Duration) {
  lo.mutex.Lock()
  defer lo.mutex.Unlock()

  lo.mux.ticker = time.NewTicker(dur)
  lo.mux.tickerHandler = h
}

func (lo *Loop) Execute() {
  lo.mutex.Lock()
  defer lo.mutex.Unlock()

  ctx, cancel := context.WithCancel(lo.ctx)
  lo.cancel    = cancel
  lo.exec.Submit(lo.mux.loopJob(ctx))
}

func (lo *Loop) ExecuteTimeout(timeout time.Duration) {
  lo.mutex.Lock()
  defer lo.mutex.Unlock()

  ctx, cancel := context.WithTimeout(lo.ctx, timeout)
  lo.cancel    = cancel
  lo.exec.Submit(lo.mux.loopJob(ctx))
}
