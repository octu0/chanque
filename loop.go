package chanque

import (
	"context"
	"sync"
	"time"
)

type loopTickerPool struct {
	pool *sync.Pool
}

func (p *loopTickerPool) Get(dur time.Duration) *time.Ticker {
	if v := p.pool.Get(); v != nil {
		if t, ok := v.(*time.Ticker); ok {
			// reuse
			t.Reset(dur)
			return t
		}
	}
	return time.NewTicker(dur)
}

func (p *loopTickerPool) Put(t *time.Ticker) {
	t.Stop()
	p.pool.Put(t)
}

func newLoopTickerPool() *loopTickerPool {
	return &loopTickerPool{
		pool: new(sync.Pool),
	}
}

type LoopNext uint8

const (
	LoopNextContinue LoopNext = iota + 1
	LoopNextBreak
)

type LoopDefaultHandler func() LoopNext

type LoopDequeueHandler func(val interface{}, ok bool) LoopNext

type LoopTickerHandler func() LoopNext

type LoopOptionFunc func(*optLoop)

type optLoop struct {
	ctx context.Context
}

func LoopContext(ctx context.Context) LoopOptionFunc {
	return func(opt *optLoop) {
		opt.ctx = ctx
	}
}

var (
	tickerPool = newLoopTickerPool()
)

type loopMux struct {
	ticker         *time.Ticker
	tickerHandler  LoopTickerHandler
	queue          *Queue
	dequeueHandler LoopDequeueHandler
	defaultHandler LoopDefaultHandler
}

func (m *loopMux) loopJob(ctx context.Context) Job {
	return func() {
		defer m.releaseTicker()

		for {
			next := m.sel(ctx)
			if next != LoopNextContinue {
				return
			}
		}
	}
}

func (m *loopMux) releaseTicker() {
	if m.ticker != nil {
		tickerPool.Put(m.ticker)
		m.ticker = nil
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
	mutex  *sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	exec   *SubExecutor
	mux    *loopMux
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

	if lo.mux.ticker != nil {
		tickerPool.Put(lo.mux.ticker)
	}
	lo.mux.ticker = tickerPool.Get(dur)
	lo.mux.tickerHandler = h
}

func (lo *Loop) Execute() {
	lo.mutex.Lock()
	defer lo.mutex.Unlock()

	ctx, cancel := context.WithCancel(lo.ctx)
	lo.cancel = cancel
	lo.exec.Submit(lo.mux.loopJob(ctx))
}

func (lo *Loop) ExecuteTimeout(timeout time.Duration) {
	lo.mutex.Lock()
	defer lo.mutex.Unlock()

	ctx, cancel := context.WithTimeout(lo.ctx, timeout)
	lo.cancel = cancel
	lo.exec.Submit(lo.mux.loopJob(ctx))
}

func NewLoop(e *Executor, funcs ...LoopOptionFunc) *Loop {
	opt := new(optLoop)
	for _, fn := range funcs {
		fn(opt)
	}
	if opt.ctx == nil {
		opt.ctx = context.Background()
	}

	return &Loop{
		mutex:  new(sync.Mutex),
		ctx:    opt.ctx,
		cancel: nil,
		exec:   e.SubExecutor(),
		mux:    new(loopMux),
	}
}
