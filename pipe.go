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
  bootOut  *sync.WaitGroup
  bootDone *sync.WaitGroup
}
type pipeInputResult struct {
  result interface{}
  err    error
}

func callInput(pp *pipe, in PipelineInputFunc) Job {
  return func(){
    result, err := in(pp.input)

    pp.bootOut.Wait()
    pp.result <-&pipeInputResult{result, err}
    close(pp.result)
  }
}

func callOutput(pp *pipe, out PipelineOutputFunc) Job {
  return func(){
    pp.bootDone.Wait()
    pp.bootOut.Done()

    r := <-pp.result
    out(r.result, r.err)

    pp.done <-struct{}{}
    close(pp.done)
  }
}

func callDone(pp *pipe, wg *sync.WaitGroup) Job {
  return func(){
    pp.bootDone.Done()

    <-pp.done

    wg.Done()
  }
}

func callDoneCancel(wg *sync.WaitGroup) Job {
  return func(){
    wg.Done()
  }
}

type PipelineOptionFunc func(*PipelineOption)
type PipelineOption struct {
  panicHandler PanicHandler
  maxCapacity  int
}

func PipelinePanicHandler(handler PanicHandler) PipelineOptionFunc {
  return func(p *PipelineOption) {
    p.panicHandler = handler
  }
}
func PipelineMaxCapacity(capacity int) PipelineOptionFunc {
  return func(p *PipelineOption) {
    p.maxCapacity = capacity
  }
}

type Pipeline struct {
  done         *sync.WaitGroup
  parameters   Worker
  inout        *Executor
  inFunc       PipelineInputFunc
  outFunc      PipelineOutputFunc
}

func CreatePipeline(minWorker,maxWorker int, inFunc PipelineInputFunc, outFunc PipelineOutputFunc, opts ...PipelineOptionFunc) *Pipeline {
  opt:= new(PipelineOption)
  for _, fn := range opts {
    fn(opt)
  }
  if opt.maxCapacity < 1 {
    opt.maxCapacity = 3 // in + out + res
  }
  if opt.panicHandler == nil {
    opt.panicHandler = defaultPanicHandler
  }
  if minWorker < 1 {
    minWorker = 1
  }
  if maxWorker < 3 {
    maxWorker = 3 // in + out + res
  }

  p           := new(Pipeline)
  p.done       = new(sync.WaitGroup)
  p.inFunc     = inFunc
  p.outFunc    = outFunc
  p.parameters = NewDefaultWorker(p.worker)
  p.parameters.PanicHandler(opt.panicHandler)
  p.parameters.Run(nil)

  p.inout      = CreateExecutor(minWorker, maxWorker,
    ExecutorMaxCapacity(opt.maxCapacity),
    ExecutorPanicHandler(opt.panicHandler),
  )

  return p
}

func (p *Pipeline) worker(parameter interface{}) {
  pp, ok := parameter.(*pipe)
  if ok != true {
    panic("invalid worker parameter")
  }
  pp.bootDone.Add(1)
  pp.bootOut.Add(1)

  p.inout.Submit(callDone(pp, p.done))
  p.inout.Submit(callOutput(pp, p.outFunc))
  p.inout.Submit(callInput(pp, p.inFunc))
}

func (p *Pipeline) Enqueue(parameter interface{}) bool {
  pp := &pipe{
    input:    parameter,
    result:   make(chan *pipeInputResult),
    done:     make(chan struct{}),
    bootOut:  new(sync.WaitGroup),
    bootDone: new(sync.WaitGroup),
  }

  p.done.Add(1)
  if ok := p.parameters.Enqueue(pp); ok != true {
    p.done.Add(-1)
    return false
  }
  return true
}

func (p *Pipeline) Shutdown() {
  p.parameters.Shutdown()
  p.inout.Release()
}

func (p *Pipeline) ShutdownAndWait() {
  if p.parameters.CloseEnqueue() {
    p.parameters.ShutdownAndWait()
  }
  p.done.Wait()
  p.inout.Release()
}
