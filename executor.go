package chanque

import (
	"bytes"
	"context"
	"hash/fnv"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rogpeppe/fastuuid"
)

const (
	defaultReducerInterval = 10 * time.Second
)

type ExecutorOptionFunc func(*optExecutor)

type optExecutor struct {
	ctx               context.Context
	panicHandler      PanicHandler
	reducerInterval   time.Duration
	maxCapacity       int
	collectStacktrace bool
}

func ExecutorPanicHandler(handler PanicHandler) ExecutorOptionFunc {
	return func(opt *optExecutor) {
		opt.panicHandler = handler
	}
}

func ExecutorReducderInterval(interval time.Duration) ExecutorOptionFunc {
	return func(opt *optExecutor) {
		opt.reducerInterval = interval
	}
}

func ExecutorMaxCapacity(capacity int) ExecutorOptionFunc {
	return func(opt *optExecutor) {
		opt.maxCapacity = capacity
	}
}

func ExecutorContext(ctx context.Context) ExecutorOptionFunc {
	return func(opt *optExecutor) {
		opt.ctx = ctx
	}
}

func ExecutorCollectStacktrace(enable bool) ExecutorOptionFunc {
	return func(opt *optExecutor) {
		opt.collectStacktrace = enable
	}
}

type Job func()

type executeJob struct {
	job              Job
	stacktraceID     uint64
	removeStacktrace bool
}

type cancelWorker struct {
	workerID string
	cancel   context.CancelFunc
}

type Executor struct {
	mutex               *sync.RWMutex
	wg                  *sync.WaitGroup
	ctx                 context.Context
	uuidGen             *fastuuid.Generator
	jobs                *Queue // chan *executeJob
	workerCancel        *Queue // chan *cancelWorker
	workerDone          *Queue // chan string(cancelID)
	workerRelease       *Queue // chan int(numOfReleaseWorkers)
	workerCancelAllFunc context.CancelFunc
	healthCancel        context.CancelFunc
	minWorker           int
	maxWorker           int
	panicHandler        PanicHandler
	reducerInterval     time.Duration
	runningNum          int32
	workerNum           int32
	collectStacktrace   bool
	stacktraces         map[uint64][]byte
}

func (e *Executor) initWorker(ctx context.Context) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	workerCancelAllCtx, workerCancelAllFunc := context.WithCancel(ctx)
	e.workerCancelAllFunc = workerCancelAllFunc

	hctx, hcancel := context.WithCancel(ctx)
	e.healthCancel = hcancel

	e.wg.Add(1)
	go e.workerCancelQueueLoop(workerCancelAllCtx, e.workerCancel, e.workerDone, e.workerRelease)

	e.wg.Add(1)
	go e.healthloop(hctx, e.workerRelease)

	e.goExecLoopLocked(ctx, e.minWorker, e.workerCancel)
}

func (e *Executor) goExecLoopLocked(ctx context.Context, num int, workerCancel *Queue) {
	if num < 1 {
		return
	}

	for i := 0; i < num; i += 1 {
		workerCtx, workerCancelFunc := context.WithCancel(ctx)
		workerID := e.uuidGen.Hex128()
		workerCancel.Enqueue(&cancelWorker{
			workerID: workerID,
			cancel:   workerCancelFunc,
		})

		e.wg.Add(1)
		e.increWorker()
		go e.execloop(workerCtx, workerID, e.jobs, e.workerDone)
	}
}

func (e *Executor) MinWorker() int {
	return e.minWorker
}

func (e *Executor) MaxWorker() int {
	return e.maxWorker
}

func (e *Executor) TuneMinWorker(nextMinWorker int) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if nextMinWorker < 1 {
		nextMinWorker = 1
	}
	if e.maxWorker < nextMinWorker {
		nextMinWorker = e.maxWorker
	}

	if e.minWorker < nextMinWorker {
		currentWorkers := int(e.Workers())
		if currentWorkers < nextMinWorker {
			diff := nextMinWorker - currentWorkers
			e.goExecLoopLocked(e.ctx, diff, e.workerCancel)
		}
	}

	e.minWorker = nextMinWorker
}

