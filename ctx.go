package chanque

type DoneFunc func()

type Context struct {
  wait  chan struct{}
  done  DoneFunc
}

func NewContext(done DoneFunc) *Context {
  return &Context{
    wait: make(chan struct{}),
    done: done,
  }
}

func (c *Context) Wait() <-chan struct{} {
  return c.wait
}

func (c *Context) Done() (canDone bool) {
  defer func(){
    if canDone {
      c.done()
    }
  }()

  select {
  case c.wait <-struct{}{}:
    canDone = true
    return
  default:
    canDone = false
    return
  }
}
