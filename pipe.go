package chanque

import(
  "sync"
)

type PipelineInputFunc  func(parameter interface{}) (result interface{}, err error)
type PipelineOutputFunc func(result interface{}, err error)

type pipe struct {
  input    interface{}
  result   chan *pipeInputResult
  done     chan struct{}
}

type pipeInputResult struct {
  result interface{}
  err    error
}

type PipelineOptionFunc func(*PipelineOption)
type PipelineOption struct {
  panicHandler PanicHandler
}

func PipelinePanicHandler(handler PanicHandler) PipelineOptionFunc {
  return func(p *PipelineOption) {
    p.panicHandler = handler
  }
}

type Pipeline struct {
  done         *sync.WaitGroup
  parameters   Worker
  inWorker     Worker
  outWorker    Worker
  doneWorker   Worker
  inFunc       PipelineInputFunc
  outFunc      PipelineOutputFunc
}

func NewPipeline(inFunc PipelineInputFunc, outFunc PipelineOutputFunc, opts ...PipelineOptionFunc) *Pipeline {
  opt:= new(PipelineOption)
  for _, fn := range opts {
    fn(opt)
  }
  if opt.panicHandler == nil {
    opt.panicHandler = defaultPanicHandler
  }

  p           := new(Pipeline)
  p.done       = new(sync.WaitGroup)
  p.inFunc     = inFunc
  p.outFunc    = outFunc
  p.parameters = NewBufferWorker(p.workerPrepare, WorkerPanicHandler(opt.panicHandler))
  p.inWorker   = NewBufferWorker(p.workerIn, WorkerPanicHandler(opt.panicHandler))
  p.outWorker  = NewBufferWorker(p.workerOut, WorkerPanicHandler(opt.panicHandler))
  p.doneWorker = NewBufferWorker(p.workerDone, WorkerPanicHandler(opt.panicHandler))

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
  pp.result <-&pipeInputResult{result, err}
  close(pp.result)
}

func (p *Pipeline) workerOut(parameter interface{}) {
  pp := parameter.(*pipe)
  r  := <-pp.result
  p.outFunc(r.result, r.err)

  pp.done <-struct{}{}
  close(pp.done)
}

func (p *Pipeline) workerDone(parameter interface{}) {
  pp := parameter.(*pipe)

  <-pp.done
  p.done.Done()
}

func (p *Pipeline) Enqueue(parameter interface{}) bool {
  pp := &pipe{
    input:    parameter,
    result:   make(chan *pipeInputResult),
    done:     make(chan struct{}),
  }

  p.done.Add(1)
  if ok := p.parameters.Enqueue(pp); ok != true {
    p.done.Add(-1)
    return false
  }
  return true
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
  if p.parameters.CloseEnqueue() {
    p.parameters.ShutdownAndWait()
  }
  if p.inWorker.CloseEnqueue() {
    p.inWorker.ShutdownAndWait()
  }
  if p.outWorker.CloseEnqueue() {
    p.outWorker.ShutdownAndWait()
  }
  if p.doneWorker.CloseEnqueue() {
    p.doneWorker.ShutdownAndWait()
  }
  p.done.Wait()
}
