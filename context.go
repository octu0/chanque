package chanque

type DoneFunc func()

type Context struct {
  waits    *SubExecutor
  bg       *SubExecutor
  done     DoneFunc
}

func NewContext(executor *Executor, done DoneFunc) *Context {
  c      := new(Context)
  c.waits = executor.SubExecutor()
  c.bg    = executor.SubExecutor()
  c.done  = done
  return c
}

func (c *Context) createWriteDone(done chan struct{}) func() {
  return func(){
    done <-struct{}{}
  }
}

func (c *Context) createReadDone(done chan struct{}) Job {
  return func(){
    <-done
  }
}

func (c *Context) Add() func() {
  done := make(chan struct{})
  f    := c.createWriteDone(done)
  c.waits.Submit(c.createReadDone(done))
  return f
}

func (c *Context) Wait() {
  c.waits.Wait()
  c.done()
}

func (c *Context) Background() {
  c.bg.Submit(c.Wait)
}
