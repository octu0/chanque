package chanque

import (
	"context"
	"errors"
	"sync"
)

type PipelineInputFunc func(parameter interface{}) (result interface{}, err error)

type PipelineOutputFunc func(result interface{}, err error)

var (
	ErrPipeClosed = errors.New("pipe queue closed")
)

type pipe struct {
	input  interface{}
	result chan *pipeInputResult
	done   chan struct{}
}

type pipeInputResult struct {
	result interface{}
	err    error
}

type PipelineOptionFunc func(*optPipeline)

type optPipeline struct {
	ctx          context.Context
	panicHandler PanicHandler
	executor     *Executor
}

func PipelineContext(ctx context.Context) PipelineOptionFunc {
	return func(opt *optPipeline) {
		opt.ctx = ctx
	}
}

func PipelinePanicHandler(handler PanicHandler) PipelineOptionFunc {
	return func(opt *optPipeline) {
		opt.panicHandler = handler
	}
}

func PipelineExecutor(executor *Executor) PipelineOptionFunc {
	return func(opt *optPipeline) {
		opt.executor = executor
	}
}

type Pipeline struct {
	done       *sync.WaitGroup
	inFunc     PipelineInputFunc
	outFunc    PipelineOutputFunc
	parameters Worker
	inWorker   Worker
	outWorker  Worker
	doneWorker Worker
}

func NewPipeline(inFunc PipelineInputFunc, outFunc PipelineOutputFunc, opts ...PipelineOptionFunc) *Pipeline {
	opt := new(optPipeline)
	for _, fn := range opts {
		fn(opt)
	}
	if opt.ctx == nil {
		opt.ctx = context.Background()
	}
	if opt.panicHandler == nil {
		opt.panicHandler = defaultPanicHandler
	}

	p := &Pipeline{
		done:    new(sync.WaitGroup),
		inFunc:  inFunc,
		outFunc: outFunc,
	}
	p.parameters = NewBufferWorker(p.workerPrepare,
		WorkerContext(opt.ctx),
		WorkerPanicHandler(opt.panicHandler),
		WorkerExecutor(opt.executor),
		WorkerAbortQueueHandler(p.abortParam),
	)
	p.inWorker = NewBufferWorker(p.workerIn,
		WorkerContext(opt.ctx),
		WorkerPanicHandler(opt.panicHandler),
		WorkerExecutor(opt.executor),
		WorkerAbortQueueHandler(p.abortIn),
	)
	p.outWorker = NewBufferWorker(p.workerOut,
		WorkerContext(opt.ctx),
		WorkerPanicHandler(opt.panicHandler),
		WorkerExecutor(opt.executor),
		WorkerAbortQueueHandler(p.abortOut),
	)
	p.doneWorker = NewBufferWorker(p.workerDone,
		WorkerContext(opt.ctx),
		WorkerPanicHandler(opt.panicHandler),
		WorkerExecutor(opt.executor),
		WorkerAbortQueueHandler(p.abortDone),
	)

	return p
}

func (p *Pipeline) workerPrepare(parameter interface{}) {
	pp := parameter.(*pipe)

	p.doneWorker.Enqueue(pp)
	p.outWorker.Enqueue(pp)
	p.inWorker.Enqueue(pp)
}

func (p *Pipeline) workerIn(parameter interface{}) {
	pp := parameter.(*pipe)

	result, err := p.inFunc(pp.input)
	pp.result <- &pipeInputResult{result, err}
	close(pp.result)
}

func (p *Pipeline) workerOut(parameter interface{}) {
	pp := parameter.(*pipe)
	r := <-pp.result
	p.outFunc(r.result, r.err)

	pp.done <- struct{}{}
	close(pp.done)
}

func (p *Pipeline) workerDone(parameter interface{}) {
	pp := parameter.(*pipe)

	<-pp.done
	p.done.Done()
}

func (p *Pipeline) Enqueue(parameter interface{}) bool {
	pp := &pipe{
		input:  parameter,
		result: make(chan *pipeInputResult),
		done:   make(chan struct{}),
	}

	p.done.Add(1)
	if ok := p.parameters.Enqueue(pp); ok != true {
		return false
	}
	return true
}

func (p *Pipeline) abortParam(parameter interface{}) {
	pp := parameter.(*pipe)

	pp.result <- &pipeInputResult{nil, ErrPipeClosed}
	close(pp.result)

	pp.done <- struct{}{}
	close(pp.done)

	p.done.Done()
}

func (p *Pipeline) abortIn(parameter interface{}) {
	pp := parameter.(*pipe)
	pp.result <- &pipeInputResult{nil, ErrPipeClosed}
	close(pp.result)
}

func (p *Pipeline) abortOut(parameter interface{}) {
	pp := parameter.(*pipe)
	pp.done <- struct{}{}
	close(pp.done)
}

func (p *Pipeline) abortDone(parameter interface{}) {
	pp := parameter.(*pipe)
	<-pp.done
	p.done.Done()
}

func (p *Pipeline) CloseEnqueue() bool {
	return p.parameters.CloseEnqueue()
}

func (p *Pipeline) Shutdown() {
	p.parameters.Shutdown()
	p.inWorker.Shutdown()
	p.outWorker.Shutdown()
	p.doneWorker.Shutdown()
}

func (p *Pipeline) ShutdownAndWait() {
	p.parameters.ShutdownAndWait()
	p.inWorker.ShutdownAndWait()
	p.outWorker.ShutdownAndWait()
	p.doneWorker.ShutdownAndWait()
	p.done.Wait()
}
