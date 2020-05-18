package chanque

import(
  "time"
)

type PanicType uint8
const(
  PanicTypeEnqueue PanicType = iota + 1
  PanicTypeDequeue
  PanicTypeClose
)

type PanicHandler func(PanicType, interface{}, *bool)

type Queue struct {
  ch         chan interface{}
  pncHandler PanicHandler
}

func New(c int) *Queue {
  return &Queue{
    ch: make(chan interface{}, c),
  }
}

func (q *Queue) PanicHandler(handler PanicHandler) {
  q.pncHandler = handler
}

func (q *Queue) Close() (closed bool) {
  defer func(){
    if rcv := recover(); rcv != nil {
      if q.pncHandler != nil {
        q.pncHandler(PanicTypeClose, rcv, &closed)
      }
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
        q.pncHandler(PanicTypeEnqueue, rcv, &write)
      }
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
        q.pncHandler(PanicTypeEnqueue, rcv, &write)
      }
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
        q.pncHandler(PanicTypeDequeue, rcv, &found)
      }
    }
  }()
  val, found = <-q.ch, true
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
