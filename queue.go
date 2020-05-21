package chanque

import(
  "time"
)

type Queue struct {
  ch         chan interface{}
  pncHandler PanicHandler
}

func NewQueue(c int) *Queue {
  return &Queue{
    ch:         make(chan interface{}, c),
    pncHandler: defaultPanicHandler,
  }
}

func (q *Queue) PanicHandler(handler PanicHandler) {
  q.pncHandler = handler
}

func (q *Queue) Chan() <-chan interface{} {
  return q.ch
}

func (q *Queue) Close() (closed bool) {
  defer func(){
    if rcv := recover(); rcv != nil {
      if q.pncHandler != nil {
        q.pncHandler(PanicTypeClose, rcv)
      }
      closed = false
    }
  }()

  close(q.ch)
  closed = true
  return
}

// blocking enqueue
func (q *Queue) Enqueue(val interface{}) (write bool) {
  defer func(){
    if rcv := recover(); rcv != nil {
      if q.pncHandler != nil {
        q.pncHandler(PanicTypeEnqueue, rcv)
      }
      write = false
    }
  }()

  q.ch <-val
  write = true
  return
}

// non-blocking enqueue
func (q *Queue) EnqueueNB(val interface{}) (write bool) {
  defer func(){
    if rcv := recover(); rcv != nil {
      if q.pncHandler != nil {
        q.pncHandler(PanicTypeEnqueue, rcv)
      }
      write = false
    }
  }()

  select {
  case q.ch <-val:
    write = true
  default:
    // drop
    write = false
  }
  return
}

// retry w/ enqueue until channel can be written(waiting for channel to read)
func (q *Queue) EnqueueRetry(val interface{}, retryInterval time.Duration, retryLimit int) (timeout bool) {
  for i := 0; i < retryLimit; i += 1 {
    if q.EnqueueNB(val) {
      timeout = false
      return
    }
    time.Sleep(retryInterval)
  }
  timeout = true
  return
}

// blocking dequeue
func (q *Queue) Dequeue() (val interface{}, found bool) {
  defer func(){
    if rcv := recover(); rcv != nil {
      if q.pncHandler != nil {
        q.pncHandler(PanicTypeDequeue, rcv)
      }
      found = false
    }
  }()

  val, found = <-q.ch
  return
}

// non-blocking dequeue
func (q *Queue) DequeueNB() (val interface{}, found bool) {
  select {
  case v := <-q.ch:
    val, found = v, true
  default:
    val, found = nil, false
  }
  return
}

// retry w/ dequeue until channel can be read(waiting for channel to write)
func (q *Queue) DequeueRetry(retryInterval time.Duration, retryLimit int) (val interface{}, found bool) {
  for i := 0; i < retryLimit; i += 1 {
    if v, ok := q.DequeueNB(); ok {
      val, found = v, true
      return
    }
    time.Sleep(retryInterval)
  }
  val, found = nil, false
  return
}