func (e *Executor) TuneMaxWorker(nextMaxWorker int) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if nextMaxWorker < 1 {
		nextMaxWorker = 1
	}
	if nextMaxWorker < e.minWorker {
		nextMaxWorker = e.minWorker
	}

	e.maxWorker = nextMaxWorker
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

func (e *Executor) startOndemandLocked() {
	running := e.Running()
	workerSize := e.Workers()
	nextSize := running + 1

	switch {
	case workerSize < nextSize:
		needToWorkerSize := int(nextSize - workerSize)
		if e.maxWorker < needToWorkerSize {
			needToWorkerSize = needToWorkerSize - e.maxWorker
		}
		if needToWorkerSize < 1 {
			needToWorkerSize = 1
		}
		e.goExecLoopLocked(e.ctx, needToWorkerSize, e.workerCancel)
	case workerSize == nextSize:
		e.goExecLoopLocked(e.ctx, 1, e.workerCancel)
	}
}

// enqueue job
func (e *Executor) Submit(fn Job) {
	if fn == nil {
		return
	}

	defer func() {
		if rcv := recover(); rcv != nil {
			e.panicHandler(PanicTypeEnqueue, rcv)
		}
	}()

	e.mutex.Lock()
	e.startOndemandLocked()
	e.mutex.Unlock()

	if e.collectStacktrace {
		id, stack := currentStacktrace()
		e.putStacktrace(id, stack)
		e.jobs.Enqueue(&executeJob{
			job:              fn,
			stacktraceID:     id,
			removeStacktrace: true,
		})
		return
	}

	e.jobs.Enqueue(&executeJob{
		job:              fn,
		stacktraceID:     0,
		removeStacktrace: false,
	})
}

func (e *Executor) ForceStop() {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.workerCancelAllFunc()
}

// release goroutines
func (e *Executor) Release() {
	defer func() {
		if rcv := recover(); rcv != nil {
			e.panicHandler(PanicTypeClose, rcv)
		}
	}()

	e.workerCancelAllFunc()
	e.healthCancel()
}

func (e *Executor) ReleaseAndWait() {
	e.Release()
	e.wg.Wait()
}

func (e *Executor) workerCancelQueueLoop(ctx context.Context, workerCancel, workerDone, workerRelease *Queue) {
	defer e.wg.Done()
	defer func() {
		if rcv := recover(); rcv != nil {
			e.panicHandler(PanicTypeEnqueue, rcv)
		}
	}()

	workerCancels := make(map[string]context.CancelFunc, e.minWorker)
	defer func() {
		for _, cancel := range workerCancels {
			cancel()
		}
	}()

	release := func(releaseNum int) {
		if len(workerCancels) < releaseNum {
			releaseNum = len(workerCancels)
		}
		e.mutex.RLock()
		minWorker := e.minWorker
		defer e.mutex.RUnlock()

		count := 0
		for id, cancel := range workerCancels {
			if len(workerCancels) <= minWorker {
				return
			}
			if releaseNum <= count {
				return
			}
			cancel()
			delete(workerCancels, id)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case cw := <-workerCancel.Chan():
			w := cw.(*cancelWorker)
			workerCancels[w.workerID] = w.cancel

		case id := <-workerDone.Chan():
			workerID := id.(string)
			if cancel, ok := workerCancels[workerID]; ok {
				cancel()
				delete(workerCancels, workerID)
			}
		case rn := <-workerRelease.Chan():
			releaseNum := rn.(int)
			release(releaseNum)
		}
	}
}

func (e *Executor) healthloop(ctx context.Context, workerRelease *Queue) {
	defer e.wg.Done()
	defer func() {
		if rcv := recover(); rcv != nil {
			e.panicHandler(PanicTypeEnqueue, rcv)
		}
	}()

	ticker := time.NewTicker(e.reducerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			currentWorkerNum := e.Workers()
			runningWorkerNum := e.Running()
			idleWorkers := int(currentWorkerNum - runningWorkerNum)
			if e.minWorker < idleWorkers {
				workerRelease.Enqueue(int(idleWorkers - e.minWorker))
			}
		}
	}
}

