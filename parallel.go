package chanque

import (
	"context"
	"runtime"
	"sync"
)

type ParallelJob func() (result interface{}, err error)

type optParallel struct {
	ctx context.Context
	num int
}

type ParallelOptionFunc func(*optParallel)

func ParallelContext(ctx context.Context) ParallelOptionFunc {
	return func(opt *optParallel) {
		opt.ctx = ctx
	}
}

func Parallelism(p int) ParallelOptionFunc {
	return func(opt *optParallel) {
		opt.num = p
	}
}

type Parallel struct {
	mutex       *sync.Mutex
	queue       []ParallelJob
	ctx         context.Context
	parallelism int
	executor    *Executor
}

func NewParallel(e *Executor, funcs ...ParallelOptionFunc) *Parallel {
	opt := new(optParallel)
	for _, fn := range funcs {
		fn(opt)
	}

	if opt.ctx == nil {
		opt.ctx = context.Background()
	}
	if opt.num < 1 {
		opt.num = runtime.NumCPU()
	}

	return &Parallel{
		mutex:       new(sync.Mutex),
		queue:       make([]ParallelJob, 0),
		ctx:         opt.ctx,
		parallelism: opt.num,
		executor:    e,
	}
}

func (p *Parallel) Queue(fn ParallelJob) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.queue = append(p.queue, fn)
}

func (p *Parallel) Submit() *ParallelFuture {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	future := newParallelFuture(p.ctx, p.executor)
	for _, chunk := range chunk(p.queue, p.parallelism) {
		future.add(chunk)
	}
	p.queue = p.queue[len(p.queue):]
	return future
}

type ParallelFutureResult struct {
	mutex   *sync.Mutex
	results []ValueError
}

func newParallelFutureResult() *ParallelFutureResult {
	return &ParallelFutureResult{
		mutex:   new(sync.Mutex),
		results: make([]ValueError, 0),
	}
}
func (r *ParallelFutureResult) add(re ValueError) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.results = append(r.results, re)
}
func (r *ParallelFutureResult) Results() []ValueError {
	return r.results
}

type ParallelFuture struct {
	once     *sync.Once
	ctx      context.Context
	executor *Executor
	result   *ParallelFutureResult
	worker   Worker
}

func newParallelFuture(ctx context.Context, executor *Executor) *ParallelFuture {
	result := newParallelFutureResult()
	return &ParallelFuture{
		once:     new(sync.Once),
		ctx:      ctx,
		executor: executor,
		result:   result,
		worker: NewDefaultWorker(
			createFutureJobHandler(executor, result),
			WorkerContext(ctx),
			WorkerExecutor(executor),
		),
	}
}

func (f *ParallelFuture) add(jobs []ParallelJob) {
	f.worker.Enqueue(jobs)
}

func (f *ParallelFuture) Result() []ValueError {
	f.once.Do(func() {
		f.worker.ShutdownAndWait()
	})
	return f.result.Results()
}

func runParallelFutureJob(job ParallelJob, pr *ParallelFutureResult) Job {
	return func() {
		res, err := job()
		pr.add(&tupleValueError{
			value: res,
			err:   err,
		})
	}
}

func createFutureJobHandler(executor *Executor, result *ParallelFutureResult) WorkerHandler {
	return func(param interface{}) {
		jobs := param.([]ParallelJob)
		subexec := executor.SubExecutor()
		for _, job := range jobs {
			subexec.Submit(runParallelFutureJob(job, result))
		}
		subexec.Wait()
	}
}

func chunk(jobs []ParallelJob, size int) [][]ParallelJob {
	chunks := make([][]ParallelJob, 0)
	for size < len(jobs) {
		chunks = append(chunks, jobs[0:size:size])
		jobs = jobs[size:]
	}
	return append(chunks, jobs)
}
