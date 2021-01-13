package chanque

import (
	"context"
	"sync/atomic"
)

type Worker interface {
	Enqueue(interface{}) bool
	CloseEnqueue() bool
	Shutdown()
	ShutdownAndWait()
	ForceStop()
}

type WorkerHandler func(parameter interface{})

type WorkerHook func()

func noopWorkerHook() {
	/* noop */
}

type WorkerAbortQueueHandlerFunc func(paramter interface{})

func noopAbortQueueHandler(interface{}) {
	/* noop */
}

type WorkerOptionFunc func(*optWorker)

type optWorker struct {
	ctx               context.Context
	panicHandler      PanicHandler
	preHook           WorkerHook
	postHook          WorkerHook
	abortQueueHandler WorkerAbortQueueHandlerFunc
	executor          *Executor
}

func WorkerContext(ctx context.Context) WorkerOptionFunc {
	return func(opt *optWorker) {
		opt.ctx = ctx
	}
}

func WorkerPanicHandler(handler PanicHandler) WorkerOptionFunc {
	return func(opt *optWorker) {
		opt.panicHandler = handler
	}
}

func WorkerPreHook(hook WorkerHook) WorkerOptionFunc {
	return func(opt *optWorker) {
		opt.preHook = hook
	}
}

func WorkerPostHook(hook WorkerHook) WorkerOptionFunc {
	return func(opt *optWorker) {
		opt.postHook = hook
	}
}

func WorkerAbortQueueHandler(handler WorkerAbortQueueHandlerFunc) WorkerOptionFunc {
	return func(opt *optWorker) {
		opt.abortQueueHandler = handler
	}
}

func WorkerExecutor(executor *Executor) WorkerOptionFunc {
	return func(opt *optWorker) {
		opt.executor = executor
	}
}

// compile check
var (
	_ Worker = (*defaultWorker)(nil)
)

const (
	workerEnqueueInit   int32 = 0
	workerEnqueueClosed int32 = 1
)

type defaultWorker struct {
	opt     *optWorker
	queue   *Queue
	handler WorkerHandler
	closed  int32
	ctx     context.Context
	cancel  context.CancelFunc
	subexec *SubExecutor
}

// run background
func NewDefaultWorker(handler WorkerHandler, funcs ...WorkerOptionFunc) Worker {
	opt := new(optWorker)
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
	if opt.abortQueueHandler == nil {
		opt.abortQueueHandler = noopAbortQueueHandler
	}
	if opt.executor == nil {
		opt.executor = NewExecutor(1, 1)
	}

	ctx, cancel := context.WithCancel(opt.ctx)
	w := &defaultWorker{
		opt:     opt,
		queue:   NewQueue(0, QueuePanicHandler(opt.panicHandler)),
		handler: handler,
		closed:  workerEnqueueInit,
		ctx:     ctx,
		cancel:  cancel,
		subexec: opt.executor.SubExecutor(),
	}
	w.initWorker()
	return w
}

func (w *defaultWorker) initWorker() {
	w.subexec.Submit(w.runloop)
}

func (w *defaultWorker) ForceStop() {
	w.CloseEnqueue()
	w.cancel()
}

// release channels and executor goroutine
func (w *defaultWorker) Shutdown() {
	w.CloseEnqueue()
}

func (w *defaultWorker) ShutdownAndWait() {
	w.CloseEnqueue()
	w.subexec.Wait()
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

func (w *defaultWorker) isClosed() bool {
	return atomic.LoadInt32(&w.closed) == workerEnqueueClosed
}

// enqueue parameter w/ blocking until handler running
func (w *defaultWorker) Enqueue(param interface{}) bool {
	if w.isClosed() {
		// collect queue that queue closed
		w.opt.abortQueueHandler(param)
		return false
	}
	return w.queue.Enqueue(param)
}

func (w *defaultWorker) runloop() {
	defer w.cancel() // ensure release

	for {
		select {
		case <-w.ctx.Done():
			return

		case param, ok := <-w.queue.Chan():
			if ok != true {
				return
			}

			w.opt.preHook()
			w.handler(param)
			w.opt.postHook()
		}
	}
}

// compile check
var (
	_ Worker = (*bufferWorker)(nil)
)

func bufferExecNoopDone() {
	/* noop */
}

type bufferWorker struct {
	*defaultWorker
}

func NewBufferWorker(handler WorkerHandler, funcs ...WorkerOptionFunc) Worker {
	opt := new(optWorker)
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
	if opt.abortQueueHandler == nil {
		opt.abortQueueHandler = noopAbortQueueHandler
	}
	if opt.executor == nil {
		opt.executor = NewExecutor(2, 2) // checker + dequeue
	}

	ctx, cancel := context.WithCancel(opt.ctx)
	w := &bufferWorker{
		defaultWorker: &defaultWorker{
			opt:     opt,
			queue:   NewQueue(0, QueuePanicHandler(opt.panicHandler)),
			handler: handler,
			closed:  workerEnqueueInit,
			ctx:     ctx,
			cancel:  cancel,
			subexec: opt.executor.SubExecutor(),
		},
	}

	w.initWorker()
	return w
}

// run background
func (w *bufferWorker) initWorker() {
	w.subexec.Submit(w.runloop)
}

func (w *bufferWorker) ForceStop() {
	w.cancel()
}

// release channels and executor goroutine
func (w *bufferWorker) Shutdown() {
	w.CloseEnqueue()
}

func (w *bufferWorker) ShutdownAndWait() {
	if w.CloseEnqueue() {
		w.subexec.Wait()
	}
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

func (w *bufferWorker) isClosed() bool {
	return atomic.LoadInt32(&w.closed) == workerEnqueueClosed
}

// enqueue parameter w/ non-blocking until capacity
func (w *bufferWorker) Enqueue(param interface{}) bool {
	if w.isClosed() {
		// collect queue that queue closed
		w.opt.abortQueueHandler(param)
		return false
	}
	return w.queue.Enqueue(param)
}

// execute handler from queue
func (w *bufferWorker) exec(parameters []interface{}, done func()) {
	defer done()

	w.opt.preHook()
	for _, param := range parameters {
		w.handler(param)
	}
	w.opt.postHook()
}

func (w *bufferWorker) submitBuffer(parameters []interface{}, done func()) {
	w.subexec.Submit(func() {
		w.exec(parameters, done)
	})
}

func (w *bufferWorker) runloop() {
	running := int32(0)

	buffer := make([]interface{}, 0)
	defer func() {
		if 0 < len(buffer) {
			w.submitBuffer(buffer, bufferExecNoopDone)
		}
		w.cancel() // ensure release
	}()

	deq := NewQueue(1, QueuePanicHandler(noopPanicHandler))
	defer deq.Close()

	for {
		select {
		case <-w.ctx.Done():
			return

		case param, ok := <-w.queue.Chan():
			if ok != true {
				return
			}
			buffer = append(buffer, param)
			deq.EnqueueNB(struct{}{})

		case <-deq.Chan():
			if len(buffer) < 1 {
				continue
			}

			if atomic.CompareAndSwapInt32(&running, 0, 1) != true {
				continue
			}

			queue := make([]interface{}, len(buffer))
			copy(queue, buffer)
			buffer = buffer[len(buffer):]

			w.submitBuffer(queue, func() {
				atomic.StoreInt32(&running, 0)
				deq.EnqueueNB(struct{}{}) // dequeue remain buffer
			})
		}
	}
}