func (e *Executor) execloop(ctx context.Context, workerID string, jobs, workerDone *Queue) {
	defer e.wg.Done()
	defer func() {
		if rcv := recover(); rcv != nil {
			e.panicHandler(PanicTypeDequeue, rcv)
		}
	}()
	defer e.decreWorker()

	defer workerDone.Enqueue(workerID)

	for {
		select {
		case <-ctx.Done():
			return

		case job := <-jobs.Chan():
			e.increRunning()

			if e.Workers() <= e.Running() {
				// job added in a short interval will not be allocated,
				// maybe when Worker has been in use for a long-term or Worker in blocking process.
				e.mutex.Lock()
				e.startOndemandLocked()
				e.mutex.Unlock()
			}

			exec := job.(*executeJob)
			j := exec.job
			exec.job = nil // finalizable
			j()

			if exec.removeStacktrace {
				e.deleteStacktrace(exec.stacktraceID)
			}
			e.decreRunning()
		}
	}
}

func (e *Executor) SubExecutor() *SubExecutor {
	return newSubExecutor(e)
}

func (e *Executor) putStacktrace(id uint64, stack []byte) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.stacktraces[id] = stack
}

func (e *Executor) deleteStacktrace(id uint64) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	delete(e.stacktraces, id)
}

func (e *Executor) CurrentStacktrace() []byte {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	size := 0
	for _, buf := range e.stacktraces {
		size += len(buf)
	}

	out := bytes.NewBuffer(make([]byte, 0, size))
	for _, buf := range e.stacktraces {
		out.Write(buf)
	}
	return out.Bytes()
}

func NewExecutor(minWorker, maxWorker int, funcs ...ExecutorOptionFunc) *Executor {
	opt := new(optExecutor)
	for _, fn := range funcs {
		fn(opt)
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

	if opt.maxCapacity < 1 {
		opt.maxCapacity = 0
	}
	if opt.panicHandler == nil {
		opt.panicHandler = defaultPanicHandler
	}
	if opt.reducerInterval < 1 {
		opt.reducerInterval = defaultReducerInterval
	}
	if opt.ctx == nil {
		opt.ctx = context.Background()
	}

	stacktracesSize := 0
	if opt.collectStacktrace {
		stacktracesSize = minWorker
	}

	e := &Executor{
		mutex:               new(sync.RWMutex),
		wg:                  new(sync.WaitGroup),
		ctx:                 opt.ctx,
		uuidGen:             fastuuid.MustNewGenerator(),
		jobs:                NewQueue(opt.maxCapacity, QueuePanicHandler(opt.panicHandler)),
		workerCancel:        NewQueue(minWorker, QueuePanicHandler(opt.panicHandler)),
		workerDone:          NewQueue(minWorker, QueuePanicHandler(opt.panicHandler)),
		workerRelease:       NewQueue(0, QueuePanicHandler(opt.panicHandler)),
		workerCancelAllFunc: nil,
		healthCancel:        nil,
		minWorker:           minWorker,
		maxWorker:           maxWorker,
		panicHandler:        opt.panicHandler,
		reducerInterval:     opt.reducerInterval,
		runningNum:          int32(0),
		workerNum:           int32(0),
		collectStacktrace:   opt.collectStacktrace,
		stacktraces:         make(map[uint64][]byte, stacktracesSize),
	}

	e.initWorker(e.ctx)
	return e
}

type SubExecutor struct {
	wg     *sync.WaitGroup
	parent *Executor
}

func (s *SubExecutor) Submit(fn Job) {
	s.wg.Add(1)
	s.parent.Submit(runSubExec(s.wg, fn))
}

func (s *SubExecutor) Wait() {
	s.wg.Wait()
}

func newSubExecutor(parent *Executor) *SubExecutor {
	return &SubExecutor{
		wg:     new(sync.WaitGroup),
		parent: parent,
	}
}

func runSubExec(wg *sync.WaitGroup, fn Job) Job {
	return func() {
		defer wg.Done()
		fn()
	}
}

func currentStacktrace() (uint64, []byte) {
	h := fnv.New64()
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			h.Write(buf[:n])
			return h.Sum64(), buf[:n]
		}
		buf = make([]byte, len(buf)*2)
	}
}
