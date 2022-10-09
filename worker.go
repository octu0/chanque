package chanque

import (
	"context"
	"sync/atomic"
	"time"
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

type (
	defaultWorkerDequeueLoopFunc func(
		ctx context.Context,
		queue *Queue,
		handler WorkerHandler,
		preHook, postHook WorkerHook,
		maxDequeueSize int,
	)
)

type (
	bufferWorkerDequeueLoopFunc func(
		ctx context.Context,
		queue *Queue,
		subexec *SubExecutor,
		handler WorkerHandler,
		preHook, postHook WorkerHook,
		maxDequeueSize int,
	)
	bufferWorkerSubmitBufferFunc func(params []interface{}, done func())
)

type WorkerOptionFunc func(*optWorker)

type optWorker struct {
	ctx               context.Context
	panicHandler      PanicHandler
	preHook           WorkerHook
	postHook          WorkerHook
	abortQueueHandler WorkerAbortQueueHandlerFunc
	executor          *Executor
	capacity          int
	maxDequeueSize    int
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

func WorkerCapacity(capacity int) WorkerOptionFunc {
	return func(opt *optWorker) {
		opt.capacity = capacity
	}
}

func WorkerMaxDequeueSize(size int) WorkerOptionFunc {
	return func(opt *optWorker) {
		opt.maxDequeueSize = size
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
	if opt.capacity < 1 {
		opt.capacity = 0
	}

	if 0 < opt.capacity && 0 < opt.maxDequeueSize {
		if opt.capacity < opt.maxDequeueSize {
			opt.maxDequeueSize = opt.capacity
		}
	}

	ctx, cancel := context.WithCancel(opt.ctx)
	w := &defaultWorker{
		opt:     opt,
		queue:   NewQueue(opt.capacity, QueuePanicHandler(opt.panicHandler)),
		handler: handler,
		closed:  workerEnqueueInit,
		ctx:     ctx,
		cancel:  cancel,
		subexec: opt.executor.SubExecutor(),
	}
	w.initWorker()
	return w
}

func (w *defaultWorker) workerDequeueLoop() defaultWorkerDequeueLoopFunc {
	if 0 < w.opt.capacity && 0 < w.opt.maxDequeueSize {
		return defaultWorkerDequeueLoopMulti
	}
	return defaultWorkerDequeueLoop
}

func (w *defaultWorker) initWorker() {
	w.subexec.Submit(func(me *defaultWorker, dequeueLoop defaultWorkerDequeueLoopFunc) Job {
		return func() {
			dequeueLoop(me.ctx, me.queue, me.handler, me.opt.preHook, me.opt.postHook, w.opt.maxDequeueSize)
		}
	}(w, w.workerDequeueLoop()))
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

// finalizer safe loop
func defaultWorkerDequeueLoop(ctx context.Context, queue *Queue, handler WorkerHandler, preHook, postHook WorkerHook, _ int) {
	for {
		select {
		case <-ctx.Done():
			return

		case param, ok := <-queue.Chan():
			if ok != true {
				return
			}

			preHook()
			handler(param)
			postHook()
		}
	}
}

// finalizer safe loop
func defaultWorkerDequeueLoopMulti(ctx context.Context, queue *Queue, handler WorkerHandler, preHook, postHook WorkerHook, maxDequeueSize int) {
	var params = make([]interface{}, 0, maxDequeueSize+1)
	for {
		select {
		case <-ctx.Done():
			return

		case param, ok := <-queue.Chan():
			if ok != true {
				return
			}

			params = append(params, param)

			run := true
			for run {
				select {
				case p, succ := <-queue.Chan():
					if succ != true {
						run = false
					} else {
						params = append(params, p)
						if maxDequeueSize <= len(params) {
							run = false
						}
					}
				default:
					run = false
				}
			}

			preHook()
			for _, param := range params {
				handler(param)
			}
			postHook()

			params = params[len(params):]
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

	if 0 < opt.capacity && 0 < opt.maxDequeueSize {
		if opt.capacity < opt.maxDequeueSize {
			opt.maxDequeueSize = opt.capacity
		}
	}

	ctx, cancel := context.WithCancel(opt.ctx)
	w := &bufferWorker{
		defaultWorker: &defaultWorker{
			opt:     opt,
			queue:   NewQueue(opt.capacity, QueuePanicHandler(opt.panicHandler)),
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

func (w *bufferWorker) workerDequeueLoop() bufferWorkerDequeueLoopFunc {
	if 0 < w.opt.capacity && 0 < w.opt.maxDequeueSize {
		return bufferWorkerDequeueLoopMulti
	}
	return bufferWorkerDequeueLoopSingle
}

// run background
func (w *bufferWorker) initWorker() {
	w.subexec.Submit(func(me *bufferWorker, dequeueLoop bufferWorkerDequeueLoopFunc) Job {
		return func() {
			dequeueLoop(me.ctx, me.queue, me.subexec, w.handler, w.opt.preHook, w.opt.postHook, w.opt.maxDequeueSize)
		}
	}(w, w.workerDequeueLoop()))
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

func createSubmitBuffer(subexec *SubExecutor, handler WorkerHandler, preHook, postHook WorkerHook) bufferWorkerSubmitBufferFunc {
	handleExec := func(parameters []interface{}, done func()) {
		defer done()

		preHook()
		for _, param := range parameters {
			handler(param)
		}
		postHook()
	}
	return func(parameters []interface{}, done func()) {
		subexec.Submit(func() {
			handleExec(parameters, done)
		})
	}
}

func bufferWorkerDequeueLoopMulti(ctx context.Context, queue *Queue, subexec *SubExecutor, handler WorkerHandler, preHook, postHook WorkerHook, maxDequeueSize int) {
	submit := createSubmitBuffer(subexec, handler, preHook, postHook)
	bufferWorkerDequeueLoop(ctx, queue, submit, maxDequeueSize)
}

func bufferWorkerDequeueLoopSingle(ctx context.Context, queue *Queue, subexec *SubExecutor, handler WorkerHandler, preHook, postHook WorkerHook, maxDequeueSize int) {
	submit := createSubmitBuffer(subexec, handler, preHook, postHook)
	bufferWorkerDequeueLoop(ctx, queue, submit, 0)
}

// finalizer safe loop
func bufferWorkerDequeueLoop(ctx context.Context, queue *Queue, submitBuffer bufferWorkerSubmitBufferFunc, maxDequeueSize int) {
	running := int32(0)

	deq := NewQueue(1, QueuePanicHandler(noopPanicHandler))
	defer deq.Close()

	buffer := make([]interface{}, 0)
	defer func() {
		// wait submit handler
		submitting := true
		for submitting {
			if atomic.CompareAndSwapInt32(&running, 0, 1) != true {
				submitting = false
			}
			time.Sleep(1 * time.Millisecond)
		}

		if 0 < len(buffer) {
			submitBuffer(buffer, bufferExecNoopDone)
		}
	}()

	readMulti := false
	if 0 < maxDequeueSize {
		readMulti = true
	}
	for {
		select {
		case <-ctx.Done():
			return

		case param, ok := <-queue.Chan():
			if ok != true {
				return
			}
			buffer = append(buffer, param)

			if readMulti != true {
				deq.EnqueueNB(struct{}{})
				continue
			}

			run := true
			for run {
				select {
				case p, succ := <-queue.Chan():
					if succ != true {
						run = false
					} else {
						buffer = append(buffer, p)
						if maxDequeueSize <= len(buffer) {
							run = false
						}
					}
				default:
					run = false
				}
			}
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

			submitBuffer(queue, func() {
				atomic.StoreInt32(&running, 0)
				deq.EnqueueNB(struct{}{}) // dequeue remain buffer
			})
		}
	}
}
